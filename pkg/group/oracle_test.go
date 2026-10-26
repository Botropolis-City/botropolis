package group

import "sort"

// The oracle is item 158's detector as it was before item 198 (r905):
// every pair scored, and its own sort, groups, memberships, remembered
// ids, names and suggestions, kept here unchanged so that rewriting
// those for speed is held to what they did.

func detectEveryPair(members []Member, memory Memory) Result {
	d := newDetection(sortedMembers(members), memory, nil)
	edges := d.oracleEdges()
	comps := d.oracleCluster(edges)
	groups := d.oracleGroups(comps, edges, memory)
	groups = d.placeMoved(groups, memory)
	groups = d.joinStarted(groups, memory)
	result := Result{Groups: groups, Capped: d.capped}
	result.Suggestions = d.oracleSuggest(groups, edges)
	return result
}

func (d *detection) oracleEdges() []edge {
	var edges []edge
	for i := range d.members {
		for j := i + 1; j < len(d.members); j++ {
			if d.blocked[[2]string{d.members[i].ID, d.members[j].ID}] || d.sitsOut(d.members[i].ID) || d.sitsOut(d.members[j].ID) {
				continue
			}
			if e, ok := d.pair(i, j); ok {
				edges = append(edges, e)
			}
		}
	}
	sort.SliceStable(edges, func(x, y int) bool {
		if edges[x].tier != edges[y].tier {
			return edges[x].tier > edges[y].tier
		}
		if edges[x].timed != edges[y].timed {
			return !edges[x].timed
		}
		return signalRank[edges[x].reason.Signal] < signalRank[edges[y].reason.Signal]
	})
	return edges
}

func (d *detection) oracleGroups(comps [][]int, edges []edge, memory Memory) []Group {
	taken := map[string]bool{}
	groups := make([]Group, 0, len(comps))
	for _, comp := range comps {
		in := map[int]bool{}
		for _, i := range comp {
			in[i] = true
		}
		g := Group{}
		for _, i := range comp {
			g.Members = append(g.Members, d.oracleMembership(i, in, edges))
		}
		stored, ok := oracleMatchStored(comp, d, memory, taken)
		if ok {
			taken[stored.ID] = true
			g.ID = stored.ID
		} else {
			g.ID = newID(d.members[comp[0]].ID)
		}
		d.name(&g)
		if ok && stored.Renamed {
			g.Name, g.Renamed = stored.Name, true
		}
		groups = append(groups, g)
	}
	return groups
}

func (d *detection) oracleMembership(i int, in map[int]bool, edges []edge) Membership {
	m := Membership{Session: d.members[i].ID, Dir: d.members[i].Dir, Started: d.members[i].Started}
	for _, e := range edges {
		other := -1
		switch i {
		case e.a:
			other = e.b
		case e.b:
			other = e.a
		}
		if other < 0 || !in[other] || e.tier < Likely {
			continue
		}
		m.Tier, m.Reason = e.tier, e.reason
		m.Reason.With = d.members[other].ID
		break
	}
	if m.Tier == Likely && d.confirmed[m.Session] {
		m.Tier = Sure
	}
	return m
}

func oracleMatchStored(comp []int, d *detection, memory Memory, taken map[string]bool) (Stored, bool) {
	best, bestOverlap := Stored{}, 0
	for _, s := range memory.Groups {
		if taken[s.ID] {
			continue
		}
		overlap := 0
		for _, i := range comp {
			if contains(s.Members, d.members[i].ID) {
				overlap++
			}
		}
		if overlap > bestOverlap {
			best, bestOverlap = s, overlap
		}
	}
	return best, bestOverlap > 0
}

func (d *detection) oracleSuggest(groups []Group, edges []edge) []Suggestion {
	var out []Suggestion
	seen := map[[2]string]bool{}
	for _, e := range edges {
		if e.tier != Possible {
			continue
		}
		for _, p := range [][2]int{{e.a, e.b}, {e.b, e.a}} {
			loose, member := d.members[p[0]].ID, d.members[p[1]].ID
			if _, in := (Result{Groups: groups}).GroupOf(loose); in {
				continue
			}
			g, ok := Result{Groups: groups}.GroupOf(member)
			if !ok || seen[[2]string{loose, g.ID}] {
				continue
			}
			seen[[2]string{loose, g.ID}] = true
			reason := e.reason
			reason.With = member
			out = append(out, Suggestion{Session: loose, Group: g.ID, Reason: reason})
		}
	}
	return out
}

func (d *detection) oracleCluster(edges []edge) [][]int {
	comp := make([]int, len(d.members))
	sets := map[int][]int{}
	for i := range d.members {
		comp[i] = i
		sets[i] = []int{i}
	}
	joinedOnTiming := map[int]bool{}
	left := map[int]bool{}
	d.capped = nil
	for _, e := range edges {
		if e.tier < Likely || left[e.a] || left[e.b] {
			continue
		}
		ca, cb := comp[e.a], comp[e.b]
		if ca == cb {
			continue
		}
		na, nb := len(sets[ca]), len(sets[cb])
		if e.timed {
			hop := (na == 1 && nb == 1) || (na == 1 && !joinedOnTiming[e.b]) || (nb == 1 && !joinedOnTiming[e.a])
			if !hop {
				continue
			}
		}
		if d.oracleAnyBlocked(sets[ca], sets[cb]) {
			continue
		}
		if na+nb > MaxMembers {
			if na == 1 || nb == 1 {
				loose, with := e.a, e.b
				if na != 1 {
					loose, with = e.b, e.a
				}
				left[loose] = true
				reason := e.reason
				reason.With = d.members[with].ID
				d.capped = append(d.capped, Capped{Session: d.members[loose].ID, Reason: reason})
			}
			continue
		}
		if e.timed {
			joinedOnTiming[e.a], joinedOnTiming[e.b] = true, true
		}
		for _, m := range sets[cb] {
			comp[m] = ca
		}
		sets[ca] = append(sets[ca], sets[cb]...)
		delete(sets, cb)
	}
	var comps [][]int
	for _, set := range sets {
		if len(set) > 1 {
			sort.Ints(set)
			comps = append(comps, set)
		}
	}
	sort.Slice(comps, func(i, j int) bool { return comps[i][0] < comps[j][0] })
	return comps
}

func (d *detection) oracleAnyBlocked(xs, ys []int) bool {
	for _, x := range xs {
		for _, y := range ys {
			if d.blocked[[2]string{d.members[x].ID, d.members[y].ID}] {
				return true
			}
		}
	}
	return false
}
