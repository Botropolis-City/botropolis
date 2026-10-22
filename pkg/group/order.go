package group

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// OrderSite is one site of a group as its order is read: what it was
// started at, the pull requests seen there, and the lines that can name
// another site — its words, the commands it ran and its recap (item 90).
type OrderSite struct {
	Name     string
	Started  time.Time
	PRs      []SitePR
	Words    []string
	Commands []string
	Recap    string
	// Done is a site with nothing left open, which a wait on it by name
	// is waiting for.
	Done bool
}

type SitePR struct {
	Repository string
	Number     int
	URL        string
	Merged     bool
}

// Wait is a site waiting on another until an event: Until says what,
// Done whether it has been seen, and Unblocked, once it has, what
// unblocked it.
type Wait struct {
	Site, On  int
	Until     string
	Done      bool
	Unblocked string
}

var (
	waitWord  = `(?:after|once|until|waits?\s+(?:on|for)|waiting\s+(?:on|for)|blocked\s+(?:on|by)|depends\s+on|needs)`
	waitHead  = regexp.MustCompile(`(?i)\b` + waitWord + `\s`)
	version   = regexp.MustCompile(`\bv\d+\.\d+(?:\.\d+)?\b`)
	releasing = regexp.MustCompile(`(?i)\b(?:tag|release|publish|cut)\b`)
	bumping   = regexp.MustCompile(`(?i)\b(?:bump|upgrade|update|to)\b`)
)

// readSite is what Order reads from a site once, rather than once for
// every other site in its group (item 197).
type readSite struct {
	// lines are its words and then its commands; dependsOn and waiting
	// say which carry a Depends-On footer and which a waiting word.
	lines     []string
	dependsOn []bool
	waiting   []bool
	// released are the versions its words say were released, and
	// bumpLines the words that move to a version without releasing one.
	released  []string
	bumpLines []string
	// recapWaits is whether its recap has a waiting word at all.
	recapWaits bool
}

func readOf(s OrderSite) readSite {
	r := readSite{recapWaits: s.Recap != "" && waitHead.MatchString(s.Recap)}
	r.lines = append(append(r.lines, s.Words...), s.Commands...)
	for _, line := range r.lines {
		r.dependsOn = append(r.dependsOn, strings.Contains(strings.ToLower(line), "depends-on:"))
		r.waiting = append(r.waiting, waitHead.MatchString(line))
	}
	for _, line := range s.Words {
		if releasing.MatchString(line) {
			r.released = append(r.released, version.FindAllString(line, -1)...)
		} else if bumping.MatchString(line) {
			r.bumpLines = append(r.bumpLines, line)
		}
	}
	return r
}

// bumps is whether a site's words move it to a version.
func (r readSite) bumps(v string) bool {
	for _, line := range r.bumpLines {
		if strings.Contains(line, v) {
			return true
		}
	}
	return false
}

// reader compiles each expression Order needs once per reading.
type reader struct {
	waits  map[string]*regexp.Regexp
	tagged map[taggedKey]bool
}

type taggedKey struct {
	site    int
	version string
}

// waitsFor is whether text has a waiting word followed, within a few
// words, by what it names.
func (r *reader) waitsFor(text, name string) bool {
	re, ok := r.waits[name]
	if !ok {
		re = regexp.MustCompile(`(?i)\b` + waitWord + `\s+(?:\S+\s+){0,3}?` + regexp.QuoteMeta(name) + `(?:\b|$|[^\w])`)
		r.waits[name] = re
	}
	return re.MatchString(text)
}

// names is how text names a pull request whose short reference is
// ref: by its URL, or by that reference.
func names(text string, pr SitePR, ref string) (string, bool) {
	if pr.URL != "" && strings.Contains(text, pr.URL) {
		return pr.URL, true
	}
	if ref != "" && strings.Contains(text, ref) {
		return ref, true
	}
	return "", false
}

func shortRef(pr SitePR) string {
	if pr.Repository == "" || pr.Number <= 0 {
		return ""
	}
	return pr.Repository + "#" + strconv.Itoa(pr.Number)
}

// Order reads the order a group's sites depend on, and puts them in it,
// earliest started first where nothing can be read. A cycle is read as
// no order at all.
func Order(sites []OrderSite) ([]int, []Wait) {
	var waits []Wait
	type waitKey struct {
		site, on int
		until    string
	}
	seen := map[waitKey]bool{}
	add := func(w Wait) {
		k := waitKey{w.Site, w.On, w.Until}
		if seen[k] {
			return
		}
		seen[k] = true
		waits = append(waits, w)
	}
	read := make([]readSite, len(sites))
	for i, s := range sites {
		read[i] = readOf(s)
	}
	r := &reader{waits: map[string]*regexp.Regexp{}, tagged: map[taggedKey]bool{}}
	for b, site := range sites {
		mine := read[b]
		for a, up := range sites {
			if a == b {
				continue
			}
			for _, pr := range up.PRs {
				until := fmt.Sprintf("PR #%d merged", pr.Number)
				w := Wait{Site: b, On: a, Until: until, Done: pr.Merged, Unblocked: up.Name + " " + until}
				short := shortRef(pr)
				for i, line := range mine.lines {
					ref, ok := names(line, pr, short)
					if !ok {
						continue
					}
					if mine.dependsOn[i] || mine.waiting[i] && r.waitsFor(line, ref) {
						add(w)
					}
				}
			}
			for _, v := range read[a].released {
				if !mine.bumps(v) {
					continue
				}
				until := v + " tagged"
				add(Wait{Site: b, On: a, Until: until, Done: r.isTagged(a, up, v), Unblocked: up.Name + " " + until})
			}
			if mine.recapWaits && r.waitsFor(site.Recap, up.Name) {
				add(Wait{Site: b, On: a, Until: up.Name + " done", Done: up.Done, Unblocked: up.Name + " done"})
			}
		}
	}
	waits = dropCycles(waits, len(sites))
	return order(sites, waits), waits
}

func (r *reader) isTagged(i int, s OrderSite, v string) bool {
	k := taggedKey{i, v}
	if t, ok := r.tagged[k]; ok {
		return t
	}
	t := tagged(s, v)
	r.tagged[k] = t
	return t
}

// tagged is whether a site's commands made a version's tag or release:
// only what was run counts, never what was said.
func tagged(s OrderSite, v string) bool {
	var re *regexp.Regexp
	for _, c := range s.Commands {
		if !strings.Contains(c, v) {
			continue
		}
		if re == nil {
			q := regexp.QuoteMeta(v)
			re = regexp.MustCompile(`\bgit\s+tag\s+(?:-\S+\s+)*` + q + `\b|\bgit\s+push\b.*\b` + q + `\b|\bgh\s+release\s+create\s+` + q + `\b`)
		}
		if re.MatchString(c) {
			return true
		}
	}
	return false
}

// dropCycles leaves out every wait between two sites that each reach
// the other.
func dropCycles(waits []Wait, n int) []Wait {
	reach := make([][]bool, n)
	for i := range reach {
		reach[i] = make([]bool, n)
	}
	for _, w := range waits {
		reach[w.Site][w.On] = true
	}
	for k := range n {
		for i := range n {
			for j := range n {
				reach[i][j] = reach[i][j] || (reach[i][k] && reach[k][j])
			}
		}
	}
	out := []Wait{}
	for _, w := range waits {
		if !reach[w.Site][w.On] || !reach[w.On][w.Site] {
			out = append(out, w)
		}
	}
	return out
}

func order(sites []OrderSite, waits []Wait) []int {
	byStart := make([]int, len(sites))
	for i := range byStart {
		byStart[i] = i
	}
	sort.SliceStable(byStart, func(i, j int) bool {
		a, b := sites[byStart[i]], sites[byStart[j]]
		if !a.Started.Equal(b.Started) {
			return a.Started.Before(b.Started)
		}
		return a.Name < b.Name
	})
	placed := make([]bool, len(sites))
	out := make([]int, 0, len(sites))
	for len(out) < len(sites) {
		for _, i := range byStart {
			if placed[i] {
				continue
			}
			ready := true
			for _, w := range waits {
				if w.Site == i && !placed[w.On] {
					ready = false
				}
			}
			if ready {
				placed[i] = true
				out = append(out, i)
				break
			}
		}
	}
	return out
}
