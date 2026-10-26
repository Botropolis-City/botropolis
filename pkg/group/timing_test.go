package group_test

import (
	"fmt"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
)

func largestGroup(result group.Result) int {
	largest := 0
	for _, g := range result.Groups {
		largest = max(largest, len(g.Members))
	}
	return largest
}

func TestDetectTiming(t *testing.T) {
	t.Run("when sessions are started a minute apart in a steady stream", func(t *testing.T) {
		t.Run("and 500 of them share nothing else", func(t *testing.T) {
			var ms []group.Member
			for i := 0; i < 500; i++ {
				m := member(fmt.Sprintf("run-%03d", i), fmt.Sprintf("/w/run%03d", i), time.Duration(i)*time.Minute, "look around")
				m.Ended = m.Started.Add(30 * time.Second)
				ms = append(ms, m)
			}
			result := detect(group.Memory{}, ms...)

			t.Run("it should make no group", func(t *testing.T) {
				assert.Empty(t, result.Groups)
			})
		})
	})

	t.Run("when two sessions were started within a minute of each other", func(t *testing.T) {
		cases := []struct {
			scenario string
			a, b     group.Member
		}{
			{"and they share a reference",
				member("a", "/w/lib", 0, "start PROJ-31"), member("b", "/w/api", 40*time.Second, "PROJ-31 too")},
			{"and they are in one team",
				group.Member{ID: "a", Dir: "/w/lib", Started: t0, Team: "crew", Prompts: []string{"lead"}},
				group.Member{ID: "b", Dir: "/w/api", Started: t0.Add(40 * time.Second), Team: "crew", Prompts: []string{"follow"}}},
			{"and one is the other's parent",
				member("a", "/w/lib", 0, "lead"),
				group.Member{ID: "b", Dir: "/w/api", Started: t0.Add(40 * time.Second), Parent: "a", Prompts: []string{"follow"}}},
			{"and one messaged the other",
				group.Member{ID: "a", Dir: "/w/lib", Started: t0, Messaged: []string{"b"}, Prompts: []string{"lead"}},
				member("b", "/w/api", 40*time.Second, "follow")},
		}
		for _, c := range cases {
			t.Run(c.scenario, func(t *testing.T) {
				result := detect(group.Memory{}, c.a, c.b)
				ga, _ := result.GroupOf("a")
				gb, ok := result.GroupOf("b")

				t.Run("it should still group them", func(t *testing.T) {
					assert.True(t, ok && ga.ID == gb.ID)
				})
			})
		}
	})

	t.Run("when each session in a stream shares a file with the next and starts minutes after it", func(t *testing.T) {
		var ms []group.Member
		for i := 0; i < 12; i++ {
			m := member(fmt.Sprintf("s%02d", i), fmt.Sprintf("/w/dir%02d", i), time.Duration(i)*5*time.Minute, "go")
			m.Files = []string{fmt.Sprintf("/w/f%02d.go", i), fmt.Sprintf("/w/f%02d.go", i+1)}
			ms = append(ms, m)
		}
		result := detect(group.Memory{}, ms...)

		t.Run("it should not chain them into a group of more than a pair", func(t *testing.T) {
			assert.LessOrEqual(t, largestGroup(result), 2)
		})
	})

	t.Run("when a group is tied by a reference", func(t *testing.T) {
		withFile := func(id, dir string, after time.Duration, files ...string) group.Member {
			m := member(id, dir, after, "go")
			m.Files = files
			return m
		}
		a := member("a", "/w/lib", 0, "start PROJ-5")
		a.Files = []string{"/w/lib/limiter.go"}
		result := detect(group.Memory{},
			a,
			member("c", "/w/api", 3*time.Hour, "PROJ-5 next"),
			withFile("b", "/w/infra", 5*time.Minute, "/w/lib/limiter.go", "/w/infra/gate.go"),
			withFile("d", "/w/docs", 10*time.Minute, "/w/infra/gate.go"))
		g, _ := result.GroupOf("a")

		t.Run("and a session shares a file with a member and started minutes after it", func(t *testing.T) {
			t.Run("it should join", func(t *testing.T) {
				joined, _ := result.GroupOf("b")
				assert.Equal(t, g.ID, joined.ID)
			})
		})

		t.Run("and a session is tied the same way only to a member that joined on timing", func(t *testing.T) {
			t.Run("it should not join", func(t *testing.T) {
				_, ok := result.GroupOf("d")
				assert.False(t, ok)
			})
		})
	})
}
