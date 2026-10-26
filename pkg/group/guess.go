package group

import "sort"

// A Guess is the group New session proposes for a session about to
// start, weighed by detection's own signals and tiers: a reference in
// the prompt is sure, and being in one of the group's sessions minutes
// ago or shared wording alone is only possible (items 87 and 271).
type Guess struct {
	Group  string
	Tier   Tier
	Reason Reason
	// Alternatives are the groups to walk with Tab, the guess first,
	// then the others by how strongly they matched.
	Alternatives []string
}

// A Guesser detects the history once and then weighs one prospective
// session against it as often as its prompt changes.
type Guesser struct {
	members []Member
	memory  Memory
	result  Result
}

func NewGuesser(members []Member, memory Memory) *Guesser {
	sorted := append([]Member(nil), members...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Started.Before(sorted[j].Started) })
	return &Guesser{members: sorted, memory: memory, result: Detect(sorted, memory)}
}

// Groups are the groups the guesser detected.
func (g *Guesser) Groups() []Group {
	return g.result.Groups
}

func (g *Guesser) Guess(prospect Member) Guess {
	all := append(append([]Member(nil), g.members...), prospect)
	d := newDetection(all, g.memory, nil)
	p := len(all) - 1
	type match struct {
		group string
		edge  edge
		order int
	}
	var matches []match
	for order, gr := range g.result.Groups {
		best := match{group: gr.ID, order: order}
		for _, m := range gr.Members {
			i, ok := d.index[m.Session]
			if !ok {
				continue
			}
			e, ok := d.pair(i, p)
			if !ok {
				continue
			}
			e.reason.With = m.Session
			if stronger(e, best.edge) {
				best.edge = e
			}
		}
		matches = append(matches, best)
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if stronger(matches[i].edge, matches[j].edge) {
			return true
		}
		if stronger(matches[j].edge, matches[i].edge) {
			return false
		}
		return matches[i].order < matches[j].order
	})
	var guess Guess
	for _, m := range matches {
		guess.Alternatives = append(guess.Alternatives, m.group)
	}
	if len(matches) > 0 && matches[0].edge.tier > 0 {
		guess.Group, guess.Tier, guess.Reason = matches[0].group, matches[0].edge.tier, matches[0].edge.reason
	}
	return guess
}

// stronger reports whether a is a better tie than b: a higher tier, or
// the same tier on a stronger signal.
func stronger(a, b edge) bool {
	if a.tier != b.tier {
		return a.tier > b.tier
	}
	if a.tier == 0 {
		return false
	}
	return signalRank[a.reason.Signal] < signalRank[b.reason.Signal]
}
