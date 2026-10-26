package group

import (
	"maps"
	"slices"
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
	index  *candidateIndex
	scored int
	kept   int
	// taken, keys and sorted are the last run's edges, kept to take the
	// next run's into rather than to allocate them again; sorted is
	// patched in place when the next run's sessions are in the same
	// order and sit out and are kept apart as before (item 210).
	taken, sorted, spare []edge
	keys                 []keyedEdge
	order                []string
	apart                apart
	// names are the groups' names from the last Detect, by their
	// sessions; a run that is not followed by naming forgets them.
	names map[string]named
}

type scored struct {
	first  string
	tier   Tier
	timed  bool
	reason Reason
}

func NewDetector() *Detector {
	return &Detector{ties: map[string]map[string]scored{}, index: newCandidateIndex(), members: map[string]Member{}}
}

// PairsScored is how many pairs of sessions the detector has compared.
func (dt *Detector) PairsScored() int {
	return dt.scored
}

// PairsKept is how many pairs' scores the detector keeps between runs.
func (dt *Detector) PairsKept() int {
	return dt.kept
}

// Detect groups members as group.Detect does. A session's pairs are
// scored again when it changed or the rarity of any word it uses did;
// when the number of sessions changes every word's rarity does, and so
// every candidate pair is scored again.
func (dt *Detector) Detect(members []Member, memory Memory) Result {
	d, edges := dt.run(members, memory)
	result := d.detect(memory, edges)
	dt.names = d.names.next
	return result
}

// sortedMembers are members by when they started, those started together
// in the order given; it sorts their places rather than moving members.
func sortedMembers(members []Member) []Member {
	order := make([]int, len(members))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(x, y int) int { return members[x].Started.Compare(members[y].Started) })
	sorted := make([]Member, len(members))
	for k, i := range order {
		sorted[k] = members[i]
	}
	return sorted
}

func (dt *Detector) run(members []Member, memory Memory) (*detection, []edge) {
	members = sortedMembers(members)
	d := newDetection(members, memory, dt)
	differs := dt.differs(members)
	for id := range dt.members {
		if _, ok := d.index[id]; !ok {
			dt.untieAll(id)
			delete(dt.members, id)
		}
	}
	dt.index.update(d, differs, d.wording.allMoved || len(dt.index.under) == 0)
	changed := dt.changed(differs, members, d.wording)
	d.names = &keptNames{last: dt.names, next: map[string]named{}, changed: changed}
	dt.names = nil
	for id := range changed {
		dt.untieAll(id)
	}
	if len(changed) > 0 {
		dt.scoreChanged(d, changed)
	}
	edges := dt.edges(d, memory, changed)
	dt.wording, dt.refs = d.wording, d.read
	for id := range changed {
		dt.members[id] = members[d.index[id]]
	}
	return d, edges
}

// scoreChanged scores each changed session against its candidates, each
// pair once, and ties those that score.
func (dt *Detector) scoreChanged(d *detection, changed map[string]bool) {
	c := d.candidates(dt.index)
	for id := range changed {
		i := d.index[id]
		for _, j := range c.of(i) {
			if changed[d.members[j].ID] && j < i {
				continue
			}
			a, b := min(i, j), max(i, j)
			dt.scored++
			if e, ok := d.pair(a, b); ok {
				dt.tie(d.members[a].ID, d.members[b].ID, scored{first: d.members[a].ID, tier: e.tier, timed: e.timed, reason: e.reason})
			}
		}
	}
}

// apart is what decides which sessions sit out and which pairs are
// kept apart, besides the sessions themselves.
type apart struct {
	moves       []Move
	notRelated  []Pair
	startedFrom map[string]string
}

func apartOf(memory Memory) apart {
	return apart{moves: slices.Clone(memory.Moves), notRelated: slices.Clone(memory.NotRelated), startedFrom: maps.Clone(memory.StartedFrom)}
}

func (a apart) same(memory Memory) bool {
	return slices.Equal(a.moves, memory.Moves) && slices.Equal(a.notRelated, memory.NotRelated) &&
		maps.Equal(a.startedFrom, memory.StartedFrom)
}

// edges are the ties between the members, in the order detection takes
// them, leaving out pairs marked not related and sessions that sit out.
// When the sessions are in the order they were last run and nothing
// sits out or is kept apart anew, the last run's edges are kept but for
// the changed sessions', whose ties are taken again and merged in.
func (dt *Detector) edges(d *detection, memory Memory, changed map[string]bool) []edge {
	patch := dt.order != nil && dt.apart.same(memory) && slices.EqualFunc(dt.order, d.members, func(id string, m Member) bool { return id == m.ID })
	dt.apart = apartOf(memory)
	dt.order = dt.order[:0]
	for _, m := range d.members {
		dt.order = append(dt.order, m.ID)
	}
	if !patch {
		return dt.allEdges(d)
	}
	if len(changed) == 0 {
		return dt.sorted
	}
	moved := make([]bool, len(d.members))
	for id := range changed {
		moved[d.index[id]] = true
	}
	fresh := dt.taken[:0]
	for id := range changed {
		i := d.index[id]
		for other := range dt.ties[id] {
			j := d.index[other]
			if moved[j] && j < i {
				continue
			}
			if e, ok := dt.taken2(d, min(i, j), max(i, j)); ok {
				fresh = append(fresh, e)
			}
		}
	}
	dt.taken = fresh
	var sortedFresh []edge
	sortedFresh, dt.keys = sortEdges(fresh, dt.keys, nil)
	merged := dt.spare[:0]
	k := 0
	for _, e := range dt.sorted {
		if moved[e.a] || moved[e.b] {
			continue
		}
		for k < len(sortedFresh) && edgeKey(sortedFresh[k]) < edgeKey(e) {
			merged = append(merged, sortedFresh[k])
			k++
		}
		merged = append(merged, e)
	}
	merged = append(merged, sortedFresh[k:]...)
	dt.spare, dt.sorted = dt.sorted, merged
	return dt.sorted
}

// allEdges takes every tie again and sorts them.
func (dt *Detector) allEdges(d *detection) []edge {
	edges := dt.taken[:0]
	for i, m := range d.members {
		for other := range dt.ties[m.ID] {
			j := d.index[other]
			if j < i {
				continue
			}
			if e, ok := dt.taken2(d, i, j); ok {
				edges = append(edges, e)
			}
		}
	}
	dt.taken = edges
	dt.sorted, dt.keys = sortEdges(edges, dt.keys, dt.sorted)
	return dt.sorted
}

// taken2 is the tie between members i and j, i first, as an edge, unless
// either sits out or the pair is marked not related. A tie scored the
// other way round, which only sessions started at the same moment can
// be, is scored again.
func (dt *Detector) taken2(d *detection, i, j int) (edge, bool) {
	a, b := d.members[i].ID, d.members[j].ID
	s, ok := dt.ties[a][b]
	if !ok {
		return edge{}, false
	}
	if s.first != a {
		dt.scored++
		e, ok := d.pair(i, j)
		if !ok {
			dt.untie(a, b)
			return edge{}, false
		}
		s = scored{first: a, tier: e.tier, timed: e.timed, reason: e.reason}
		dt.tie(a, b, s)
	}
	if d.sitsOut(a) || d.sitsOut(b) || (len(d.blocked) > 0 && d.blocked[[2]string{a, b}]) {
		return edge{}, false
	}
	return edge{a: i, b: j, tier: s.tier, timed: s.timed, reason: s.reason}, true
}

func (dt *Detector) tie(a, b string, s scored) {
	if _, ok := dt.ties[a][b]; !ok {
		dt.kept++
	}
	for _, p := range [][2]string{{a, b}, {b, a}} {
		if dt.ties[p[0]] == nil {
			dt.ties[p[0]] = map[string]scored{}
		}
		dt.ties[p[0]][p[1]] = s
	}
}

func (dt *Detector) untie(a, b string) {
	if _, ok := dt.ties[a][b]; ok {
		dt.kept--
	}
	for _, p := range [][2]string{{a, b}, {b, a}} {
		delete(dt.ties[p[0]], p[1])
		if len(dt.ties[p[0]]) == 0 {
			delete(dt.ties, p[0])
		}
	}
}

func (dt *Detector) untieAll(id string) {
	dt.kept -= len(dt.ties[id])
	for other := range dt.ties[id] {
		delete(dt.ties[other], id)
		if len(dt.ties[other]) == 0 {
			delete(dt.ties, other)
		}
	}
	delete(dt.ties, id)
}

// differs is the sessions new or different since the last run.
func (dt *Detector) differs(members []Member) map[string]bool {
	differs := map[string]bool{}
	for _, m := range members {
		if last, ok := dt.members[m.ID]; !ok || !sameMember(last, m) {
			differs[m.ID] = true
		}
	}
	return differs
}

// changed is the sessions whose pairs must be scored again: those that
// differ, and those using a word whose rarity moved, found through the
// candidate index's words.
func (dt *Detector) changed(differs map[string]bool, members []Member, wording *Wording) map[string]bool {
	changed := make(map[string]bool, len(differs))
	for id := range differs {
		changed[id] = true
	}
	if wording.allMoved {
		for _, m := range members {
			changed[m.ID] = true
		}
		return changed
	}
	for tok := range wording.moved {
		for _, id := range dt.index.keys[indexKey{wordKey, tok}] {
			changed[id] = true
		}
	}
	return changed
}

// usesAny is whether words holds any of moved, looking up whichever of
// the two is smaller in the other.
func usesAny(words, moved map[string]bool) bool {
	if len(moved) < len(words) {
		words, moved = moved, words
	}
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
