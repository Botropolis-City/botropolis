package group

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// The order as it was read before item 197, kept as the oracle the
// faster reading must agree with.
func oracleOrder(sites []OrderSite) ([]int, []Wait) {
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
					ref, ok := oracleNames(line, pr)
					if !ok {
						continue
					}
					if strings.Contains(strings.ToLower(line), "depends-on:") || oracleWaitsFor(line, ref) {
						add(w)
					}
				}
			}
			for _, v := range oracleReleased(up) {
				if !oracleBumps(site, v) {
					continue
				}
				until := v + " tagged"
				add(Wait{Site: b, On: a, Until: until, Done: oracleTagged(up, v), Unblocked: up.Name + " " + until})
			}
			if site.Recap != "" && oracleWaitsFor(site.Recap, up.Name) {
				add(Wait{Site: b, On: a, Until: up.Name + " done", Done: up.Done, Unblocked: up.Name + " done"})
			}
		}
	}
	waits = dropCycles(waits, len(sites))
	return order(sites, waits), waits
}

const oracleWaitWord = `(?:after|once|until|waits?\s+(?:on|for)|waiting\s+(?:on|for)|blocked\s+(?:on|by)|depends\s+on|needs)`

func oracleWaitsFor(text, name string) bool {
	re := regexp.MustCompile(`(?i)\b` + oracleWaitWord + `\s+(?:\S+\s+){0,3}?` + regexp.QuoteMeta(name) + `(?:\b|$|[^\w])`)
	return re.MatchString(text)
}

func oracleNames(text string, pr SitePR) (string, bool) {
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

var (
	oracleVersion   = regexp.MustCompile(`\bv\d+\.\d+(?:\.\d+)?\b`)
	oracleReleasing = regexp.MustCompile(`(?i)\b(?:tag|release|publish|cut)\b`)
	oracleBumping   = regexp.MustCompile(`(?i)\b(?:bump|upgrade|update|to)\b`)
)

func oracleReleased(s OrderSite) []string {
	var out []string
	for _, line := range s.Words {
		if oracleReleasing.MatchString(line) {
			out = append(out, oracleVersion.FindAllString(line, -1)...)
		}
	}
	return out
}

func oracleBumps(s OrderSite, v string) bool {
	for _, line := range s.Words {
		if oracleBumping.MatchString(line) && strings.Contains(line, v) && !oracleReleasing.MatchString(line) {
			return true
		}
	}
	return false
}

func oracleTagged(s OrderSite, v string) bool {
	q := regexp.QuoteMeta(v)
	re := regexp.MustCompile(`\bgit\s+tag\s+(?:-\S+\s+)*` + q + `\b|\bgit\s+push\b.*\b` + q + `\b|\bgh\s+release\s+create\s+` + q + `\b`)
	for _, c := range s.Commands {
		if re.MatchString(c) {
			return true
		}
	}
	return false
}

// generatedSites is a group read from a seed: sites whose words,
// commands and recaps name each other, their pull requests and their
// versions in every way Order reads, and in ways it must not.
func generatedSites(seed uint64) []OrderSite {
	r := rand.New(rand.NewPCG(seed, 197))
	names := []string{"lib", "api", "web", "Lib", "core.io", "c++", "docs", "api-v2"}
	versions := []string{"v1.2", "v1.2.3", "v2.0.0", "v10.4"}
	pick := func(xs []string) string { return xs[r.IntN(len(xs))] }
	n := 2 + r.IntN(5)
	t0 := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	sites := make([]OrderSite, n)
	for i := range sites {
		name := names[(i+int(seed))%len(names)]
		s := OrderSite{Name: name, Started: t0.Add(time.Duration(r.IntN(4)) * time.Hour), Done: r.IntN(2) == 0}
		for range r.IntN(3) {
			num := 1 + r.IntN(60)
			s.PRs = append(s.PRs, SitePR{Repository: "acme/" + name, Number: num, URL: fmt.Sprintf("https://code.example.com/acme/%s/pull/%d", name, num), Merged: r.IntN(2) == 0})
		}
		sites[i] = s
	}
	other := func() OrderSite { return sites[r.IntN(n)] }
	ref := func() string {
		up := other()
		if len(up.PRs) == 0 {
			return fmt.Sprintf("acme/%s#%d", up.Name, 1+r.IntN(60))
		}
		pr := up.PRs[r.IntN(len(up.PRs))]
		if r.IntN(2) == 0 {
			return pr.URL
		}
		return fmt.Sprintf("%s#%d", pr.Repository, pr.Number)
	}
	words := []func() string{
		func() string { return "waiting on " + ref() + " before merging" },
		func() string { return "this needs the fix in " + ref() },
		func() string { return "Depends-On: " + ref() },
		func() string { return "looked at " + ref() + " for ideas" },
		func() string { return "blocked by the review of " + ref() },
		func() string { return "release " + pick(versions) + " of " + other().Name },
		func() string { return "cut " + pick(versions) + " and publish it" },
		func() string { return "bump " + other().Name + " to " + pick(versions) },
		func() string { return "update the lock file to " + pick(versions) + " after the tag" },
		func() string { return "upgrade to " + pick(versions) },
		func() string { return "nothing to do with " + other().Name },
	}
	commands := []func() string{
		func() string { return "git tag " + pick(versions) },
		func() string { return "git tag -s -m release " + pick(versions) },
		func() string { return "git push github " + pick(versions) },
		func() string { return "gh release create " + pick(versions) + " --notes x" },
		func() string { return "git commit -m 'Depends-On: " + ref() + "'" },
		func() string { return "go test ./..." },
	}
	recaps := []func() string{
		func() string { return "waits on " + other().Name + " to finish" },
		func() string { return "WAITING FOR the " + other().Name },
		func() string { return "once " + other().Name + "." },
		func() string { return "depends on a b c d " + other().Name },
		func() string { return "finished, unlike " + other().Name },
		func() string { return "" },
	}
	for i := range sites {
		for range r.IntN(6) {
			sites[i].Words = append(sites[i].Words, words[r.IntN(len(words))]())
		}
		for range r.IntN(4) {
			sites[i].Commands = append(sites[i].Commands, commands[r.IntN(len(commands))]())
		}
		sites[i].Recap = recaps[r.IntN(len(recaps))]()
	}
	return sites
}

// Item 197: Order reads each site once and compiles its expressions
// once, and must read what it read before.
func TestOrderAgainstTheOracle(t *testing.T) {
	t.Run("when sites are generated from seeds 1 to 400", func(t *testing.T) {
		waited := 0
		for seed := uint64(1); seed <= 400; seed++ {
			sites := generatedSites(seed)
			wantOrder, wantWaits := oracleOrder(sites)
			gotOrder, gotWaits := Order(sites)
			waited += len(wantWaits)
			t.Run(fmt.Sprintf("seed %d, it should read the same order", seed), func(t *testing.T) {
				assert.Equal(t, wantOrder, gotOrder)
			})
			t.Run(fmt.Sprintf("seed %d, it should read the same waits", seed), func(t *testing.T) {
				assert.Equal(t, wantWaits, gotWaits)
			})
		}
		t.Run("the seeds should read waits at all", func(t *testing.T) {
			assert.Greater(t, waited, 400)
		})
	})
}

// Item 197: a group of 25 sites with 40 lines each. It reports and
// never gates.
func BenchmarkOrder(b *testing.B) {
	var sites []OrderSite
	for seed := uint64(1); len(sites) < 25; seed++ {
		for _, s := range generatedSites(seed) {
			for len(s.Words) < 40 {
				s.Words = append(s.Words, s.Words...)
				s.Words = append(s.Words, "release v1.2.3 after the review of acme/lib#7")
			}
			s.Words = s.Words[:40]
			s.Name = fmt.Sprintf("%s-%d", s.Name, len(sites))
			sites = append(sites, s)
		}
	}
	sites = sites[:25]
	b.ReportAllocs()
	for b.Loop() {
		Order(sites)
	}
}
