package acceptance_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/measure"
	"botropolis.city/botropolis/pkg/render"
	"botropolis.city/botropolis/testing/helpers"
)

// What each piece of work was doing and what is next, on one screen,
// after opening the map: no steps at all. The journal opens on the first
// frame after a gap, and reads one conversation for each entry it draws.
// Item 91.

// lastSeen is the Friday evening the map was last looked at.
var lastSeen = time.Date(2026, 10, 9, 18, 0, 0, 0, time.Local)

type opened struct {
	journal []city.JournalEntry
	steps   int
	ticks   int
	work    measure.Work
}

// openAfterTheWeekend opens the map on Monday, last seen on Friday.
func openAfterTheWeekend(t testing.TB, home string) opened {
	t.Helper()
	before := measure.Take()
	d, err := render.NewDrive(render.DriveOptions{Home: home, Now: monday, Seen: lastSeen})
	require.NoError(t, err)
	return opened{journal: d.Shown().Journal, steps: d.Steps(), ticks: d.Ticks(), work: measure.Take().Since(before)}
}

func TestWhatEachPieceOfWorkWasDoing(t *testing.T) {
	t.Run("when the map opens after the weekend", func(t *testing.T) {
		home, grouped := helpers.JournalWeek(t, monday)
		o := openAfterTheWeekend(t, home)
		require.NotEmpty(t, o.journal, "no journal was drawn, so every check below passes on nothing")
		in := map[string]int{}
		for _, e := range o.journal {
			for _, id := range e.Sessions {
				in[id]++
			}
		}

		t.Run("it should take no steps", func(t *testing.T) {
			assert.Zero(t, o.steps)
		})

		t.Run("it should draw the journal on the first frame", func(t *testing.T) {
			assert.Equal(t, 1, o.ticks)
		})

		t.Run("it should give the group one entry and each session in none its own", func(t *testing.T) {
			sizes := map[int]int{}
			for _, e := range o.journal {
				sizes[len(e.Sessions)]++
			}
			assert.Equal(t, map[int]int{len(grouped): 1, 1: 4}, sizes)
		})

		t.Run("it should put the grouped sessions in the same entry", func(t *testing.T) {
			var together bool
			for _, e := range o.journal {
				together = together || (len(e.Sessions) == len(grouped) && subset(grouped, e.Sessions))
			}
			assert.True(t, together)
		})

		t.Run("it should put every session it draws in exactly one entry", func(t *testing.T) {
			for id, n := range in {
				assert.Equal(t, 1, n, id)
			}
		})

		t.Run("it should say what is next in every entry drawn", func(t *testing.T) {
			for _, e := range o.journal {
				assert.NotContains(t, []string{"", "not recorded"}, e.Next, e.Title)
			}
		})

		t.Run("it should read one conversation per entry drawn", func(t *testing.T) {
			assert.Equal(t, int64(len(o.journal)), o.work[measure.ConversationsRead])
		})
	})

	t.Run("when it opens on 1,000 generated sessions", func(t *testing.T) {
		o := openAfterTheWeekend(t, helpers.ParkedWeek(t, 1000, monday, reopenTitle))
		small := openAfterTheWeekend(t, helpers.ParkedWeek(t, 100, monday, reopenTitle))
		require.NotEmpty(t, o.journal, "no journal was drawn, so every check below passes on nothing")
		require.Positive(t, small.work[measure.ConversationsRead], "nothing read on 100: the bound below holds on nothing")

		t.Run("it should read one conversation per entry drawn", func(t *testing.T) {
			assert.Equal(t, int64(len(o.journal)), o.work[measure.ConversationsRead])
		})

		t.Run("it should read at most ten times the conversations of 100 sessions", func(t *testing.T) {
			assert.LessOrEqual(t, o.work[measure.ConversationsRead], 10*small.work[measure.ConversationsRead])
		})
	})
}

func subset(want, of []string) bool {
	for _, w := range want {
		found := false
		for _, o := range of {
			found = found || o == w
		}
		if !found {
			return false
		}
	}
	return true
}
