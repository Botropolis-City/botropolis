package group_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/group"
)

// Item 333: a group renamed or a member taken out, by hand.
func TestCorrectMembership(t *testing.T) {
	trio := func() []group.Member {
		return []group.Member{
			member("a", "/w/lib", 0, "start PROJ-9 in the library"),
			member("b", "/w/api", time.Hour, "carry on with PROJ-9"),
			member("c", "/w/site", 2*time.Hour, "PROJ-9 on the site"),
		}
	}
	idOf := func(r group.Result, session string) string {
		for _, g := range r.Groups {
			if g.Member(session).Session != "" {
				return g.ID
			}
		}
		return ""
	}
	members := func(r group.Result, id string) []string {
		var ids []string
		for _, g := range r.Groups {
			if g.ID == id {
				for _, m := range g.Members {
					ids = append(ids, m.Session)
				}
			}
		}
		return ids
	}
	nameOf := func(r group.Result, id string) string {
		for _, g := range r.Groups {
			if g.ID == id {
				return g.Name
			}
		}
		return ""
	}

	t.Run("when a stored group is renamed", func(t *testing.T) {
		first := detect(group.Memory{}, trio()...)
		id := idOf(first, "a")
		memory := group.Memory{}.Remember(first).WithRename(id, "  The limiter ")
		var stored group.Stored
		for _, s := range memory.Groups {
			if s.ID == id {
				stored = s
			}
		}

		t.Run("it should store the trimmed name as named by hand", func(t *testing.T) {
			assert.Equal(t, [2]any{"The limiter", true}, [2]any{stored.Name, stored.Renamed})
		})

		t.Run("and detection runs again on the same history", func(t *testing.T) {
			again := detect(memory, trio()...)

			t.Run("it should keep the new name", func(t *testing.T) {
				assert.Equal(t, "The limiter", nameOf(again, id))
			})
		})
	})

	t.Run("when a group is closed", func(t *testing.T) {
		first := detect(group.Memory{}, trio()...)
		id := idOf(first, "a")
		memory := group.Memory{}.Remember(first).WithClosed(id)

		t.Run("it should store the group as closed", func(t *testing.T) {
			assert.Equal(t, []string{id}, memory.Closed)
		})

		t.Run("and memory is remembered again", func(t *testing.T) {
			t.Run("it should still hold it closed", func(t *testing.T) {
				assert.Equal(t, []string{id}, memory.Remember(first).Closed)
			})
		})

		t.Run("and detection runs again", func(t *testing.T) {
			loose := member("x", "/w/docs", 90*time.Minute, "unrelated work on the docs")
			again := detect(memory, append(trio(), loose)...)

			t.Run("it should not form the group", func(t *testing.T) {
				assert.Empty(t, members(again, id))
			})

			t.Run("it should leave its members in no group", func(t *testing.T) {
				assert.Empty(t, idOf(again, "a"))
			})

			t.Run("it should suggest nothing into it", func(t *testing.T) {
				for _, s := range again.Suggestions {
					assert.NotEqual(t, id, s.Group)
				}
			})
		})

		t.Run("and it is reopened", func(t *testing.T) {
			reopened := memory.WithReopened(id)

			t.Run("it should no longer be closed", func(t *testing.T) {
				assert.Empty(t, reopened.Closed)
			})

			t.Run("and detection runs again, it should form the group again", func(t *testing.T) {
				assert.Equal(t, []string{"a", "b", "c"}, members(detect(reopened, trio()...), id))
			})
		})
	})

	t.Run("when a session is taken out of a group", func(t *testing.T) {
		first := detect(group.Memory{}, trio()...)
		id := idOf(first, "a")
		memory := group.Memory{}.Remember(first).WithMove("c", id).WithTakenOut("c", id)

		t.Run("it should remember it as not related to every other member", func(t *testing.T) {
			assert.ElementsMatch(t, []group.Pair{{"c", "a"}, {"c", "b"}}, memory.NotRelated)
		})

		t.Run("and it had been moved in by hand, it should drop that move", func(t *testing.T) {
			assert.Empty(t, memory.Moves)
		})

		t.Run("and detection runs again", func(t *testing.T) {
			again := detect(memory, trio()...)

			t.Run("it should no longer be a member", func(t *testing.T) {
				assert.Equal(t, []string{"a", "b"}, members(again, id))
			})
		})
	})
}
