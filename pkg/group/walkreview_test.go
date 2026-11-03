package group_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/group"
)

// Item 333: the walk's corrections, written when it is left.
func TestWalkReview(t *testing.T) {
	members := []group.Member{
		member("a", "/w/lib", 0, "PROJ-1 start"),
		member("b", "/w/lib", time.Hour, "PROJ-1 tests"),
		member("c", "/w/api", 2*time.Hour, "PROJ-1 bump"),
		member("d", "/w/logs", 6*time.Hour, "LOG-4 buffer"),
		member("e", "/w/config", 9*time.Hour, "LOG-4 schema"),
	}
	reviewed := t0.Add(72 * time.Hour)
	proposal := func() group.Review {
		return group.NewReview(detect(group.Memory{}, members...))
	}
	rowOf := func(r group.Review, session string) int {
		for i, row := range r.Rows {
			for _, m := range row.Members {
				if m.Session == session {
					return i
				}
			}
		}
		return -1
	}
	storedOf := func(m group.Memory, session string) group.Stored {
		for _, s := range m.Groups {
			for _, id := range s.Members {
				if id == session {
					return s
				}
			}
		}
		return group.Stored{}
	}

	t.Run("when a row is renamed and confirmed", func(t *testing.T) {
		r := proposal()
		r.Rename(rowOf(r, "a"), "  The library ")
		memory := group.Memory{}.Confirm(r, reviewed)

		t.Run("it should store the name as named by hand", func(t *testing.T) {
			s := storedOf(memory, "a")
			assert.Equal(t, [2]any{"The library", true}, [2]any{s.Name, s.Renamed})
		})

		t.Run("and detection runs again, it should keep the name", func(t *testing.T) {
			g, _ := detect(memory, members...).GroupOf("a")
			assert.Equal(t, "The library", g.Name)
		})
	})

	t.Run("when a member is taken out of a row and confirmed", func(t *testing.T) {
		r := proposal()
		r.TakeOut(rowOf(r, "c"), "c")
		memory := group.Memory{}.Confirm(r, reviewed)

		t.Run("it should store the row without it", func(t *testing.T) {
			assert.Equal(t, []string{"a", "b"}, storedOf(memory, "a").Members)
		})

		t.Run("it should remember it as not related to the rest", func(t *testing.T) {
			assert.ElementsMatch(t, []group.Pair{{"c", "a"}, {"c", "b"}}, memory.NotRelated)
		})

		t.Run("and detection runs again, it should be in no group", func(t *testing.T) {
			_, in := detect(memory, members...).GroupOf("c")
			assert.False(t, in)
		})
	})

	t.Run("when only some rows are decided and the walk is left for now", func(t *testing.T) {
		r := proposal()
		r.Decide(rowOf(r, "a"))
		memory := group.Memory{}.ConfirmDecided(r, reviewed)

		t.Run("it should store the decided row", func(t *testing.T) {
			assert.NotEmpty(t, storedOf(memory, "a").ID)
		})

		t.Run("it should leave the undecided row unwritten", func(t *testing.T) {
			assert.Empty(t, storedOf(memory, "d").ID)
		})

		t.Run("it should mark the tidy-up reviewed", func(t *testing.T) {
			assert.Equal(t, reviewed, *memory.Reviewed)
		})
	})

	for _, c := range []struct {
		name   string
		change func(r *group.Review, row int)
	}{
		{"dropped", func(r *group.Review, row int) { r.Toggle(row) }},
		{"split", func(r *group.Review, row int) { r.Split(row) }},
		{"renamed", func(r *group.Review, row int) { r.Rename(row, "x") }},
		{"taken out of", func(r *group.Review, row int) { r.TakeOut(row, "c") }},
	} {
		t.Run("when a row is "+c.name, func(t *testing.T) {
			r := proposal()
			c.change(&r, rowOf(r, "c"))

			t.Run("it should count as a decision", func(t *testing.T) {
				assert.True(t, r.AnyDecided())
			})
		})
	}
}
