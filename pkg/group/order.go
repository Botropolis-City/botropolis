package group

import (
	"fmt"
	"regexp"
	"sort"
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
	version   = regexp.MustCompile(`\bv\d+\.\d+(?:\.\d+)?\b`)
	releasing = regexp.MustCompile(`(?i)\b(?:tag|release|publish|cut)\b`)
	bumping   = regexp.MustCompile(`(?i)\b(?:bump|upgrade|update|to)\b`)
)

// waitsFor is whether text has a waiting word followed, within a few
// words, by what it names.
func waitsFor(text, name string) bool {
	re := regexp.MustCompile(`(?i)\b` + waitWord + `\s+(?:\S+\s+){0,3}?` + regexp.QuoteMeta(name) + `(?:\b|$|[^\w])`)
	return re.MatchString(text)
}

func names(text string, pr SitePR) (string, bool) {
	if pr.URL != "" && strings.Contains(text, pr.URL) {
		return pr.URL, true
	}
	if pr.Repository != "" && pr.Number > 0 {
		ref := fmt.Sprintf("%s#%d", pr.Repository, pr.Number)
		if strings.Contains(text, ref) {
			return ref, true
		}
	}
	return "", false
}

// Order reads the order a group's sites depend on, and puts them in it,
// earliest started first where nothing can be read. A cycle is read as
// no order at all.
func Order(sites []OrderSite) ([]int, []Wait) {
	var waits []Wait
	add := func(w Wait) {
		for _, have := range waits {
			if have.Site == w.Site && have.On == w.On && have.Until == w.Until {
				return
			}
		}
		waits = append(waits, w)
	}
	for b, site := range sites {
		for a, up := range sites {
			if a == b {
				continue
			}
			for _, pr := range up.PRs {
				until := fmt.Sprintf("PR #%d merged", pr.Number)
				w := Wait{Site: b, On: a, Until: until, Done: pr.Merged, Unblocked: up.Name + " " + until}
				for _, line := range append(append([]string{}, site.Words...), site.Commands...) {
					ref, ok := names(line, pr)
					if !ok {
						continue
					}
					if strings.Contains(strings.ToLower(line), "depends-on:") || waitsFor(line, ref) {
						add(w)
					}
				}
			}
			for _, v := range released(up) {
				if !bumps(site, v) {
					continue
				}
				until := v + " tagged"
				add(Wait{Site: b, On: a, Until: until, Done: tagged(up, v), Unblocked: up.Name + " " + until})
			}
			if site.Recap != "" && waitsFor(site.Recap, up.Name) {
				add(Wait{Site: b, On: a, Until: up.Name + " done", Done: up.Done, Unblocked: up.Name + " done"})
			}
		}
	}
	waits = dropCycles(waits, len(sites))
	return order(sites, waits), waits
}

func released(s OrderSite) []string {
	var out []string
	for _, line := range s.Words {
		if releasing.MatchString(line) {
			out = append(out, version.FindAllString(line, -1)...)
		}
	}
	return out
}

func bumps(s OrderSite, v string) bool {
	for _, line := range s.Words {
		if bumping.MatchString(line) && strings.Contains(line, v) && !releasing.MatchString(line) {
			return true
		}
	}
	return false
}

// tagged is whether a site's commands made a version's tag or release:
// only what was run counts, never what was said.
func tagged(s OrderSite, v string) bool {
	q := regexp.QuoteMeta(v)
	re := regexp.MustCompile(`\bgit\s+tag\s+(?:-\S+\s+)*` + q + `\b|\bgit\s+push\b.*\b` + q + `\b|\bgh\s+release\s+create\s+` + q + `\b`)
	for _, c := range s.Commands {
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
