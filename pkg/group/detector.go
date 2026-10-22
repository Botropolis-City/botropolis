package group

import (
	"slices"
	"sort"
)

// Detector groups sessions run after run (item 158). It keeps each
// session's words and references from the last run and the pairs that
// scored, its ties, so that a prompt in one session scores again only
// that session's pairs. It keeps no pair that did not score: a pair
// sharing no signal is never scored, since it is never a candidate, and
// a pair of unchanged sessions missing from the ties did not score last
// run and would not now (item 198). Detect is a fresh Detector.
type Detector struct {
	wording *Wording
	refs    map[string]readRefs
	members map[string]Member
	// ties are the pairs that scored, under both sessions, each scored
	// with the earlier of the two first.
	ties   map[string]map[string]scored
	pastes map[string]pasteKeys
	scored int
}

type scored struct {
	first  string
	tier   Tier
	reason Reason
}

func NewDetector() *Detector {
	return &Detector{ties: map[string]map[string]scored{}, pastes: map[string]pasteKeys{}}
}

// PairsScored is how many pairs of sessions the detector has compared.
func (dt *Detector) PairsScored() int {
	return dt.scored
}

// PairsKept is how many pairs' scores the detector keeps between runs.
func (dt *Detector) PairsKept() int {
	kept := 0
	for _, ties := range dt.ties {
		kept += len(ties)
	}
	return kept / 2
}

// Detect groups members as group.Detect does. A session's pairs are
// scored again when it changed or the rarity of any word it uses did;
// when the number of sessions changes every word's rarity does, and so
// every candidate pair is scored again.
func (dt *Detector) Detect(members []Member, memory Memory) Result {
	d, edges := dt.run(members, memory)
	return d.detect(memory, edges)
}

func sortedMembers(members []Member) []Member {
	members = append([]Member(nil), members...)
	sort.SliceStable(members, func(i, j int) bool { return members[i].Started.Before(members[j].Started) })
	return members
}

// detectEveryPair is item 158's detector from scratch: it scores every
// pair of sessions, and the tests hold Detector to it.
func detectEveryPair(members []Member, memory Memory) Result {
	d := newDetection(sortedMembers(members), memory, nil)
	return d.detect(memory, d.everyPair())
}

func (dt *Detector) run(members []Member, memory Memory) (*detection, []edge) {
	members = sortedMembers(members)
	d := newDetection(members, memory, dt)
	changed := dt.changed(members, d.wording)
	for id := range dt.members {
		if _, ok := d.index[id]; !ok {
			dt.untieAll(id)
			delete(dt.pastes, id)
		}
	}
	for id := range changed {
		dt.untieAll(id)
	}
	if len(changed) > 0 {
		dt.scoreChanged(d, changed)
	}
	edges := dt.edges(d)
	dt.wording, dt.refs = d.wording, d.read
	dt.members = make(map[string]Member, len(members))
	for _, m := range members {
		dt.members[m.ID] = m
	}
	return d, edges
}

// scoreChanged scores each changed session against its candidates, each
// pair once, and ties those that score.
func (dt *Detector) scoreChanged(d *detection, changed map[string]bool) {
	c := d.candidates(dt.pasteKeys(d.members), changed)
	for id := range changed {
		i := d.index[id]
		for _, j := range c.of(i) {
			if changed[d.members[j].ID] && j < i {
				continue
			}
			a, b := min(i, j), max(i, j)
			dt.scored++
			if e, ok := d.pair(a, b); ok {
				dt.tie(d.members[a].ID, d.members[b].ID, scored{first: d.members[a].ID, tier: e.tier, reason: e.reason})
			}
		}
	}
}

// edges are the ties between the members, in the order detection takes
// them, leaving out pairs marked not related and sessions that sit out.
// A tie scored the other way round, which only sessions started at the
// same moment can be, is scored again.
func (dt *Detector) edges(d *detection) []edge {
	var edges []edge
	for i, m := range d.members {
		for other, s := range dt.ties[m.ID] {
			j := d.index[other]
			if j < i {
				continue
			}
			if s.first != m.ID {
				dt.scored++
				e, ok := d.pair(i, j)
				if !ok {
					dt.untie(m.ID, other)
					continue
				}
				s = scored{first: m.ID, tier: e.tier, reason: e.reason}
				dt.tie(m.ID, other, s)
			}
			if d.blocked[[2]string{m.ID, other}] || d.sitsOut(m.ID) || d.sitsOut(other) {
				continue
			}
			edges = append(edges, edge{a: i, b: j, tier: s.tier, reason: s.reason})
		}
	}
	sortEdges(edges)
	return edges
}

func (dt *Detector) tie(a, b string, s scored) {
	for _, p := range [][2]string{{a, b}, {b, a}} {
		if dt.ties[p[0]] == nil {
			dt.ties[p[0]] = map[string]scored{}
		}
		dt.ties[p[0]][p[1]] = s
	}
}

func (dt *Detector) untie(a, b string) {
	for _, p := range [][2]string{{a, b}, {b, a}} {
		delete(dt.ties[p[0]], p[1])
		if len(dt.ties[p[0]]) == 0 {
			delete(dt.ties, p[0])
		}
	}
}

func (dt *Detector) untieAll(id string) {
	for other := range dt.ties[id] {
		delete(dt.ties[other], id)
		if len(dt.ties[other]) == 0 {
			delete(dt.ties, other)
		}
	}
	delete(dt.ties, id)
}

// pasteKeys are each member's paste keys, read again only for a
// session whose pastes changed.
func (dt *Detector) pasteKeys(members []Member) map[string]pasteKeys {
	for _, m := range members {
		if len(m.Pasted) == 0 {
			delete(dt.pastes, m.ID)
			continue
		}
		if k, ok := dt.pastes[m.ID]; ok && slices.Equal(k.pasted, m.Pasted) {
			continue
		}
		dt.pastes[m.ID] = readPasteKeys(m.Pasted)
	}
	return dt.pastes
}

// changed is the sessions whose pairs must be scored again: those new or
// different since the last run, and those using a word whose rarity
// moved.
func (dt *Detector) changed(members []Member, wording *Wording) map[string]bool {
	changed := map[string]bool{}
	for _, m := range members {
		if last, ok := dt.members[m.ID]; !ok || !sameMember(last, m) {
			changed[m.ID] = true
		}
	}
	if dt.wording == nil {
		return changed
	}
	moved := map[string]bool{}
	for tok, idf := range wording.idf {
		if last, ok := dt.wording.idf[tok]; !ok || last != idf {
			moved[tok] = true
		}
	}
	for tok := range dt.wording.idf {
		if _, ok := wording.idf[tok]; !ok {
			moved[tok] = true
		}
	}
	if len(moved) == 0 {
		return changed
	}
	for id, words := range wording.words {
		if !changed[id] && (usesAny(words, moved) || usesAny(dt.wording.words[id], moved)) {
			changed[id] = true
		}
	}
	return changed
}

func usesAny(words, moved map[string]bool) bool {
	for tok := range words {
		if moved[tok] {
			return true
		}
	}
	return false
}

func sameMember(a, b Member) bool {
	return a.ID == b.ID && a.Dir == b.Dir && a.Branch == b.Branch && a.Started.Equal(b.Started) &&
		a.Ended.Equal(b.Ended) && a.Parent == b.Parent && a.Team == b.Team &&
		a.StartedFrom == b.StartedFrom && a.Job == b.Job &&
		slices.Equal(a.Prompts, b.Prompts) && slices.Equal(a.Pasted, b.Pasted) &&
		slices.Equal(a.Messaged, b.Messaged) && slices.Equal(a.Files, b.Files) &&
		slices.EqualFunc(a.Visited, b.Visited, func(x, y Visit) bool {
			return x.Dir == y.Dir && x.From.Equal(y.From) && x.To.Equal(y.To)
		})
}
