package acceptance_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/measure"
	"botropolis.city/botropolis/pkg/render"
	"botropolis.city/botropolis/testing/helpers"
)

// Reopening last Friday's session, with the keys the city has today: the
// finder narrows the map by a word of its title, a click selects the one
// building left lit, and Enter resumes it. The ceilings are what that
// costs now; the items that improve the task lower them. Item 91.

// monday is the morning the week is seen from, so last Friday is three
// days back whatever day the test runs.
var monday = time.Date(2026, 10, 12, 10, 0, 0, 0, time.Local)

const (
	reopenWord     = "battery"
	reopenTitle    = "Battery check compares as text"
	reopenSteps    = 5
	reopenTicks    = 1
	reopenTickSlop = 4 * settleTicks
)

// reopen is the task: what it took, and what it asked the city to do.
type reopen struct {
	lit    []string
	steps  int
	ticks  int
	acted  city.Action
	work   measure.Work
	target string
}

func runReopen(t *testing.T, home string) reopen {
	t.Helper()
	d, err := render.NewDrive(render.DriveOptions{Home: home, Now: monday})
	require.NoError(t, err)
	require.Greater(t, len(d.Sessions()), 1, "with one session there is nothing to find it among")
	before := measure.Take()
	require.NoError(t, d.Press("slash"))
	require.NoError(t, d.Type(reopenWord))
	require.NoError(t, d.Press("Enter"))
	require.NoError(t, d.Run(0))
	var r reopen
	r.lit = d.Shown().Lit
	require.Len(t, r.lit, 1, "the finder should leave one building lit to click")
	r.target = r.lit[0]
	require.NoError(t, d.Click(r.target))
	require.NoError(t, d.Press("Enter"))
	r.ticks, err = d.UntilActed(func(a city.Action) bool { return a.Kind == city.ActionResume }, reopenTickSlop)
	require.NoError(t, err)
	r.work = measure.Take().Since(before)
	r.steps = d.Steps()
	r.acted = d.Acted()[len(d.Acted())-1]
	return r
}

func titleOf(t *testing.T, home, id string) string {
	t.Helper()
	d, err := render.NewDrive(render.DriveOptions{Home: home, Now: monday})
	require.NoError(t, err)
	return d.Title(id)
}

func TestReopenLastFriday(t *testing.T) {
	t.Run("when last Friday's session is reopened with the finder, a click and enter", func(t *testing.T) {
		home := stage(t, filepath.Join("testdata", "week.yaml"), monday)
		r := runReopen(t, home)

		t.Run("it should leave only last Friday's session lit", func(t *testing.T) {
			assert.Equal(t, reopenTitle, titleOf(t, home, r.target))
		})

		t.Run("it should take at most five steps", func(t *testing.T) {
			assert.LessOrEqual(t, r.steps, reopenSteps)
		})

		t.Run("it should resume that session, not another", func(t *testing.T) {
			assert.Equal(t, city.Action{Kind: city.ActionResume, SessionID: r.target}, r.acted)
		})

		t.Run("it should hand the resume over within a tick of the last input", func(t *testing.T) {
			assert.LessOrEqual(t, r.ticks, reopenTicks)
		})

		t.Run("it should open no transcripts on the way", func(t *testing.T) {
			assert.Zero(t, r.work[measure.TranscriptsOpened])
		})
	})

	t.Run("when the city loads 100 and then 1,000 generated sessions", func(t *testing.T) {
		opened := map[int]int64{}
		for _, n := range []int{100, 1000} {
			home := helpers.ParkedWeek(t, n, monday, reopenTitle)
			before := measure.Take()
			d, err := render.NewDrive(render.DriveOptions{Home: home, Now: monday})
			require.NoError(t, err)
			opened[n] = measure.Take().Since(before)[measure.TranscriptsOpened]
			require.Len(t, d.Sessions(), n, "a city missing sessions would bound the work on fewer than it claims")
		}
		require.Positive(t, opened[100], "nothing opened: the bound below holds on nothing")

		t.Run("it should open at most ten times the transcripts for ten times the sessions", func(t *testing.T) {
			assert.LessOrEqual(t, opened[1000], 10*opened[100])
		})
	})

	t.Run("when the same task is run on 1,000 sessions", func(t *testing.T) {
		r := runReopen(t, helpers.ParkedWeek(t, 1000, monday, reopenTitle))

		t.Run("it should still open no transcripts", func(t *testing.T) {
			assert.Zero(t, r.work[measure.TranscriptsOpened])
		})
	})
}
