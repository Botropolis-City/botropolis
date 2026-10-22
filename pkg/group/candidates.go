package group

import (
	"math"
	"sort"
	"strings"
	"time"
)

// candidates index the members by every signal pair reads, so that a
// session's candidates, the sessions it shares at least one signal
// with, are found without enumerating every pair (item 198). A pair
// that is not a candidate cannot score: pair needs a shared directory,
// reference, team, parent, message, paste, rare word, branch or file,
// or two starts, or a close and a start, close enough together.
//
// Only the keys the changed sessions look up are indexed, so a prompt in
// one session indexes what that session says and not the whole history.
type candidates struct {
	d      *detection
	seen   []int
	out    []int
	idx    [kinds]map[string][]int
	pastes map[uint64][]int
	keys   map[string]pasteKeys
	// ends are the members that closed, by when they closed.
	ends []int
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

// indexedUnder gives every key member i is indexed under, its words
// only when asked to.
func (d *detection) indexedUnder(i int, words bool, key func(kind int, key string)) {
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
	if words {
		for tok := range d.wording.words[m.ID] {
			if _, ok := d.wording.idf[tok]; ok {
				key(wordKey, tok)
			}
		}
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

func (d *detection) candidates(keys map[string]pasteKeys, changed map[string]bool) *candidates {
	c := &candidates{d: d, seen: make([]int, len(d.members)), keys: keys, pastes: map[uint64][]int{}}
	for k := range c.idx {
		c.idx[k] = map[string][]int{}
	}
	all := len(changed) >= len(d.members)
	var want [kinds]map[string]bool
	if !all {
		for k := range want {
			want[k] = map[string]bool{}
		}
		for id := range changed {
			d.looksUp(d.index[id], func(kind int, key string) { want[kind][key] = true })
		}
	}
	for i, m := range d.members {
		d.indexedUnder(i, all, func(kind int, key string) {
			if all || want[kind][key] {
				c.idx[kind][key] = append(c.idx[kind][key], i)
			}
		})
		if !all {
			words, wanted := d.wording.words[m.ID], want[wordKey]
			if len(wanted) < len(words) {
				words, wanted = wanted, words
			}
			for tok := range words {
				if _, rare := d.wording.idf[tok]; rare && wanted[tok] {
					c.idx[wordKey][tok] = append(c.idx[wordKey][tok], i)
				}
			}
		}
		for _, h := range keys[m.ID].prefix {
			c.pastes[h] = append(c.pastes[h], i)
		}
		if !m.Ended.IsZero() {
			c.ends = append(c.ends, i)
		}
	}
	sort.SliceStable(c.ends, func(x, y int) bool { return d.members[c.ends[x]].Ended.Before(d.members[c.ends[y]].Ended) })
	return c
}

// of is member i's candidates, each once, in no particular order, valid
// until the next call.
func (c *candidates) of(i int) []int {
	d, m := c.d, c.d.members[i]
	stamp := i + 1
	out := c.out[:0]
	defer func() { c.out = out }()
	add := func(js []int) {
		for _, j := range js {
			if j != i && c.seen[j] != stamp {
				c.seen[j] = stamp
				out = append(out, j)
			}
		}
	}
	d.looksUp(i, func(kind int, key string) { add(c.idx[kind][key]) })
	if j, ok := d.index[m.Parent]; ok {
		add([]int{j})
	}
	for _, to := range m.Messaged {
		if j, ok := d.index[to]; ok {
			add([]int{j})
		}
	}
	for _, h := range c.keys[m.ID].prefix {
		add(c.pastes[h])
	}
	add(c.startedWithin(m.Started.Add(-burst), m.Started.Add(burst)))
	if !m.Ended.IsZero() {
		add(c.startedWithin(m.Ended, m.Ended.Add(continuing)))
	}
	add(c.endedWithin(m.Started.Add(-continuing), m.Started))
	return out
}

// startedWithin are the members started from one moment to another,
// both included.
func (c *candidates) startedWithin(from, to time.Time) []int {
	ms := c.d.members
	lo := sort.Search(len(ms), func(k int) bool { return !ms[k].Started.Before(from) })
	hi := sort.Search(len(ms), func(k int) bool { return ms[k].Started.After(to) })
	if lo >= hi {
		return nil
	}
	out := make([]int, 0, hi-lo)
	for k := lo; k < hi; k++ {
		out = append(out, k)
	}
	return out
}

// endedWithin are the members closed from one moment to another, both
// included.
func (c *candidates) endedWithin(from, to time.Time) []int {
	ms := c.d.members
	lo := sort.Search(len(c.ends), func(k int) bool { return !ms[c.ends[k]].Ended.Before(from) })
	hi := sort.Search(len(c.ends), func(k int) bool { return ms[c.ends[k]].Ended.After(to) })
	if lo >= hi {
		return nil
	}
	return c.ends[lo:hi]
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
