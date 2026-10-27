package group

import (
	"math"
	"slices"
	"sort"
	"strings"
	"time"
)

// candidates are a run's view of the detector's candidate index, which
// holds every member under every signal pair reads, so that a session's
// candidates, the sessions it shares at least one signal with, are found
// without enumerating every pair (item 198). A pair that is not a
// candidate cannot score: pair needs a shared directory, reference,
// team, parent, message, paste, rare word, branch or file, or two
// starts, or a close and a start, close enough together.
type candidates struct {
	d    *detection
	ix   *candidateIndex
	seen []int
	out  []int
	// keyed is which candidates the last of found by a key other than a
	// word, and shared the rarity of the words each shares, which is
	// what the wording weighs the pair at (item 270).
	keyed            []int
	shared, sharedSq []float64
	// weights are each member's words' squared rarities summed, once
	// weighed.
	weights []float64
	weighed []bool
}

// The kinds of key a member is indexed under. dirAt and visitAt are a
// path the member's directory, or one of its visits, is or lies under;
// dirIs and visitIs are exactly that path.
const (
	dirIs = iota
	dirAt
	visitIs
	visitAt
	refKey
	teamKey
	childOf
	messagedBy
	wordKey
	branchKey
	fileKey
	kinds
)

type indexKey struct {
	kind int
	key  string
}

// candidateIndex is kept from run to run and patched for the sessions
// that changed, so that a prompt in one session indexes what that
// session says and not the whole history (item 210). It holds every
// word, rare or not, and a lookup takes only the rare ones, since a
// word's rarity can move without the sessions using it changing.
type candidateIndex struct {
	keys   map[indexKey][]string
	pastes map[uint64][]string
	// ends are the members that closed, by when they closed.
	ends  []string
	under map[string]indexed
}

// indexed is what a member was indexed under, to take it out again.
type indexed struct {
	keys   []indexKey
	pastes pasteKeys
	ended  time.Time
}

func newCandidateIndex() *candidateIndex {
	return &candidateIndex{keys: map[indexKey][]string{}, pastes: map[uint64][]string{}, under: map[string]indexed{}}
}

// indexedUnder gives every key member i is indexed under.
func (d *detection) indexedUnder(i int, key func(kind int, key string)) {
	m := d.members[i]
	key(dirIs, m.Dir)
	for _, at := range atOrAbove(m.Dir) {
		key(dirAt, at)
	}
	for _, v := range m.Visited {
		key(visitIs, v.Dir)
		for _, at := range atOrAbove(v.Dir) {
			key(visitAt, at)
		}
	}
	for _, r := range d.refs[i] {
		key(refKey, r)
	}
	if m.Team != "" {
		key(teamKey, m.Team)
	}
	if m.Parent != "" {
		key(childOf, m.Parent)
	}
	for _, to := range m.Messaged {
		key(messagedBy, to)
	}
	for tok := range d.wording.words[m.ID] {
		key(wordKey, tok)
	}
	for _, b := range branchKeys(m.Branch) {
		key(branchKey, b)
	}
	for _, f := range m.Files {
		key(fileKey, f)
	}
}

// looksUp gives every key member i's candidates are indexed under.
func (d *detection) looksUp(i int, key func(kind int, key string)) {
	m := d.members[i]
	for _, v := range m.Visited {
		key(dirAt, v.Dir)
		for _, a := range above(v.Dir) {
			key(dirIs, a)
		}
	}
	key(visitAt, m.Dir)
	for _, a := range above(m.Dir) {
		key(visitIs, a)
	}
	for _, r := range d.refs[i] {
		key(refKey, r)
	}
	if m.Team != "" {
		key(teamKey, m.Team)
	}
	key(childOf, m.ID)
	key(messagedBy, m.ID)
	for tok := range d.wording.words[m.ID] {
		key(wordKey, tok)
	}
	for _, b := range branchKeys(m.Branch) {
		key(branchKey, b)
	}
	for _, f := range m.Files {
		key(fileKey, f)
	}
}

// update brings the index to d's members: all of them indexed again when
// rebuild, otherwise only the sessions differing from the last run, and
// those that left taken out.
func (ix *candidateIndex) update(d *detection, differs map[string]bool, rebuild bool) {
	if rebuild {
		last := ix.under
		*ix = *newCandidateIndex()
		for i, m := range d.members {
			ix.add(d, i, last[m.ID].pastes)
		}
		return
	}
	for id := range ix.under {
		if _, ok := d.index[id]; !ok {
			ix.remove(id)
		}
	}
	for id := range differs {
		last := ix.under[id].pastes
		ix.remove(id)
		ix.add(d, d.index[id], last)
	}
}

func (ix *candidateIndex) add(d *detection, i int, last pasteKeys) {
	m := d.members[i]
	var in indexed
	d.indexedUnder(i, func(kind int, key string) {
		k := indexKey{kind, key}
		ix.keys[k] = append(ix.keys[k], m.ID)
		in.keys = append(in.keys, k)
	})
	if len(m.Pasted) > 0 {
		in.pastes = last
		if !slices.Equal(last.pasted, m.Pasted) {
			in.pastes = readPasteKeys(m.Pasted)
		}
		for _, h := range in.pastes.prefix {
			ix.pastes[h] = append(ix.pastes[h], m.ID)
		}
	}
	if in.ended = m.Ended; !in.ended.IsZero() {
		at, _ := slices.BinarySearchFunc(ix.ends, in.ended, func(id string, t time.Time) int {
			return ix.under[id].ended.Compare(t)
		})
		ix.ends = slices.Insert(ix.ends, at, m.ID)
	}
	ix.under[m.ID] = in
}

func (ix *candidateIndex) remove(id string) {
	in, ok := ix.under[id]
	if !ok {
		return
	}
	for _, k := range in.keys {
		if ids := without(ix.keys[k], id); len(ids) > 0 {
			ix.keys[k] = ids
		} else {
			delete(ix.keys, k)
		}
	}
	for _, h := range in.pastes.prefix {
		if ids := without(ix.pastes[h], id); len(ids) > 0 {
			ix.pastes[h] = ids
		} else {
			delete(ix.pastes, h)
		}
	}
	if !in.ended.IsZero() {
		ix.ends = slices.DeleteFunc(ix.ends, func(e string) bool { return e == id })
	}
	delete(ix.under, id)
}

// without is ids with id taken out, once.
func without(ids []string, id string) []string {
	for k, x := range ids {
		if x == id {
			ids[k] = ids[len(ids)-1]
			return ids[:len(ids)-1]
		}
	}
	return ids
}

func (d *detection) candidates(ix *candidateIndex) *candidates {
	n := len(d.members)
	return &candidates{d: d, ix: ix, seen: make([]int, n), keyed: make([]int, n), shared: make([]float64, n),
		sharedSq: make([]float64, n), weights: make([]float64, n), weighed: make([]bool, n)}
}

// of is member i's candidates, each once, in no particular order, valid
// until the next call.
func (c *candidates) of(i int) []int {
	d, m := c.d, c.d.members[i]
	stamp := i + 1
	out := c.out[:0]
	defer func() { c.out = out }()
	found := func(j int) {
		if c.seen[j] != stamp {
			c.seen[j] = stamp
			c.shared[j], c.sharedSq[j] = 0, 0
			out = append(out, j)
		}
	}
	addAt := func(j int) {
		if j != i {
			found(j)
			c.keyed[j] = stamp
		}
	}
	add := func(ids []string) {
		for _, id := range ids {
			if j, ok := d.index[id]; ok {
				addAt(j)
			}
		}
	}
	d.looksUp(i, func(kind int, key string) {
		if kind == wordKey {
			idf, rare := d.wording.idf[key]
			if !rare {
				return
			}
			for _, id := range c.ix.keys[indexKey{kind, key}] {
				if j, ok := d.index[id]; ok && j != i {
					found(j)
					c.shared[j] += idf
					c.sharedSq[j] += idf * idf
				}
			}
			return
		}
		if d.isCommon(kind, key) {
			return
		}
		add(c.ix.keys[indexKey{kind, key}])
	})
	if j, ok := d.index[m.Parent]; ok {
		addAt(j)
	}
	add(m.Messaged)
	for _, h := range c.ix.under[m.ID].pastes.prefix {
		add(c.ix.pastes[h])
	}
	c.startedWithin(m.Started.Add(-burst), m.Started.Add(burst), addAt)
	if !m.Ended.IsZero() {
		c.startedWithin(m.Ended, m.Ended.Add(continuing), addAt)
	}
	add(c.ix.endedWithin(m.Started.Add(-continuing), m.Started))
	// A candidate found only through words can be tied only by its
	// wording, Wording.Related, and every other signal a pair is scored
	// on has a key of its own. The walk above adds up what Related reads,
	// the rarity the two share and its square, so a candidate that falls
	// short of either of its thresholds is left unscored; one within a
	// rounding of them is scored, so the sums' order can change nothing.
	// On a heavy history, words each in a sixth of the sessions made
	// nearly every pair a candidate (item 270).
	const margin = 1e-9
	return slices.DeleteFunc(out, func(j int) bool {
		if c.keyed[j] == stamp {
			return false
		}
		if c.shared[j] < wordingThreshold-margin {
			return true
		}
		return c.sharedSq[j] < (wordingOverlap-margin)*math.Sqrt(c.weight(i)*c.weight(j))
	})
}

// weight is what Wording.overlap weighs a member's words at: the sum of
// their rarities' squares, worked out once a run.
func (c *candidates) weight(i int) float64 {
	if !c.weighed[i] {
		sum := 0.0
		for tok := range c.d.wording.words[c.d.members[i].ID] {
			idf := c.d.wording.idf[tok]
			sum += idf * idf
		}
		c.weights[i], c.weighed[i] = sum, true
	}
	return c.weights[i]
}

// startedWithin gives the members started from one moment to another,
// both included.
func (c *candidates) startedWithin(from, to time.Time, add func(int)) {
	ms := c.d.members
	lo := sort.Search(len(ms), func(k int) bool { return !ms[k].Started.Before(from) })
	for k := lo; k < len(ms) && !ms[k].Started.After(to); k++ {
		add(k)
	}
}

// endedWithin are the members closed from one moment to another, both
// included.
func (ix *candidateIndex) endedWithin(from, to time.Time) []string {
	lo := sort.Search(len(ix.ends), func(k int) bool { return !ix.under[ix.ends[k]].ended.Before(from) })
	hi := sort.Search(len(ix.ends), func(k int) bool { return ix.under[ix.ends[k]].ended.After(to) })
	if lo >= hi {
		return nil
	}
	return ix.ends[lo:hi]
}

// above are the paths a directory lies under, as coordinates matches
// them: everything before each slash.
func above(dir string) []string {
	var out []string
	for k := 0; k < len(dir); k++ {
		if dir[k] == '/' {
			out = append(out, dir[:k])
		}
	}
	return out
}

func atOrAbove(dir string) []string {
	return append(above(dir), dir)
}

// branchKeys are what sameBranch matches a branch on: the branch itself
// and the references in it, none for a default branch.
func branchKeys(branch string) []string {
	if defaultBranches[branch] {
		return nil
	}
	return append([]string{"branch:" + branch}, prefixed("ref:", References(strings.ToUpper(branch)))...)
}

func prefixed(prefix string, xs []string) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = prefix + x
	}
	return out
}

// pasteKeys are the hashes that index a session's pastes: for each
// paste, the lowest of its shingles. Two pastes SamePaste calls
// near-duplicates share at least a share pasteSimilarity of the larger
// one's shingles, so the lowest |x| - floor(pasteSimilarity·|x|) + 1 of
// each must meet (prefix filtering); floor makes the prefix no shorter
// than it needs to be.
type pasteKeys struct {
	pasted []string
	prefix []uint64
}

func readPasteKeys(pasted []string) pasteKeys {
	k := pasteKeys{pasted: append([]string(nil), pasted...)}
	for _, p := range pasted {
		set := shingles(p)
		if set == nil {
			continue
		}
		hashes := make([]uint64, 0, len(set))
		for h := range set {
			hashes = append(hashes, h)
		}
		sort.Slice(hashes, func(x, y int) bool { return hashes[x] < hashes[y] })
		n := len(hashes) - int(math.Floor(pasteSimilarity*float64(len(hashes)))) + 1
		k.prefix = append(k.prefix, hashes[:min(n, len(hashes))]...)
	}
	return k
}
