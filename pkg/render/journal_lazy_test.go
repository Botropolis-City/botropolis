package render

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/measure"
	"botropolis.city/botropolis/testing/helpers"
)

// The journal reads what an entry was doing from its transcript when the
// entry is first drawn or selected, keeps it only while the journal is
// open, and reads again when it is opened again. Items 86 and 91.
func TestJournalReadsWhatItShows(t *testing.T) {
	now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.Local)
	seen := now.Add(-64 * time.Hour)
	home := helpers.ParkedWeek(t, 40, now, "Battery check compares as text")
	reads := func() int64 { return measure.Read(measure.ConversationsRead) }
	open := func(t *testing.T) (*Drive, int64) {
		t.Helper()
		before := reads()
		d, err := NewDrive(DriveOptions{Home: home, Now: now, Seen: seen})
		require.NoError(t, err)
		j, ok := d.game.overlays.top().(*journal)
		require.True(t, ok, "the journal did not open")
		require.Less(t, j.drawn, len(j.entries), "every entry fits, so nothing is left undrawn to test")
		return d, reads() - before
	}

	t.Run("when the journal opens over more entries than it draws", func(t *testing.T) {
		d, opened := open(t)
		j := d.game.overlays.top().(*journal)

		t.Run("it should read only the entries it draws", func(t *testing.T) {
			assert.Equal(t, int64(j.drawn), opened)
		})

		t.Run("and the selection moves onto an undrawn entry", func(t *testing.T) {
			for range j.drawn - 1 {
				require.NoError(t, d.Press("ArrowDown"))
			}
			require.NoError(t, d.Run(1))
			before := reads()
			require.NoError(t, d.Press("ArrowDown"))
			require.NoError(t, d.Run(1))
			first := reads() - before
			require.NoError(t, d.Press("ArrowUp"))
			require.NoError(t, d.Press("ArrowDown"))
			require.NoError(t, d.Run(1))
			again := reads() - before - first

			t.Run("it should read it then, once", func(t *testing.T) {
				assert.Equal(t, [2]int64{1, 0}, [2]int64{first, again})
			})
		})
	})

	t.Run("when the journal closes and opens again", func(t *testing.T) {
		d, _ := open(t)
		require.NoError(t, d.Press("Escape"))
		require.NoError(t, d.Run(1))
		_, still := d.game.overlays.top().(*journal)
		require.False(t, still, "Escape did not close the journal")
		before := reads()
		require.NoError(t, d.Press(d.game.findChords().Journal.String()))
		require.NoError(t, d.Run(1))

		// Against what it draws, not against the first opening's reads: a
		// cache kept across openings would lower both to nothing.
		reopened, ok := d.game.overlays.top().(*journal)
		require.True(t, ok, "the journal key did not reopen the journal")

		t.Run("it should read again", func(t *testing.T) {
			assert.Equal(t, int64(reopened.drawn), reads()-before)
		})
	})
}
