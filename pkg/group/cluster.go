package group

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"time"
)

var defaultBranches = map[string]bool{"": true, "HEAD": true, "main": true, "master": true, "trunk": true, "develop": true}

// signalRank orders the reasons a session joined, strongest first.
var signalRank = map[Signal]int{
	SignalStartedFrom: 0, SignalContinuing: 1, SignalReference: 2, SignalParentage: 3,
	SignalPasted: 4, SignalWording: 5, SignalBranch: 6, SignalFiles: 7, SignalTiming: 8,
}

// Detect groups sessions by what they say and how they were started,
// strongest signal first, keeping the ids and names memory already has
// and never joining a pair marked not related.
func Detect(members []Member, memory Memory) Result {
	members = append([]Member(nil), members...)
	sort.SliceStable(members, func(i, j int) bool { return members[i].Started.Before(members[j].Started) })
	d := newDetection(members, memory)
	edges := d.edges()
	comps := d.cluster(edges)
	groups := d.groups(comps, edges, memory)
	d.joinStarted(groups)
	result := Result{Groups: groups}
	result.Suggestions = d.suggest(groups, edges)
	return result
}

func newDetection(members []Member, memory Memory) *detection {
	d := &detection{members: members, index: map[string]int{}, blocked: map[[2]string]bool{}}
	prompts := map[string][]string{}
	for i, m := range members {
		d.index[m.ID] = i
		prompts[m.ID] = m.Prompts
		seen := map[string]bool{}
		for _, text := range append(append([]string(nil), m.Prompts...), m.Pasted...) {
			for _, r := range References(text) {
				seen[r] = true
			}
		}
		d.refs = append(d.refs, sortedKeys(seen))
	}
	d.wording = NewWording(prompts, memory.Muted)
	for _, p := range memory.NotRelated {
		d.blocked[[2]string{p[0], p[1]}] = true
		d.blocked[[2]string{p[1], p[0]}] = true
	}
	return d
}

func (d *detection) edges() []edge {
	var edges []edge
	for i := range d.members {
		for j := i + 1; j < len(d.members); j++ {
			if d.blocked[[2]string{d.members[i].ID, d.members[j].ID}] {
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
		return signalRank[edges[x].reason.Signal] < signalRank[edges[y].reason.Signal]
	})
	return edges
}

func (d *detection) pair(i, j int) (edge, bool) {
	a, b := d.members[i], d.members[j]
	e := edge{a: i, b: j}
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
	if gap <= together {
		e.tier, e.reason = Likely, Reason{Signal: when}
		return e, true
	}
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
	if (when == SignalTiming && gap <= burst) || (when == SignalContinuing && gap <= continuing) {
		weak = append(weak, Reason{Signal: when})
	}
	switch {
	case len(weak) >= 2:
		e.tier = Likely
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
// refusing any join that would put a not-related pair in one group.
func (d *detection) cluster(edges []edge) [][]int {
	comp := make([]int, len(d.members))
	sets := map[int][]int{}
	for i := range d.members {
		comp[i] = i
		sets[i] = []int{i}
	}
	for _, e := range edges {
		if e.tier < Likely {
			continue
		}
		ca, cb := comp[e.a], comp[e.b]
		if ca == cb || d.anyBlocked(sets[ca], sets[cb]) {
			continue
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

func (d *detection) anyBlocked(xs, ys []int) bool {
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
	for _, comp := range comps {
		in := map[int]bool{}
		for _, i := range comp {
			in[i] = true
		}
		g := Group{}
		for _, i := range comp {
			g.Members = append(g.Members, d.membership(i, in, edges))
		}
		stored, ok := matchStored(comp, d, memory, taken)
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

// membership takes a member's strongest tie inside its group as its tier
// and its reason.
func (d *detection) membership(i int, in map[int]bool, edges []edge) Membership {
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
	return m
}

func matchStored(comp []int, d *detection, memory Memory, taken map[string]bool) (Stored, bool) {
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

func newID(seed string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	return fmt.Sprintf("g-%08x", h.Sum32())
}

// joinStarted adds the loose sessions that were started from a group.
func (d *detection) joinStarted(groups []Group) {
	for _, m := range d.members {
		if m.StartedFrom == "" || inAny(groups, m.ID) {
			continue
		}
		if g := groupByID(groups, m.StartedFrom); g != nil {
			g.Members = append(g.Members, Membership{Session: m.ID, Dir: m.Dir, Started: m.Started, Tier: Sure,
				Reason: Reason{Signal: SignalStartedFrom, Detail: g.Name}})
		}
	}
	for gi := range groups {
		sort.SliceStable(groups[gi].Members, func(x, y int) bool {
			return groups[gi].Members[x].Started.Before(groups[gi].Members[y].Started)
		})
	}
}

func (d *detection) suggest(groups []Group, edges []edge) []Suggestion {
	var out []Suggestion
	seen := map[[2]string]bool{}
	for _, e := range edges {
		if e.tier != Possible {
			continue
		}
		for _, p := range [][2]int{{e.a, e.b}, {e.b, e.a}} {
			loose, member := d.members[p[0]].ID, d.members[p[1]].ID
			if inAny(groups, loose) {
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
