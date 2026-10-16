package group_test

import (
	"fmt"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
)

var t0 = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

// fillers are twenty sessions days apart in directories of their own,
// sharing only common words, so that they join nothing and make the
// words of the sessions under test rare.
func fillers() []group.Member {
	var ms []group.Member
	for i := 0; i < 20; i++ {
		ms = append(ms, group.Member{
			ID:      fmt.Sprintf("filler-%02d", i),
			Dir:     fmt.Sprintf("/w/filler%d", i),
			Started: t0.Add(-time.Duration(i+1) * 24 * time.Hour),
			Prompts: []string{fmt.Sprintf("fix the test in module%d", i)},
		})
	}
	return ms
}

func detect(memory group.Memory, ms ...group.Member) group.Result {
	return group.Detect(append(fillers(), ms...), memory)
}

func member(id, dir string, after time.Duration, prompts ...string) group.Member {
	return group.Member{ID: id, Dir: dir, Started: t0.Add(after), Prompts: prompts}
}

func TestDetect(t *testing.T) {
	t.Run("when a session joins by a sure signal", func(t *testing.T) {
		withPROJ9 := func(ms ...group.Member) []group.Member {
			a := member("a", "/w/lib", 0, "start PROJ-9 in the library")
			a.Ended = t0.Add(time.Hour)
			c := member("c", "/w/api", 3*time.Hour, "carry on with PROJ-9")
			return append([]group.Member{a, c}, ms...)
		}
		stored := group.Memory{Groups: []group.Stored{{ID: "g1", Members: []string{"a", "c"}}}}
		cases := []struct {
			scenario string
			memory   group.Memory
			members  []group.Member
			signal   group.Signal
		}{
			{
				scenario: "it shares a ticket key in its prompts",
				members: []group.Member{
					member("a", "/w/lib", 0, "start PROJ-123 in the library"),
					member("b", "/w/api", 3*time.Hour, "bump for PROJ-123"),
				},
				signal: group.SignalReference,
			},
			{
				scenario: "it is a teammate in the same agent team",
				members: []group.Member{
					{ID: "a", Dir: "/w/lib", Started: t0, Team: "crew", Prompts: []string{"lead"}},
					{ID: "b", Dir: "/w/api", Started: t0.Add(3 * time.Hour), Team: "crew", Prompts: []string{"follow"}},
				},
				signal: group.SignalParentage,
			},
			{
				scenario: "it was forked from the other",
				members: []group.Member{
					member("a", "/w/lib", 0, "lead"),
					{ID: "b", Dir: "/w/api", Started: t0.Add(3 * time.Hour), Parent: "a", Prompts: []string{"follow"}},
				},
				signal: group.SignalParentage,
			},
			{
				scenario: "the two message each other",
				members: []group.Member{
					{ID: "a", Dir: "/w/lib", Started: t0, Messaged: []string{"b"}, Prompts: []string{"lead"}},
					member("b", "/w/api", 3*time.Hour, "follow"),
				},
				signal: group.SignalParentage,
			},
			{
				scenario: "it was started from the group",
				memory:   stored,
				members: withPROJ9(group.Member{
					ID: "b", Dir: "/w/infra", Started: t0.Add(9 * time.Hour), StartedFrom: "g1", Prompts: []string{"open the gateway"},
				}),
				signal: group.SignalStartedFrom,
			},
		}
		for _, c := range cases {
			t.Run("and "+c.scenario, func(t *testing.T) {
				result := detect(c.memory, c.members...)
				g, _ := result.GroupOf("a")
				joined, _ := result.GroupOf("b")

				t.Run("it should join", func(t *testing.T) {
					assert.Equal(t, g.ID, joined.ID)
				})

				t.Run("it should be sure", func(t *testing.T) {
					assert.Equal(t, group.Sure, joined.Member("b").Tier)
				})

				t.Run("it should say why", func(t *testing.T) {
					assert.Equal(t, c.signal, joined.Member("b").Reason.Signal)
				})
			})
		}
	})

	t.Run("when a session is started long after a member closed", func(t *testing.T) {
		a := member("a", "/w/lib", 0, "start PROJ-9 in the library")
		a.Ended = t0.Add(time.Hour)
		result := detect(group.Memory{}, a, member("c", "/w/api", 3*time.Hour, "carry on with PROJ-9"),
			member("b", "/w/infra", time.Hour+30*time.Minute, "open the gateway"))

		t.Run("it should stay loose", func(t *testing.T) {
			_, ok := result.GroupOf("b")
			assert.False(t, ok)
		})
	})

	t.Run("when a session already in a group is started minutes after another group's member closed", func(t *testing.T) {
		a := member("a", "/w/lib", 0, "start PROJ-9 in the library")
		a.Ended = t0.Add(time.Hour)
		result := detect(group.Memory{}, a, member("c", "/w/api", 3*time.Hour, "carry on with PROJ-9"),
			member("b", "/w/infra", time.Hour+5*time.Minute, "LOG-4 shipping"),
			member("d", "/w/logs", 6*time.Hour, "LOG-4 buffer"))
		var holding []string
		for _, g := range result.Groups {
			if g.Member("b").Session != "" {
				holding = append(holding, g.Anchor)
			}
		}

		t.Run("it should stay in its own group only", func(t *testing.T) {
			assert.Equal(t, []string{"LOG-4"}, holding)
		})
	})

	t.Run("when unrelated sessions are each started minutes after the last one closed", func(t *testing.T) {
		var ms []group.Member
		for i, prompt := range []string{"look around", "open the gateway", "tidy the readme", "why is ci red", "rename the flag"} {
			m := member(fmt.Sprintf("s%d", i), fmt.Sprintf("/w/dir%d", i), time.Duration(i)*20*time.Minute, prompt)
			m.Ended = m.Started.Add(15 * time.Minute)
			ms = append(ms, m)
		}
		result := detect(group.Memory{}, ms...)

		t.Run("it should not chain them into a group", func(t *testing.T) {
			assert.Empty(t, result.Groups)
		})
	})

	t.Run("when a session joins by weaker signals that agree", func(t *testing.T) {
		pasted := func(id, dir string, after time.Duration) group.Member {
			m := member(id, dir, after, "[Pasted text #1]")
			m.Pasted = []string{plan}
			return m
		}
		files := func(id, dir string, after time.Duration, prompt string) group.Member {
			m := member(id, dir, after, prompt)
			m.Files = []string{"/w/lib/limiter.go"}
			return m
		}
		branch := func(id, dir, name string, after time.Duration) group.Member {
			m := member(id, dir, after, "go")
			m.Branch = name
			return m
		}
		cases := []struct {
			scenario string
			members  []group.Member
		}{
			{"they pasted the same text", []group.Member{pasted("a", "/w/lib", 0), pasted("b", "/w/api", 3*time.Hour)}},
			{"they share rare words and were started together", []group.Member{
				member("a", "/w/lib", 0, "add rate-limit headers to the limiter"),
				member("b", "/w/api", 5*time.Minute, "read the rate-limit headers in the middleware"),
			}},
			{"they share rare words and files", []group.Member{
				files("a", "/w/lib", 0, "add rate-limit headers to the limiter"),
				files("b", "/w/api", 3*time.Hour, "read the rate-limit headers in the middleware"),
			}},
			{"their branches share a key and they were started together", []group.Member{
				branch("a", "/w/lib", "proj-77-headers", 0),
				branch("b", "/w/api", "PROJ-77-use-headers", 5*time.Minute),
			}},
			{"they were started within a minute of each other", []group.Member{
				member("a", "/w/lib", 0, "look around"),
				member("b", "/w/api", 40*time.Second, "open the gateway"),
			}},
			{"it was started within a minute of the other closing", []group.Member{
				{ID: "a", Dir: "/w/lib", Started: t0, Ended: t0.Add(time.Hour), Prompts: []string{"look around"}},
				member("b", "/w/api", time.Hour+40*time.Second, "open the gateway"),
			}},
		}
		for _, c := range cases {
			t.Run("and "+c.scenario, func(t *testing.T) {
				result := detect(group.Memory{}, c.members...)
				g, _ := result.GroupOf("a")
				joined, _ := result.GroupOf("b")

				t.Run("it should join", func(t *testing.T) {
					assert.Equal(t, g.ID, joined.ID)
				})

				t.Run("it should be likely", func(t *testing.T) {
					assert.Equal(t, group.Likely, joined.Member("b").Tier)
				})
			})
		}
	})

	t.Run("when a session has one weak signal toward a group", func(t *testing.T) {
		members := func(b group.Member) []group.Member {
			a := member("a", "/w/lib", 0, "add rate-limit headers to the limiter for PROJ-5")
			a.Ended = t0.Add(time.Hour)
			return []group.Member{a, member("c", "/w/api", 3*time.Hour, "PROJ-5 next"), b}
		}
		cases := []struct {
			scenario string
			b        group.Member
			signal   group.Signal
		}{
			{"it shares rare words", member("b", "/w/infra", 9*time.Hour, "allow the rate-limit headers through"), group.SignalWording},
			{"it was started minutes apart", member("b", "/w/infra", 5*time.Minute, "open the gateway"), group.SignalTiming},
			{"it was started minutes after a member closed", member("b", "/w/infra", time.Hour+5*time.Minute, "open the gateway"), group.SignalContinuing},
		}
		for _, c := range cases {
			t.Run("and "+c.scenario, func(t *testing.T) {
				result := detect(group.Memory{}, members(c.b)...)
				g, _ := result.GroupOf("a")

				t.Run("it should not be a member", func(t *testing.T) {
					_, ok := result.GroupOf("b")
					assert.False(t, ok)
				})

				t.Run("it should be suggested the group, saying why", func(t *testing.T) {
					var got []string
					for _, s := range result.SuggestionsFor("b") {
						got = append(got, fmt.Sprint(s.Group, " ", s.Reason.Signal, " with ", s.Reason.With))
					}
					assert.Equal(t, []string{fmt.Sprint(g.ID, " ", c.signal, " with a")}, got)
				})
			})
		}
	})

	t.Run("when two sessions are marked not related", func(t *testing.T) {
		memory := group.Memory{NotRelated: []group.Pair{{"a", "c"}}}

		t.Run("and they share a reference", func(t *testing.T) {
			result := detect(memory,
				member("a", "/w/lib", 0, "PROJ-1"),
				member("c", "/w/api", 3*time.Hour, "PROJ-1"))

			t.Run("it should not join them", func(t *testing.T) {
				_, ok := result.GroupOf("a")
				assert.False(t, ok)
			})
		})

		t.Run("and a third session is related to both", func(t *testing.T) {
			result := detect(memory,
				member("a", "/w/lib", 0, "PROJ-1"),
				member("b", "/w/app", 3*time.Hour, "PROJ-1 and PROJ-2"),
				member("c", "/w/api", 6*time.Hour, "PROJ-2"))
			ga, _ := result.GroupOf("a")
			gc, _ := result.GroupOf("c")

			t.Run("it should still not join them", func(t *testing.T) {
				assert.NotEqual(t, ga.ID, gc.ID)
			})
		})
	})

	t.Run("when sessions chain across directories", func(t *testing.T) {
		result := detect(group.Memory{},
			member("a", "/w/lib", 0, "PROJ-1"),
			member("b", "/w/app", 3*time.Hour, "PROJ-1"),
			group.Member{ID: "c", Dir: "/w/infra", Started: t0.Add(6 * time.Hour), Parent: "b", Prompts: []string{"go"}})
		g, _ := result.GroupOf("c")

		t.Run("it should make one group through every directory, in the order started", func(t *testing.T) {
			assert.Equal(t, []string{"/w/lib", "/w/app", "/w/infra"}, g.Dirs())
		})
	})

	t.Run("when a stored group matches a detected one", func(t *testing.T) {
		result := detect(group.Memory{Groups: []group.Stored{{ID: "g-kept", Members: []string{"a"}}}},
			member("a", "/w/lib", 0, "PROJ-1"),
			member("b", "/w/app", 3*time.Hour, "PROJ-1"))
		g, _ := result.GroupOf("b")

		t.Run("it should keep the stored id", func(t *testing.T) {
			assert.Equal(t, "g-kept", g.ID)
		})
	})

	t.Run("when a session matches nothing", func(t *testing.T) {
		result := detect(group.Memory{}, member("a", "/w/lib", 0, "PROJ-1"))

		t.Run("it should be loose", func(t *testing.T) {
			_, ok := result.GroupOf("a")
			assert.False(t, ok)
		})
	})
}
