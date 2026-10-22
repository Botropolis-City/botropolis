package group

import (
	"slices"
	"sort"
)

// Detector groups sessions run after run, keeping each pair's score and
// each session's words from the last run, so that a prompt in one
// session scores again only the pairs it can have changed (item 158).
// Detect is a fresh Detector, so what it finds is what a Detector finds.
type Detector struct {
	wording *Wording
	refs    map[string]readRefs
	members map[string]Member
	pairs   map[[2]string]scored
	scored  int
}

type scored struct {
	tier   Tier
	reason Reason
	ok     bool
}

func NewDetector() *Detector {
	return &Detector{}
}

// PairsScored is how many pairs of sessions the detector has compared.
func (dt *Detector) PairsScored() int {
	return dt.scored
}

// Detect groups members as group.Detect does. A pair is scored again
// when either session changed or the rarity of any word either uses did;
// when the number of sessions changes every word's rarity does, and so
// every pair is scored again.
func (dt *Detector) Detect(members []Member, memory Memory) Result {
	members = append([]Member(nil), members...)
	sort.SliceStable(members, func(i, j int) bool { return members[i].Started.Before(members[j].Started) })
	d := newDetection(members, memory, dt)
	d.changed = dt.changed(members, d.wording)
	d.cache = dt.pairs
	d.next = make(map[[2]string]scored, len(dt.pairs))
	result := d.detect(memory)
	dt.scored += d.scored
	dt.pairs, dt.wording, dt.refs = d.next, d.wording, d.read
	dt.members = make(map[string]Member, len(members))
	for _, m := range members {
		dt.members[m.ID] = m
	}
	return result
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

// scorePair is pair, from the last run's score when neither session has
// changed since.
func (d *detection) scorePair(i, j int) (edge, bool) {
	key := [2]string{d.members[i].ID, d.members[j].ID}
	if s, ok := d.cache[key]; ok && !d.changed[key[0]] && !d.changed[key[1]] {
		d.next[key] = s
		return edge{a: i, b: j, tier: s.tier, reason: s.reason}, s.ok
	}
	e, ok := d.pair(i, j)
	d.scored++
	d.next[key] = scored{tier: e.tier, reason: e.reason, ok: ok}
	return e, ok
}
