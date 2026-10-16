package acceptance_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/measure"
	"botropolis.city/botropolis/pkg/render"
	"botropolis.city/botropolis/testing/helpers"
)

// Finding the session where something was said: the hotkey, three words
// and Enter, with no tokens spent. The words are in one session's prompt
// and nowhere else in the week. Item 91.

const (
	findWords   = "kiln held minutes"
	findTitle   = "Hold time ignores the sample rate"
	findSteps   = 5
	findTicks   = 1
	findLimit   = 4 * settleTicks
	generatedOn = "Session number 0042 said kiln held minutes"
)

type found struct {
	steps  int
	ticks  int
	stale  string
	acted  city.Action
	work   measure.Work
	titles func(string) string
}

// runFind sends the hotkey, the words and Enter, with nothing between
// them: Enter is pressed on the tick after the last letter. With pause,
// the first word's results are waited for before the rest is typed, so
// an earlier query's answer is on show when Enter comes.
func runFind(t testing.TB, home string, pause bool) found {
	t.Helper()
	d, err := render.NewDrive(render.DriveOptions{Home: home, Now: monday})
	require.NoError(t, err)
	require.Greater(t, len(d.Sessions()), 1, "with one session there is nothing to find it among")
	work := measure.Take()
	require.NoError(t, d.Press("slash"))
	words := strings.Fields(findWords)
	var f found
	if pause {
		require.NoError(t, d.Type(words[0]+" "))
		_, err := d.Until(func(s render.Shown) bool { return len(s.Found) > 0 }, findLimit)
		require.NoError(t, err, "the first word found nothing, so nothing earlier is on show")
		f.stale = d.Shown().Found[0]
		words = words[1:]
	}
	require.NoError(t, d.Type(strings.Join(words, " ")))
	require.NoError(t, d.Press("Enter"))
	f.ticks, err = d.UntilActed(func(a city.Action) bool { return a.Kind == city.ActionResume || a.Kind == city.ActionAttach }, findLimit)
	require.NoError(t, err)
	f.work = measure.Take().Since(work)
	f.steps = d.Steps()
	f.acted = d.Acted()[len(d.Acted())-1]
	f.titles = d.Title
	return f
}

func TestFindByThreeWords(t *testing.T) {
	t.Run("when the hotkey, three words and Enter are sent", func(t *testing.T) {
		f := runFind(t, stage(t, filepath.Join("testdata", "week.yaml"), monday), false)

		t.Run("it should take five steps", func(t *testing.T) {
			assert.Equal(t, findSteps, f.steps)
		})

		t.Run("it should resume the session where the words were said", func(t *testing.T) {
			assert.Equal(t, findTitle, f.titles(f.acted.SessionID))
		})

		t.Run("it should resume it rather than attach", func(t *testing.T) {
			assert.Equal(t, city.ActionResume, f.acted.Kind)
		})

		t.Run("it should hand the resume over within one tick of Enter", func(t *testing.T) {
			assert.LessOrEqual(t, f.ticks, findTicks)
		})

		t.Run("it should open no transcripts for the query", func(t *testing.T) {
			assert.Zero(t, f.work[measure.TranscriptsOpened])
		})
	})

	t.Run("when the three words are typed and their results are drawn", func(t *testing.T) {
		d, err := render.NewDrive(render.DriveOptions{Home: stage(t, filepath.Join("testdata", "week.yaml"), monday), Now: monday})
		require.NoError(t, err)
		require.NoError(t, d.Press("slash"))
		require.NoError(t, d.Type(findWords))
		_, err = d.Until(func(s render.Shown) bool { return len(s.Found) > 0 }, findLimit)
		require.NoError(t, err)

		t.Run("it should head the results with the session where the words were said", func(t *testing.T) {
			assert.Equal(t, findTitle, d.Title(d.Shown().Found[0]))
		})
	})

	t.Run("when Enter comes after an earlier query's results are on show", func(t *testing.T) {
		f := runFind(t, stage(t, filepath.Join("testdata", "week.yaml"), monday), true)
		require.NotEqual(t, findTitle, f.titles(f.stale), "the first word's top result is already the answer, so nothing stale is tested")

		t.Run("it should open the current query's top result, not the earlier one's", func(t *testing.T) {
			assert.Equal(t, findTitle, f.titles(f.acted.SessionID))
		})
	})

	t.Run("when the index is built over 100 and then 1,000 generated sessions", func(t *testing.T) {
		opened := map[int]int64{}
		for _, n := range []int{100, 1000} {
			home := helpers.ParkedWeek(t, n, monday, generatedOn)
			before := measure.Take()
			d, err := render.NewDrive(render.DriveOptions{Home: home, Now: monday})
			require.NoError(t, err)
			opened[n] = measure.Take().Since(before)[measure.TranscriptsOpened]
			require.Len(t, d.Sessions(), n, "a city missing sessions would bound the work on fewer than it claims")
		}
		require.Positive(t, opened[100], "nothing read: the bound below holds on nothing")

		t.Run("it should read at most ten times the transcripts for ten times the sessions", func(t *testing.T) {
			assert.LessOrEqual(t, opened[1000], 10*opened[100])
		})
	})

	t.Run("when the same query is run on 1,000 sessions", func(t *testing.T) {
		f := runFind(t, helpers.ParkedWeek(t, 1000, monday, generatedOn), false)

		t.Run("it should still open no transcripts", func(t *testing.T) {
			assert.Zero(t, f.work[measure.TranscriptsOpened])
		})
	})
}
