package group_test

import (
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
)

func TestGuess(t *testing.T) {
	// Two groups an hour or more before "now": PROJ-1 across lib and api,
	// LOG-4 across logs and config, with the api session last active at
	// the hour.
	now := t0.Add(10 * time.Hour)
	history := func(apiEnded time.Duration) []group.Member {
		api := member("b", "/w/api", 3*time.Hour, "PROJ-1 read the rate-limit headers")
		api.Ended = now.Add(-apiEnded)
		return append(fillers(),
			member("a", "/w/lib", 0, "PROJ-1 add rate-limit headers"),
			api,
			member("c", "/w/logs", 4*time.Hour, "LOG-4 cap the buffer"),
			member("d", "/w/config", 5*time.Hour, "LOG-4 schema"),
		)
	}
	far := 6*time.Hour + 55*time.Minute
	guess := func(members []group.Member, dir, prompt string) group.Guess {
		return group.NewGuesser(members, group.Memory{}).Guess(group.Member{ID: "new", Dir: dir, Started: now, Prompts: []string{prompt}})
	}
	idOf := func(members []group.Member, session string) string {
		g, _ := group.Detect(members, group.Memory{}).GroupOf(session)
		return g.ID
	}

	cases := []struct {
		scenario string
		members  []group.Member
		dir      string
		prompt   string
		group    string
		tier     group.Tier
		reason   string
	}{
		{"the prompt mentions a group's reference", history(far), "/w/infra", "pass the headers for LOG-4", "c", group.Sure, "mentions LOG-4"},
		{"you were in one of its sessions a minute ago", history(30 * time.Second), "/w/infra", "go", "a", group.Likely, "continued from b"},
		{"you were in one of its sessions minutes ago", history(5 * time.Minute), "/w/infra", "go", "a", group.Possible, "continued from b"},
		{"the prompt shares its rare words", history(far), "/w/infra", "allow the rate-limit headers through", "a", group.Possible, `shares "headers rate-limit" with a`},
	}
	for _, c := range cases {
		t.Run("when "+c.scenario, func(t *testing.T) {
			g := guess(c.members, c.dir, c.prompt)

			t.Run("it should guess that group", func(t *testing.T) {
				assert.Equal(t, idOf(c.members, c.group), g.Group)
			})

			t.Run("it should guess with slice 1's tier for the signal", func(t *testing.T) {
				assert.Equal(t, c.tier, g.Tier)
			})

			t.Run("it should say why", func(t *testing.T) {
				assert.Equal(t, c.reason, g.Reason.Describe(func(id string) string { return id }))
			})
		})
	}

	t.Run("when nothing matches", func(t *testing.T) {
		g := guess(history(far), "/w/infra", "tidy the readme")

		t.Run("it should guess no group", func(t *testing.T) {
			assert.Equal(t, "", g.Group)
		})
	})

	t.Run("when a guess is made", func(t *testing.T) {
		members := history(far)
		g := guess(members, "/w/infra", "pass the headers for LOG-4")

		t.Run("it should offer the other groups as alternatives, the guess first", func(t *testing.T) {
			assert.Equal(t, []string{idOf(members, "c"), idOf(members, "a")}, g.Alternatives)
		})
	})

	t.Run("when memory says a session's job was started from a group", func(t *testing.T) {
		members := history(far)
		started := member("e", "/w/infra", 9*time.Hour, "go")
		started.Job = "0898d7e4"
		g1 := idOf(members, "a")
		result := group.Detect(append(members, started),
			group.Memory{StartedFrom: map[string]string{"0898d7e4": g1}})
		joined, _ := result.GroupOf("e")

		t.Run("it should join that group", func(t *testing.T) {
			assert.Equal(t, g1, joined.ID)
		})

		t.Run("it should be sure", func(t *testing.T) {
			assert.Equal(t, group.Sure, joined.Member("e").Tier)
		})
	})

	t.Run("when memory says a job was started in a new group", func(t *testing.T) {
		started := member("e", "/w/infra", 9*time.Hour, "go")
		started.Job = "0898d7e4"
		result := group.Detect(append(fillers(), started), group.Memory{
			Groups:      []group.Stored{{ID: "g-new", Name: "Gateway", Renamed: true}},
			StartedFrom: map[string]string{"0898d7e4": "g-new"},
		})
		joined, _ := result.GroupOf("e")

		t.Run("it should make that group from memory", func(t *testing.T) {
			assert.Equal(t, "Gateway", joined.Name)
		})
	})
}
