package group_test

import (
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
)

func TestName(t *testing.T) {
	t.Run("when the members share a reference", func(t *testing.T) {
		result := detect(group.Memory{},
			member("a", "/w/lib", 0, "add rate-limit headers to the limiter for PROJ-123"),
			member("b", "/w/api", 3*time.Hour, "PROJ-123: read the rate-limit headers"))
		g, _ := result.GroupOf("a")

		t.Run("it should be named by the reference and the most distinctive words", func(t *testing.T) {
			assert.Equal(t, "PROJ-123 · rate-limit headers", g.Name)
		})

		t.Run("it should be anchored on the reference", func(t *testing.T) {
			assert.Equal(t, "PROJ-123", g.Anchor)
		})
	})

	t.Run("when the members share no reference", func(t *testing.T) {
		result := detect(group.Memory{},
			member("a", "/w/lib", 0, "add rate-limit headers to the limiter"),
			member("b", "/w/api", 5*time.Minute, "read the rate-limit headers"))
		g, _ := result.GroupOf("a")

		t.Run("it should be named by the most distinctive words alone", func(t *testing.T) {
			assert.Equal(t, "rate-limit headers", g.Name)
		})
	})

	t.Run("when the history is small and every member uses the same words", func(t *testing.T) {
		result := group.Detect([]group.Member{
			member("a", "/w/lib", 0, "PROJ-123 add rate-limit headers to the limiter"),
			member("b", "/w/api", 3*time.Hour, "PROJ-123 bump and read the rate-limit headers"),
			member("c", "/w/infra", 6*time.Hour, "PROJ-123 pass the rate-limit headers through"),
			member("d", "/w/site", 9*time.Hour, "fix the footer"),
			member("e", "/w/docs", 12*time.Hour, "tidy the readme"),
			member("f", "/w/blog", 15*time.Hour, "draft a post"),
		}, group.Memory{})
		g, _ := result.GroupOf("a")

		t.Run("it should still be named by the words they share", func(t *testing.T) {
			assert.Equal(t, "PROJ-123 · rate-limit headers", g.Name)
		})
	})

	t.Run("when the members say", func(t *testing.T) {
		cases := []struct {
			scenario string
			members  []group.Member
			want     string
		}{
			{
				scenario: "the group's words in most of a small history, and one member says something else",
				members: []group.Member{
					member("a", "/w/ratelimit-lib", 0, "PROJ-123 expose the rate-limit headers"),
					member("b", "/w/api", 3*time.Hour, "PROJ-123: read the rate-limit headers behind a flag"),
					member("c", "/w/infra", 6*time.Hour, "PROJ-123 Allow the headers through"),
					member("d", "/w/meta", 9*time.Hour, "PROJ-123 rate-limit headers in the library and the API"),
					member("e", "/w/notes", 12*time.Hour, "PROJ-123 notes on the rate-limit headers"),
					member("f", "/w/site", 30*time.Hour, "fix the footer"),
					member("g", "/w/config", 40*time.Hour, "tidy the config"),
				},
				want: "PROJ-123 · rate-limit headers",
			},
			{
				scenario: "a word only one member said beside words more of them said",
				members: append(fillers(),
					member("a", "/w/lib", 0, "PROJ-7 billing ledger xylophone"),
					member("b", "/w/api", 3*time.Hour, "PROJ-7 billing ledger"),
					member("c", "/w/infra", 6*time.Hour, "PROJ-7 billing")),
				want: "PROJ-7 · billing ledger",
			},
			{
				scenario: "the shared words in the opposite order to their rank",
				members: append(fillers(),
					member("a", "/w/lib", 0, "PROJ-5 the ledger feeds billing"),
					member("b", "/w/api", 3*time.Hour, "PROJ-5 ledger for billing")),
				want: "PROJ-5 · ledger billing",
			},
			{
				scenario: "prompts thick with stop words",
				members: append(fillers(),
					member("a", "/w/lib", 0, "PROJ-8 because the cache should go through between requests after deploys"),
					member("b", "/w/api", 3*time.Hour, "PROJ-8 the cache should go through between requests because after")),
				want: "PROJ-8 · cache requests",
			},
		}
		for _, c := range cases {
			t.Run(c.scenario, func(t *testing.T) {
				g, _ := group.Detect(c.members, group.Memory{}).GroupOf("a")

				t.Run("it should be named "+c.want, func(t *testing.T) {
					assert.Equal(t, c.want, g.Name)
				})
			})
		}
	})

	t.Run("when the group was renamed", func(t *testing.T) {
		result := detect(group.Memory{Groups: []group.Stored{{ID: "g1", Name: "Headers", Renamed: true, Members: []string{"a"}}}},
			member("a", "/w/lib", 0, "PROJ-123"),
			member("b", "/w/api", 3*time.Hour, "PROJ-123"))
		g, _ := result.GroupOf("a")

		t.Run("it should keep the new name", func(t *testing.T) {
			assert.Equal(t, "Headers", g.Name)
		})
	})
}

func TestShort(t *testing.T) {
	cases := []struct {
		scenario string
		g        group.Group
		want     string
	}{
		{"the group is anchored", group.Group{Name: "PROJ-123 · rate-limit headers", Anchor: "PROJ-123"}, "PROJ-123"},
		{"the group has no anchor", group.Group{Name: "rate-limit headers"}, "rate-limit"},
		{"the short name is long", group.Group{Name: "internationalisation strings"}, "internationa…"},
	}
	for _, c := range cases {
		t.Run("when "+c.scenario, func(t *testing.T) {
			t.Run("it should be "+c.want, func(t *testing.T) {
				assert.Equal(t, c.want, group.Short(c.g))
			})
		})
	}
}
