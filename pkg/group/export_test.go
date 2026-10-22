package group

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

// Rarities are the words rare enough to weigh, and their weight.
func (w *Wording) Rarities() map[string]float64 { return w.idf }

// Counts are how many sessions use each word.
func (w *Wording) Counts() map[string]int { return w.df }

// Moved is whether every word's rarity moved in the last Next, and
// otherwise the words whose rarity moved, sorted.
func (w *Wording) Moved() (bool, []string) { return w.allMoved, sortedKeys(w.moved) }
