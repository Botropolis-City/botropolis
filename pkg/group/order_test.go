package group_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/group"
)

func TestOrder(t *testing.T) {
	t0 := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	libPR := group.SitePR{Repository: "acme/lib", Number: 41, URL: "https://code.example.com/acme/lib/pull/41"}
	merged := libPR
	merged.Merged = true
	// lib is started last, so any order read puts it ahead of where
	// start order alone would.
	lib := func(prs ...group.SitePR) group.OrderSite {
		return group.OrderSite{Name: "lib", Started: t0.Add(2 * time.Hour), PRs: prs}
	}
	api := func(s group.OrderSite) group.OrderSite {
		s.Name, s.Started = "api", t0
		return s
	}
	names := func(sites []group.OrderSite, order []int) []string {
		out := []string{}
		for _, i := range order {
			out = append(out, sites[i].Name)
		}
		return out
	}
	type wait struct{ site, on, until, unblocked string }
	waitsOf := func(sites []group.OrderSite, waits []group.Wait) []wait {
		out := []wait{}
		for _, w := range waits {
			got := wait{site: sites[w.Site].Name, on: sites[w.On].Name, until: w.Until}
			if w.Done {
				got.unblocked = w.Unblocked
			}
			out = append(out, got)
		}
		return out
	}

	for _, c := range []struct {
		when      string
		sites     []group.OrderSite
		order     []string
		waits     []wait
		waitsName string
	}{
		{"no order can be read",
			[]group.OrderSite{lib(libPR), api(group.OrderSite{Words: []string{"tests pass"}})},
			[]string{"api", "lib"}, []wait{}, "neither should wait"},
		{"api's Depends-On names lib's pull request",
			[]group.OrderSite{lib(libPR), api(group.OrderSite{Commands: []string{"Depends-On: https://code.example.com/acme/lib/pull/41"}})},
			[]string{"lib", "api"}, []wait{{"api", "lib", "PR #41 merged", ""}}, "api should wait until lib's pull request merges"},
		{"api's Depends-On names lib's pull request, and it merged",
			[]group.OrderSite{lib(merged), api(group.OrderSite{Commands: []string{"Depends-On: acme/lib#41"}})},
			[]string{"lib", "api"}, []wait{{"api", "lib", "PR #41 merged", "lib PR #41 merged"}}, "api's wait should be done, saying what unblocked it"},
		{"api's words name lib's pull request after once",
			[]group.OrderSite{lib(libPR), api(group.OrderSite{Words: []string{"mark it ready once acme/lib#41 merges"}})},
			[]string{"lib", "api"}, []wait{{"api", "lib", "PR #41 merged", ""}}, "api should wait on lib"},
		{"api's words name lib's pull request without a waiting word",
			[]group.OrderSite{lib(libPR), api(group.OrderSite{Words: []string{"compare with acme/lib#41"}})},
			[]string{"api", "lib"}, []wait{}, "neither should wait"},
		{"api bumps to v1.4.0 and lib's words tag it, but only lib's recap mentions the tag",
			[]group.OrderSite{
				{Name: "lib", Started: t0.Add(2 * time.Hour), Words: []string{"Next: tag v1.4.0 with `" + "git tag v1.4.0` once #41 merges"}},
				api(group.OrderSite{Words: []string{"bump the client to v1.4.0"}})},
			[]string{"lib", "api"}, []wait{{"api", "lib", "v1.4.0 tagged", ""}}, "api should still wait"},
		{"api bumps to v1.4.0 and lib's command pushes the tag",
			[]group.OrderSite{
				{Name: "lib", Started: t0.Add(2 * time.Hour), Words: []string{"release v1.4.0"}, Commands: []string{"git push github v1.4.0"}},
				api(group.OrderSite{Words: []string{"upgrade to v1.4.0"}})},
			[]string{"lib", "api"}, []wait{{"api", "lib", "v1.4.0 tagged", "lib v1.4.0 tagged"}}, "api's wait should be done"},
		{"api's recap says it waits on lib by name",
			[]group.OrderSite{lib(libPR), api(group.OrderSite{Recap: "Ready to ship once lib is released."})},
			[]string{"lib", "api"}, []wait{{"api", "lib", "lib done", ""}}, "api should wait on lib"},
		{"api's recap mentions lib without a waiting word",
			[]group.OrderSite{lib(libPR), api(group.OrderSite{Recap: "Read lib's client to see the headers."})},
			[]string{"api", "lib"}, []wait{}, "it should not order on a bare mention"},
		{"two sites wait on each other",
			[]group.OrderSite{
				{Name: "lib", Started: t0.Add(2 * time.Hour), Recap: "Waits on api."},
				api(group.OrderSite{Recap: "Waits on lib."})},
			[]string{"api", "lib"}, []wait{}, "neither should wait"},
		{"a site names its own pull request",
			[]group.OrderSite{
				{Name: "lib", Started: t0.Add(2 * time.Hour), PRs: []group.SitePR{libPR}, Words: []string{"once acme/lib#41 merges, tag it"}},
				api(group.OrderSite{})},
			[]string{"api", "lib"}, []wait{}, "it should not wait on itself"},
	} {
		t.Run("when "+c.when, func(t *testing.T) {
			order, waits := group.Order(c.sites)

			t.Run("it should order "+c.order[0]+" first", func(t *testing.T) {
				assert.Equal(t, c.order, names(c.sites, order))
			})

			t.Run(c.waitsName, func(t *testing.T) {
				assert.Equal(t, c.waits, waitsOf(c.sites, waits))
			})
		})
	}
}
