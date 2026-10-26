package group

import (
	"cmp"
	"fmt"
	"hash/fnv"
	"slices"
	"sort"
	"strings"
	"time"
)

var defaultBranches = map[string]bool{"": true, "HEAD": true, "main": true, "master": true, "trunk": true, "develop": true}

// signalRank orders the reasons a session joined, strongest first.
var signalRank = map[Signal]int{
	SignalCoordinates: -1, SignalStartedFrom: 0, SignalContinuing: 1, SignalReference: 2, SignalParentage: 3,
	SignalPasted: 4, SignalWording: 5, SignalBranch: 6, SignalFiles: 7, SignalTiming: 8,
}

// Detect groups sessions by what they say and how they were started,
// strongest signal first, keeping the ids and names memory already has
// and never joining a pair marked not related.
func Detect(members []Member, memory Memory) Result {
	return NewDetector().Detect(members, memory)
}

func (d *detection) detect(memory Memory, edges []edge) Result {
	comps := d.cluster(edges)
	groups := d.groups(comps, edges, memory)
	groups = d.placeMoved(groups, memory)
	groups = d.joinStarted(groups, memory)
	result := Result{Groups: groups, Capped: d.capped}
	result.Suggestions = d.suggest(groups, edges)
	return result
}

func newDetection(members []Member, memory Memory, prior *Detector) *detection {
	d := &detection{members: members, index: make(map[string]int, len(members)), blocked: map[[2]string]bool{},
		moved: map[string]bool{}, confirmed: map[string]bool{}, refs: make([][]string, 0, len(members))}
	for _, mv := range memory.Moves {
		d.moved[mv.Session] = true
	}
	d.started = map[string]string{}
	for _, m := range members {
		from := m.StartedFrom
		if from == "" && m.Job != "" {
			from = memory.StartedFrom[m.Job]
		}
		if from != "" && !d.moved[m.ID] {
			d.started[m.ID] = from
		}
	}
	for _, id := range memory.Confirmed {
		d.confirmed[id] = true
	}
	prompts := make(map[string][]string, len(members))
	if prior != nil && prior.refs != nil {
		d.read = prior.refs
	} else {
		d.read = make(map[string]readRefs, len(members))
	}
	for i, m := range members {
		d.index[m.ID] = i
		prompts[m.ID] = m.Prompts
		r := d.read[m.ID].next(m)
		d.read[m.ID] = r
		d.refs = append(d.refs, r.refs)
	}
	for id := range d.read {
		if _, ok := d.index[id]; !ok {
			delete(d.read, id)
		}
	}
	var wording *Wording
	if prior != nil {
		wording = prior.wording
	}
	d.wording = wording.Next(prompts, memory.Muted)
	for _, p := range memory.NotRelated {
		d.blocked[[2]string{p[0], p[1]}] = true
		d.blocked[[2]string{p[1], p[0]}] = true
	}
	return d
}

type keyedEdge struct {
	key uint64
	at  int
}

// sortEdges orders edges strongest first, then by their sessions'
// order, which is the order scoring every pair finds them in, into out,
// using keys; both are buffers kept from the last run, and grown.
func sortEdges(edges []edge, keys []keyedEdge, out []edge) ([]edge, []keyedEdge) {
	keys = keys[:0]
	for k, e := range edges {
		keys = append(keys, keyedEdge{key: edgeKey(e), at: k})
	}
	slices.SortFunc(keys, func(x, y keyedEdge) int { return cmp.Compare(x.key, y.key) })
	out = out[:0]
	for _, kd := range keys {
		out = append(out, edges[kd.at])
	}
	return out, keys
}

func edgeKey(e edge) uint64 {
	rank := uint64(signalRank[e.reason.Signal] + 1)
	var timed uint64
	if e.timed {
		timed = 1
	}
	return uint64(Sure-e.tier)<<60 | timed<<56 | rank<<52 | uint64(e.a)<<26 | uint64(e.b)
}

func (d *detection) pair(i, j int) (edge, bool) {
	a, b := d.members[i], d.members[j]
	e := edge{a: i, b: j}
	if coordinates(a, b) || coordinates(b, a) {
		e.tier, e.reason = Sure, Reason{Signal: SignalCoordinates}
		return e, true
	}
	if shared := intersect(d.refs[i], d.refs[j]); len(shared) > 0 {
		e.tier, e.reason = Sure, Reason{Signal: SignalReference, Detail: shared[0]}
		return e, true
	}
	if related(a, b) {
		e.tier, e.reason = Sure, Reason{Signal: SignalParentage}
		return e, true
	}
	if samePaste(a, b) {
		e.tier, e.reason = Likely, Reason{Signal: SignalPasted}
		return e, true
	}
	when, gap := timing(a, b)
	var weak []Reason
	if d.wording.Related(a.ID, b.ID) {
		words, _ := d.wording.Shared(a.ID, b.ID)
		weak = append(weak, Reason{Signal: SignalWording, Detail: strings.Join(words, " ")})
	}
	if key, ok := sameBranch(a.Branch, b.Branch); ok {
		weak = append(weak, Reason{Signal: SignalBranch, Detail: key})
	}
	if files := intersect(a.Files, b.Files); len(files) > 0 {
		weak = append(weak, Reason{Signal: SignalFiles, Detail: files[0]})
	}
	timed := (when == SignalTiming && gap <= burst) || (when == SignalContinuing && gap <= continuing)
	if timed {
		weak = append(weak, Reason{Signal: when})
	}
	switch {
	case len(weak) >= 2:
		e.tier, e.timed = Likely, timed && len(weak) == 2
	case len(weak) == 1:
		e.tier = Possible
	default:
		return e, false
	}
	e.reason = weak[0]
	return e, true
}

// timing is how close two sessions were started: continuing, when the
// later one started after the earlier closed, measured from the close;
// otherwise between the two starts. a starts no later than b.
func timing(a, b Member) (Signal, time.Duration) {
	if !a.Ended.IsZero() && !b.Started.Before(a.Ended) {
		return SignalContinuing, b.Started.Sub(a.Ended)
	}
	return SignalTiming, b.Started.Sub(a.Started)
}

// coordinates is whether a's subagents worked in b's directory while b
// ran: the visit itself, not a's whole run, has to overlap b's.
func coordinates(a, b Member) bool {
	for _, v := range a.Visited {
		if v.Dir != b.Dir && !strings.HasPrefix(b.Dir, v.Dir+"/") && !strings.HasPrefix(v.Dir, b.Dir+"/") {
			continue
		}
		to := v.To
		if to.Before(v.From) {
			to = v.From
		}
		if b.Started.After(to) || (!b.Ended.IsZero() && b.Ended.Before(v.From)) {
			continue
		}
		return true
	}
	return false
}

func related(a, b Member) bool {
	if a.Team != "" && a.Team == b.Team {
		return true
	}
	if a.Parent == b.ID || b.Parent == a.ID {
		return true
	}
	return contains(a.Messaged, b.ID) || contains(b.Messaged, a.ID)
}

func samePaste(a, b Member) bool {
	for _, pa := range a.Pasted {
		for _, pb := range b.Pasted {
			if SamePaste(pa, pb) {
				return true
			}
		}
	}
	return false
}

func sameBranch(a, b string) (string, bool) {
	if defaultBranches[a] || defaultBranches[b] {
		return "", false
	}
	if a == b {
		return a, true
	}
	if shared := intersect(References(strings.ToUpper(a)), References(strings.ToUpper(b))); len(shared) > 0 {
		return shared[0], true
	}
	return "", false
}

// cluster joins sessions along sure and likely edges, strongest first,
// refusing any join that would put a not-related pair in one group. A
// timed edge, which comes after every other, joins a session on its own
// to another on its own, or to a member that did not itself join on
// timing, so timing never chains (item 271). No group grows past
// MaxMembers: a session refused by a full group is left out of every
// group and reported as capped.
func (d *detection) cluster(edges []edge) [][]int {
	comp := make([]int, len(d.members))
	// sets are each group's sessions by the group's first, nil for a
	// session on its own.
	sets := make([][]int, len(d.members))
	timed := make([]bool, len(d.members))
	out := make([]bool, len(d.members))
	d.capped = nil
	for i := range d.members {
		comp[i] = i
	}
	for _, e := range edges {
		if e.tier < Likely || out[e.a] || out[e.b] {
			continue
		}
		ca, cb := comp[e.a], comp[e.b]
		if ca == cb {
			continue
		}
		if e.timed && !d.oneHop(e, sets[ca] == nil, sets[cb] == nil, timed) {
			continue
		}
		for _, c := range []int{ca, cb} {
			if sets[c] == nil {
				sets[c] = []int{c}
			}
		}
		if d.anyBlocked(sets[ca], sets[cb]) {
			continue
		}
		if len(sets[ca])+len(sets[cb]) > MaxMembers {
			d.capOut(e, sets[ca], sets[cb], out)
			continue
		}
		if e.timed {
			timed[e.a], timed[e.b] = true, true
		}
		for _, m := range sets[cb] {
			comp[m] = ca
		}
		sets[ca] = append(sets[ca], sets[cb]...)
		sets[cb] = nil
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

// oneHop is whether a timed edge may join: two sessions on their own, or
// one on its own and a grouped member that did not join on timing.
func (d *detection) oneHop(e edge, aloneA, aloneB bool, timed []bool) bool {
	switch {
	case aloneA && aloneB:
		return true
	case aloneA:
		return !timed[e.b]
	case aloneB:
		return !timed[e.a]
	}
	return false
}

// capOut records the join a full group refused: the session on its own
// is left out of every group, and two groups simply stay apart.
func (d *detection) capOut(e edge, xs, ys []int, out []bool) {
	var session, with int
	switch {
	case len(xs) == 1:
		session, with = e.a, e.b
	case len(ys) == 1:
		session, with = e.b, e.a
	default:
		return
	}
	out[session] = true
	reason := e.reason
	reason.With = d.members[with].ID
	d.capped = append(d.capped, Capped{Session: d.members[session].ID, Reason: reason})
}

func (d *detection) anyBlocked(xs, ys []int) bool {
	if len(d.blocked) == 0 {
		return false
	}
	for _, x := range xs {
		for _, y := range ys {
			if d.blocked[[2]string{d.members[x].ID, d.members[y].ID}] {
				return true
			}
		}
	}
	return false
}

func (d *detection) groups(comps [][]int, edges []edge, memory Memory) []Group {
	taken := map[string]bool{}
	groups := make([]Group, 0, len(comps))
	tie := d.strongestTies(comps, edges)
	stored := storedBySession(memory)
	for _, comp := range comps {
		g := Group{Members: make([]Membership, 0, len(comp))}
		for _, i := range comp {
			g.Members = append(g.Members, d.membership(i, tie[i]))
		}
		stored, ok := matchStored(comp, d, memory, stored, taken)
		if ok {
			taken[stored.ID] = true
			g.ID = stored.ID
		} else {
			g.ID = newID(d.members[comp[0]].ID)
		}
		d.names.name(d, &g)
		if ok && stored.Renamed {
			g.Name, g.Renamed = stored.Name, true
		}
		groups = append(groups, g)
	}
	return groups
}

// strongestTies are, by member, the first edge in order, sure or likely,
// that ties it to another member of its group, if any.
func (d *detection) strongestTies(comps [][]int, edges []edge) []*edge {
	compOf := make([]int, len(d.members))
	for i := range compOf {
		compOf[i] = -1
	}
	for c, comp := range comps {
		for _, i := range comp {
			compOf[i] = c
		}
	}
	tie := make([]*edge, len(d.members))
	for k := range edges {
		e := &edges[k]
		if e.tier < Likely || compOf[e.a] < 0 || compOf[e.a] != compOf[e.b] {
			continue
		}
		for _, i := range []int{e.a, e.b} {
			if tie[i] == nil {
				tie[i] = e
			}
		}
	}
	return tie
}

// membership takes a member's strongest tie inside its group as its tier
// and its reason.
func (d *detection) membership(i int, e *edge) Membership {
	m := Membership{Session: d.members[i].ID, Dir: d.members[i].Dir, Started: d.members[i].Started}
	if e != nil {
		other := e.a
		if other == i {
			other = e.b
		}
		m.Tier, m.Reason = e.tier, e.reason
		m.Reason.With = d.members[other].ID
	}
	if m.Tier == Likely && d.confirmed[m.Session] {
		m.Tier = Sure
	}
	return m
}

// placeMoved puts each session moved by hand into the group it was put
// in, made from memory when detection did not find it, or in none.
func (d *detection) placeMoved(groups []Group, memory Memory) []Group {
	for _, mv := range memory.Moves {
		i, ok := d.index[mv.Session]
		if !ok || mv.Group == "" {
			continue
		}
		m := d.members[i]
		g := groupByID(groups, mv.Group)
		if g == nil {
			for _, s := range memory.Groups {
				if s.ID == mv.Group {
					groups = append(groups, Group{ID: s.ID, Name: s.Name, Renamed: s.Renamed, Anchor: s.Anchor})
					g = &groups[len(groups)-1]
				}
			}
		}
		if g == nil {
			continue
		}
		g.Members = append(g.Members, Membership{Session: m.ID, Dir: m.Dir, Started: m.Started, Tier: Sure,
			Reason: Reason{Signal: SignalMoved}})
	}
	return groups
}

// storedBySession is, by session, the remembered groups it is in, by
// their place in memory, each once.
func storedBySession(memory Memory) map[string][]int {
	of := map[string][]int{}
	for si, s := range memory.Groups {
		for _, id := range s.Members {
			if in := of[id]; len(in) == 0 || in[len(in)-1] != si {
				of[id] = append(in, si)
			}
		}
	}
	return of
}

// matchStored is the remembered group not yet taken that holds most of
// comp's sessions, the first in memory of those holding as many.
func matchStored(comp []int, d *detection, memory Memory, stored map[string][]int, taken map[string]bool) (Stored, bool) {
	overlap := map[int]int{}
	var holding []int
	for _, i := range comp {
		for _, si := range stored[d.members[i].ID] {
			if !taken[memory.Groups[si].ID] {
				if overlap[si] == 0 {
					holding = append(holding, si)
				}
				overlap[si]++
			}
		}
	}
	slices.Sort(holding)
	best, bestOverlap := -1, 0
	for _, si := range holding {
		if overlap[si] > bestOverlap {
			best, bestOverlap = si, overlap[si]
		}
	}
	if best < 0 {
		return Stored{}, false
	}
	return memory.Groups[best], true
}

// NewGroupID is an id for a group made by hand, from a seed unique to it.
func NewGroupID(seed string) string {
	return newID(seed)
}

func newID(seed string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	return fmt.Sprintf("g-%08x", h.Sum32())
}

// joinStarted adds the loose sessions that were started from a group,
// making the group from memory when detection did not find it.
func (d *detection) joinStarted(groups []Group, memory Memory) []Group {
	for _, m := range d.members {
		from := d.started[m.ID]
		if from == "" || inAny(groups, m.ID) {
			continue
		}
		if groupByID(groups, from) == nil {
			for _, s := range memory.Groups {
				if s.ID == from {
					groups = append(groups, Group{ID: s.ID, Name: s.Name, Renamed: s.Renamed, Anchor: s.Anchor})
				}
			}
		}
		if g := groupByID(groups, from); g != nil {
			g.Members = append(g.Members, Membership{Session: m.ID, Dir: m.Dir, Started: m.Started, Tier: Sure,
				Reason: Reason{Signal: SignalStartedFrom, Detail: g.Name}})
		}
	}
	for gi := range groups {
		sort.SliceStable(groups[gi].Members, func(x, y int) bool {
			return groups[gi].Members[x].Started.Before(groups[gi].Members[y].Started)
		})
	}
	return groups
}

func (d *detection) suggest(groups []Group, edges []edge) []Suggestion {
	type suggested struct {
		loose int
		group string
	}
	var out []Suggestion
	seen := map[suggested]bool{}
	groupOf := d.firstGroups(groups)
	for _, e := range edges {
		if e.tier != Possible {
			continue
		}
		for _, p := range [2][2]int{{e.a, e.b}, {e.b, e.a}} {
			loose, member := p[0], p[1]
			if groupOf[loose] >= 0 {
				continue
			}
			gi := groupOf[member]
			if gi < 0 || seen[suggested{loose, groups[gi].ID}] {
				continue
			}
			seen[suggested{loose, groups[gi].ID}] = true
			reason := e.reason
			reason.With = d.members[member].ID
			out = append(out, Suggestion{Session: d.members[loose].ID, Group: groups[gi].ID, Reason: reason})
		}
	}
	return out
}

// firstGroups is, by member, the first of groups it is a member of, as
// GroupOf finds it, or -1: read once rather than searched for every
// edge.
func (d *detection) firstGroups(groups []Group) []int {
	of := make([]int, len(d.members))
	for i := range of {
		of[i] = -1
	}
	for gi, g := range groups {
		for _, m := range g.Members {
			if i, ok := d.index[m.Session]; ok && of[i] < 0 {
				of[i] = gi
			}
		}
	}
	return of
}

func inAny(groups []Group, session string) bool {
	_, ok := Result{Groups: groups}.GroupOf(session)
	return ok
}

func groupByID(groups []Group, id string) *Group {
	for i := range groups {
		if groups[i].ID == id {
			return &groups[i]
		}
	}
	return nil
}

func intersect(a, b []string) []string {
	var out []string
	for _, x := range a {
		if contains(b, x) && !contains(out, x) {
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sitsOut reports whether a session is placed by hand or by where it was
// started rather than by detection.
func (d *detection) sitsOut(id string) bool {
	return d.moved[id] || d.started[id] != ""
}
