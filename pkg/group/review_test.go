package group_test

import (
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
)

func TestReview(t *testing.T) {
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
	confirmed := func(change func(*group.Review)) group.Memory {
		r := proposal()
		change(&r)
		return group.Memory{}.Confirm(r, reviewed)
	}
	again := func(m group.Memory) group.Result {
		return detect(m, members...)
	}
	sameGroup := func(result group.Result, a, b string) bool {
		ga, okA := result.GroupOf(a)
		gb, okB := result.GroupOf(b)
		return okA && okB && ga.ID == gb.ID
	}

	t.Run("when the proposal is confirmed with every row kept", func(t *testing.T) {
		memory := confirmed(func(*group.Review) {})

		t.Run("it should store the groups on the screen", func(t *testing.T) {
			assert.Len(t, memory.Groups, 2)
		})

		t.Run("it should mark the first run reviewed", func(t *testing.T) {
			assert.Equal(t, &reviewed, memory.Reviewed)
		})
	})

	t.Run("when a row is dropped", func(t *testing.T) {
		drop := func(r *group.Review) { r.Toggle(rowOf(*r, "d")) }
		memory := confirmed(drop)

		t.Run("it should remember its members as not related", func(t *testing.T) {
			assert.Contains(t, memory.NotRelated, group.Pair{"d", "e"})
		})

		t.Run("and detection runs again, it should not re-form the group", func(t *testing.T) {
			assert.False(t, sameGroup(again(memory), "d", "e"))
		})
	})

	t.Run("when one row is merged into another", func(t *testing.T) {
		merge := func(r *group.Review) { r.Merge(rowOf(*r, "d"), rowOf(*r, "a")) }
		memory := confirmed(merge)

		t.Run("it should move the first row's members into the second", func(t *testing.T) {
			assert.Contains(t, memory.Moves, group.Move{Session: "d", Group: proposal().Rows[rowOf(proposal(), "a")].ID})
		})

		t.Run("and detection runs again, it should keep them together", func(t *testing.T) {
			assert.True(t, sameGroup(again(memory), "a", "e"))
		})
	})

	t.Run("when a row is split by directory", func(t *testing.T) {
		split := func(r *group.Review) { r.Split(rowOf(*r, "a")) }
		r := proposal()
		split(&r)

		t.Run("each directory with two or more sessions should be a group of its own", func(t *testing.T) {
			assert.Equal(t, rowOf(r, "a"), rowOf(r, "b"))
		})

		t.Run("a directory with one session should go loose", func(t *testing.T) {
			assert.Equal(t, -1, rowOf(r, "c"))
		})

		t.Run("it should remember the parts as not related", func(t *testing.T) {
			assert.Contains(t, confirmed(split).NotRelated, group.Pair{"a", "c"})
		})

		t.Run("and detection runs again, it should not re-form the group", func(t *testing.T) {
			assert.False(t, sameGroup(again(confirmed(split)), "a", "c"))
		})
	})
}
