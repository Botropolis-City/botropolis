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
	groups = d.place(groups, memory)
	result := Result{Groups: groups, Capped: d.capped, Common: d.commonNames()}
	result.Suggestions = d.suggest(groups, edges)
	return result
}

// place adds to the detected groups the sessions that sit out detection,
// each where it belongs: the moved where they were put, the started
// where they were started from, and teammates with their lead; then
// drops the groups closed by hand.
func (d *detection) place(groups []Group, memory Memory) []Group {
	groups = d.placeMoved(groups, memory)
	groups = d.joinStarted(groups, memory)
	groups = d.followLeads(groups, memory)
	sortMembers(groups)
	return withoutClosed(groups, memory)
}

func sortMembers(groups []Group) {
	for gi := range groups {
		sort.SliceStable(groups[gi].Members, func(x, y int) bool {
			return groups[gi].Members[x].Started.Before(groups[gi].Members[y].Started)
		})
	}
}

// withoutClosed is groups but those closed by hand, whose members are
// left in no group (item 333).
func withoutClosed(groups []Group, memory Memory) []Group {
	if len(memory.Closed) == 0 {
		return groups
	}
	return slices.DeleteFunc(groups, func(g Group) bool { return slices.Contains(memory.Closed, g.ID) })
}

func newDetection(members []Member, memory Memory, prior *Detector) *detection {
	d := &detection{members: members, index: make(map[string]int, len(members)), blocked: map[[2]string]bool{},
		moved: map[string]bool{}, confirmed: map[string]bool{}, refs: make([][]string, 0, len(members)),
		follows: map[string]string{}}
	for _, m := range members {
		if m.Lead != "" && m.Lead != m.ID {
			d.follows[m.ID] = m.Lead
		}
	}
	d.moves = teamMoves(memory.Moves, d.follows)
	for _, mv := range d.moves {
		d.moved[mv.Session] = true
	}
	d.started = map[string]string{}
	for _, m := range members {
		from := m.StartedFrom
		if from == "" && m.Job != "" {
			from = memory.StartedFrom[m.Job]
		}
		if from != "" && !d.moved[m.ID] && d.follows[m.ID] == "" {
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
	d.common = d.commonKeys()
	for _, p := range memory.NotRelated {
		a, b := d.leadOf(p[0]), d.leadOf(p[1])
		if a == b {
			continue
		}
		d.blocked[[2]string{a, b}] = true
		d.blocked[[2]string{b, a}] = true
	}
	return d
}

// leadOf is the session that stands for id in detection: its lead when
// it is a teammate, else itself. A pair marked not related inside a team
// counts for nothing, and one between a teammate and an outsider counts
// for its lead, since a teammate's take-out acts on the team (item 343).
func (d *detection) leadOf(id string) string {
	if lead, ok := d.follows[id]; ok {
		return lead
	}
	return id
}

// teamMoves are the moves as they count: a teammate's is its team's, so
// it is read as its lead's, and of the moves one session carries the
// latest recorded wins, as WithMove already keeps it. This is what keeps
// a move written by hand on each teammate working (item 343).
func teamMoves(moves []Move, follows map[string]string) []Move {
	out := make([]Move, 0, len(moves))
	for _, mv := range moves {
		if lead, ok := follows[mv.Session]; ok {
			mv.Session = lead
		}
		out = slices.DeleteFunc(out, func(o Move) bool { return o.Session == mv.Session })
		out = append(out, mv)
	}
	return out
}

// moveOf is the move that counts for session, if any.
func (d *detection) moveOf(session string) (Move, bool) {
	for _, mv := range d.moves {
		if mv.Session == session {
			return mv, true
		}
	}
	return Move{}, false
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
	if shared := d.uncommon(refKey, intersect(d.refs[i], d.refs[j])); len(shared) > 0 {
		e.tier, e.reason = Sure, Reason{Signal: SignalReference, Detail: shared[0]}
		return e, true
	}
	if d.related(a, b) {
		e.tier, e.reason = Sure, Reason{Signal: SignalParentage}
		return e, true
	}
	if samePaste(a, b) {
		e.tier, e.reason = Likely, Reason{Signal: SignalPasted}
		return e, true
	}
	when, gap := timing(a, b)
	var weak []Reason
	d.weighed++
	if d.wording.Related(a.ID, b.ID) {
		words, _ := d.wording.Shared(a.ID, b.ID)
		weak = append(weak, Reason{Signal: SignalWording, Detail: strings.Join(words, " ")})
	}
	if key, ok := d.sameBranch(a.Branch, b.Branch); ok {
		weak = append(weak, Reason{Signal: SignalBranch, Detail: key})
	}
	if files := d.uncommon(fileKey, intersect(a.Files, b.Files)); len(files) > 0 {
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

func (d *detection) related(a, b Member) bool {
	if a.Team != "" && a.Team == b.Team && !d.isCommon(teamKey, a.Team) {
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

// sameBranch is what two branches share, by the keys branchKeys gives
// them, leaving out any too common to be evidence.
func (d *detection) sameBranch(a, b string) (string, bool) {
	if defaultBranches[a] || defaultBranches[b] {
		return "", false
	}
	if a == b && !d.isCommon(branchKey, "branch:"+a) {
		return a, true
	}
	for _, shared := range intersect(References(strings.ToUpper(a)), References(strings.ToUpper(b))) {
		if !d.isCommon(branchKey, "ref:"+shared) {
			return shared, true
		}
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
	for _, mv := range d.moves {
		i, ok := d.index[mv.Session]
		if !ok || mv.Group == "" {
			continue
		}
		var g *Group
		if groups, g = groupFrom(groups, memory, mv.Group); g == nil {
			continue
		}
		g.Members = append(g.Members, d.placed(i, Reason{Signal: SignalMoved}))
	}
	return groups
}

// placed is member i's membership as a session placed rather than
// detected: sure, for the reason given.
func (d *detection) placed(i int, reason Reason) Membership {
	m := d.members[i]
	return Membership{Session: m.ID, Dir: m.Dir, Started: m.Started, Tier: Sure, Reason: reason}
}

// groupFrom is the group with id among groups, made from memory when
// detection did not find it, or nil when memory does not hold it either.
func groupFrom(groups []Group, memory Memory, id string) ([]Group, *Group) {
	if g := groupByID(groups, id); g != nil {
		return groups, g
	}
	for _, s := range memory.Groups {
		if s.ID == id {
			groups = append(groups, Group{ID: s.ID, Name: s.Name, Renamed: s.Renamed, Anchor: s.Anchor})
			return groups, &groups[len(groups)-1]
		}
	}
	return groups, nil
}

// followLeads puts every teammate in its lead's group, whatever
// detection, the moves and the pairs marked not related did with it: the
// group the lead is in; when the lead is in none and was not moved to
// none, a group of the team, under the stored group holding the lead
// when one is free; when the lead is not among the members, the group
// the lead's move or a stored group records it in; else none. So a
// team is always in one group or in none together (item 343).
func (d *detection) followLeads(groups []Group, memory Memory) []Group {
	if len(d.follows) == 0 {
		return groups
	}
	var leads []string
	teams := map[string][]int{}
	for i, m := range d.members {
		if lead, ok := d.follows[m.ID]; ok {
			if teams[lead] == nil {
				leads = append(leads, lead)
			}
			teams[lead] = append(teams[lead], i)
		}
	}
	for _, lead := range leads {
		var g *Group
		groups, g = d.teamGroup(groups, memory, lead)
		if g == nil {
			continue
		}
		team := d.members[teams[lead][0]].Team
		fresh := len(g.Members) == 0
		for _, i := range teams[lead] {
			g.Members = append(g.Members, d.placed(i, Reason{Signal: SignalTeam, Detail: team, With: lead}))
		}
		if li, ok := d.index[lead]; ok && g.Member(lead).Session == "" {
			g.Members = append(g.Members, d.placed(li, Reason{Signal: SignalTeam, Detail: team}))
		}
		if fresh {
			d.nameTeam(g)
		}
	}
	return groups
}

// teamGroup is the group a lead's team goes in, made when it has to be,
// or nil for none.
func (d *detection) teamGroup(groups []Group, memory Memory, lead string) ([]Group, *Group) {
	if _, ok := d.index[lead]; ok {
		if g, found := (Result{Groups: groups}).GroupOf(lead); found {
			return groups, groupByID(groups, g.ID)
		}
		if mv, ok := d.moveOf(lead); ok && mv.Group == "" {
			return groups, nil
		}
		if id, ok := d.storedHolding(memory, lead, groups); ok {
			return groupFrom(groups, memory, id)
		}
		groups = append(groups, Group{ID: newID(lead)})
		return groups, &groups[len(groups)-1]
	}
	if mv, ok := d.moveOf(lead); ok {
		if mv.Group == "" {
			return groups, nil
		}
		return groupFrom(groups, memory, mv.Group)
	}
	if id, ok := d.storedHolding(memory, lead, nil); ok {
		return groupFrom(groups, memory, id)
	}
	return groups, nil
}

// storedHolding is the first stored group holding lead that is not among
// groups already and that lead was not taken out of — one holding no
// member lead is marked not related to — if any. Without the last, a team
// taken out of a group whose other members then stand alone would form
// again under that group's id.
func (d *detection) storedHolding(memory Memory, lead string, groups []Group) (string, bool) {
	for _, s := range memory.Groups {
		if !slices.Contains(s.Members, lead) || groupByID(groups, s.ID) != nil {
			continue
		}
		if slices.ContainsFunc(s.Members, func(m string) bool { return d.blocked[[2]string{lead, d.leadOf(m)}] }) {
			continue
		}
		return s.ID, true
	}
	return "", false
}

// nameTeam names a group made of a team, as groups names a detected one,
// keeping a name given by hand.
func (d *detection) nameTeam(g *Group) {
	name, renamed := g.Name, g.Renamed
	d.names.name(d, g)
	if renamed {
		g.Name, g.Renamed = name, true
	}
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
	for i, m := range d.members {
		from := d.started[m.ID]
		if from == "" || inAny(groups, m.ID) {
			continue
		}
		var g *Group
		if groups, g = groupFrom(groups, memory, from); g != nil {
			g.Members = append(g.Members, d.placed(i, Reason{Signal: SignalStartedFrom, Detail: g.Name}))
		}
	}
	return groups
}

func (d *detection) suggest(groups []Group, edges []edge) []Suggestion {
	type suggested struct {
		loose int
		group string
	}
	var out []Suggestion
	seen := map[suggested]int{}
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
			if gi < 0 {
				continue
			}
			reason := e.reason
			reason.With = d.members[member].ID
			out, seen = keepSuggestion(out, seen, suggested{loose, groups[gi].ID},
				Suggestion{Session: d.members[loose].ID, Group: groups[gi].ID, Reason: reason})
		}
	}
	return out
}

// keepSuggestion adds s, or puts it in place of the suggestion already
// made for its session and group when its reason comes first: the
// strongest signal, then the member it matched, so the suggestion does
// not depend on the order equal ties were found in (item 333).
func keepSuggestion[K comparable](out []Suggestion, seen map[K]int, key K, s Suggestion) ([]Suggestion, map[K]int) {
	at, ok := seen[key]
	if !ok {
		seen[key] = len(out)
		return append(out, s), seen
	}
	if before(s.Reason, out[at].Reason) {
		out[at] = s
	}
	return out, seen
}

func before(a, b Reason) bool {
	if ra, rb := signalRank[a.Signal], signalRank[b.Signal]; ra != rb {
		return ra < rb
	}
	if a.With != b.With {
		return a.With < b.With
	}
	return a.Detail < b.Detail
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

// sitsOut reports whether a session is placed by hand, by where it was
// started or by its lead rather than by detection.
func (d *detection) sitsOut(id string) bool {
	return d.moved[id] || d.started[id] != "" || d.follows[id] != ""
}
