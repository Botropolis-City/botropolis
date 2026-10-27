package group

import (
	"fmt"
	"slices"
	"strings"
)

// DetectEveryPair is item 158's detector from scratch, scoring every
// pair of sessions: the oracle the incremental detector is held to.
var DetectEveryPair = detectEveryPair

// EdgeView is one edge as a test sees it.
type EdgeView struct {
	A, B   string
	Tier   Tier
	Reason Reason
}

func viewEdges(d *detection, edges []edge) []EdgeView {
	out := make([]EdgeView, 0, len(edges))
	for _, e := range edges {
		out = append(out, EdgeView{A: d.members[e.a].ID, B: d.members[e.b].ID, Tier: e.tier, Reason: e.reason})
	}
	return out
}

// EdgesEveryPair are the edges found by scoring every pair, in the order
// detection takes them.
func EdgesEveryPair(members []Member, memory Memory) []EdgeView {
	d := newDetection(sortedMembers(members), memory, nil)
	return viewEdges(d, d.oracleEdges())
}

// DetectEdges runs dt as Detect does, and gives the edges it took.
func DetectEdges(dt *Detector, members []Member, memory Memory) []EdgeView {
	d, edges := dt.run(members, memory)
	return viewEdges(d, edges)
}

// CandidateIndex is the candidate index the detector keeps between runs:
// each key's sessions, sorted, and the closed sessions in order.
func (dt *Detector) CandidateIndex() map[string][]string {
	return dt.index.view()
}

func (ix *candidateIndex) view() map[string][]string {
	out := map[string][]string{}
	for k, ids := range ix.keys {
		out[fmt.Sprintf("%d %s", k.kind, k.key)] = slices.Sorted(slices.Values(ids))
	}
	for h, ids := range ix.pastes {
		out[fmt.Sprintf("paste %x", h)] = slices.Sorted(slices.Values(ids))
	}
	ends := slices.Clone(ix.ends)
	slices.SortFunc(ends, func(a, b string) int {
		if c := ix.under[a].ended.Compare(ix.under[b].ended); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	if len(ends) > 0 {
		out["ends"] = ends
	}
	return out
}

// Rarities are the words rare enough to weigh, and their weight.
func (w *Wording) Rarities() map[string]float64 { return w.idf }

// Counts are how many sessions use each word.
func (w *Wording) Counts() map[string]int { return w.df }

// Moved is whether every word's rarity moved in the last Next, and
// otherwise the words whose rarity moved, sorted.
func (w *Wording) Moved() (bool, []string) { return w.allMoved, sortedKeys(w.moved) }

// IndexSums gives, for two sessions among members, the rarity the
// candidate index adds up as the first's candidates are found, and what
// the wording weighs the pair at (item 270).
func IndexSums(members []Member) func(a, b string) (sum, shared float64) {
	dt := NewDetector()
	d, _ := dt.run(members, Memory{})
	c := d.candidates(dt.index)
	return func(a, b string) (float64, float64) {
		i, j := d.index[a], d.index[b]
		c.of(i)
		sum := 0.0
		if c.seen[j] == i+1 {
			sum = c.shared[j]
		}
		return sum, d.wording.sharedScore(a, b)
	}
}

// WordingWeighed is how many pairs the detector has weighed the wording
// of.
func (dt *Detector) WordingWeighed() int { return dt.weighed }
