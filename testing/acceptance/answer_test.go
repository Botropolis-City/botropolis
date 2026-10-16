package acceptance_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/measure"
	"botropolis.city/botropolis/pkg/render"
	"botropolis.city/botropolis/testing/helpers"
)

// Answering a session and getting back to the map: one click there and
// one key back. The click is on the needs-you line in the footer, which
// attaches the session that has waited longest; the key back is pressed
// in the terminal, outside the city, so only the click is measured here.
// The reading pane's route is recorded beside it. Item 91.

const (
	answerSteps   = 1
	answerTicks   = 1
	readingSteps  = 3
	waitingLine   = "needs-you line"
	longestTitle  = "Tide table in station time"
	hamletWaiting = "Sea conditions in the daily report"
)

type answered struct {
	steps int
	ticks int
	acted []city.Action
	work  measure.Work
	shown render.Shown
}

func drive(t testing.TB, home string) *render.Drive {
	t.Helper()
	d, err := render.NewDrive(render.DriveOptions{Home: home, Now: monday})
	require.NoError(t, err)
	return d
}

// runAnswer clicks the needs-you line and waits for the attach.
func runAnswer(t testing.TB, home string) answered {
	t.Helper()
	d := drive(t, home)
	var a answered
	a.shown = d.Shown()
	before := measure.Take()
	require.NoError(t, d.Click(waitingLine))
	var err error
	a.ticks, err = d.UntilActed(func(c city.Action) bool { return c.Kind == city.ActionAttach }, findLimit)
	require.NoError(t, err)
	a.work = measure.Take().Since(before)
	a.steps = d.Steps()
	a.acted = d.Acted()
	return a
}

func TestAnswerAndComeBack(t *testing.T) {
	t.Run("when the needs-you line is clicked", func(t *testing.T) {
		home := stage(t, filepath.Join("..", "..", "demo", "scenarios", "hamlet.yaml"), monday)
		a := runAnswer(t, home)
		d := drive(t, home)

		t.Run("it should take one step", func(t *testing.T) {
			assert.Equal(t, answerSteps, a.steps)
		})

		t.Run("it should attach that session", func(t *testing.T) {
			assert.Equal(t, hamletWaiting, d.Title(a.acted[len(a.acted)-1].SessionID))
		})

		t.Run("it should hand the attach over within one tick of the click", func(t *testing.T) {
			assert.LessOrEqual(t, a.ticks, answerTicks)
		})

		t.Run("it should open no transcripts on the way", func(t *testing.T) {
			assert.Zero(t, a.work[measure.TranscriptsOpened])
		})
	})

	t.Run("when two sessions need you", func(t *testing.T) {
		home := stage(t, filepath.Join("testdata", "waiting.yaml"), monday)
		a := runAnswer(t, home)
		d := drive(t, home)

		t.Run("it should name the one that has waited longest", func(t *testing.T) {
			assert.Equal(t, "needs-you: "+longestTitle+" (+1 more)", a.shown.Waiting)
		})

		t.Run("it should attach the one the line names", func(t *testing.T) {
			assert.Equal(t, longestTitle, d.Title(a.acted[len(a.acted)-1].SessionID))
		})
	})

	t.Run("when nothing needs you", func(t *testing.T) {
		where := drive(t, stage(t, filepath.Join("..", "..", "demo", "scenarios", "hamlet.yaml"), monday)).Shown().WaitingRect
		require.Positive(t, where.Area(), "the line was not drawn where something waits, so there is nowhere to click")
		d := drive(t, stage(t, filepath.Join("testdata", "week.yaml"), monday))
		require.NoError(t, d.ClickAt(where.Center()))
		require.NoError(t, d.Run(settleTicks))

		t.Run("it should not draw the line", func(t *testing.T) {
			assert.Empty(t, d.Shown().Waiting)
		})

		t.Run("it should attach nothing when clicked where the line would be", func(t *testing.T) {
			for _, c := range d.Acted() {
				assert.NotEqual(t, city.ActionAttach, c.Kind)
			}
		})
	})

	t.Run("when a session comes to need you", func(t *testing.T) {
		d := drive(t, stage(t, filepath.Join("testdata", "week.yaml"), monday))
		require.Empty(t, d.Shown().Waiting)
		require.NoError(t, d.Feed(stage(t, filepath.Join("..", "..", "demo", "scenarios", "hamlet.yaml"), monday)))
		ticks, err := d.Until(func(s render.Shown) bool { return s.Waiting != "" }, findLimit)
		require.NoError(t, err)

		t.Run("it should draw the line within one tick of the news", func(t *testing.T) {
			assert.LessOrEqual(t, ticks, 1)
		})
	})

	t.Run("when it is answered through the reading pane instead: a click, o and Enter", func(t *testing.T) {
		d := drive(t, stage(t, filepath.Join("..", "..", "demo", "scenarios", "hamlet.yaml"), monday))
		var id string
		for _, s := range d.Sessions() {
			if d.Title(s) == hamletWaiting {
				id = s
			}
		}
		require.NotEmpty(t, id)
		before := measure.Take()
		require.NoError(t, d.Click(id))
		require.NoError(t, d.Press("o"))
		require.NoError(t, d.Run(1))
		opened := measure.Take().Since(before)[measure.TranscriptsOpened]
		require.NoError(t, d.Press("Enter"))
		_, err := d.UntilActed(func(c city.Action) bool { return c.Kind == city.ActionAttach }, findLimit)
		require.NoError(t, err)

		t.Run("it should take at most today's three steps", func(t *testing.T) {
			assert.LessOrEqual(t, d.Steps(), readingSteps)
		})

		t.Run("it should read exactly one transcript to open the pane", func(t *testing.T) {
			assert.Equal(t, int64(1), opened)
		})

		t.Run("it should attach the session it read", func(t *testing.T) {
			assert.Equal(t, id, d.Acted()[len(d.Acted())-1].SessionID)
		})
	})

	t.Run("when the line is clicked on 1,000 generated sessions besides", func(t *testing.T) {
		home := stage(t, filepath.Join("..", "..", "demo", "scenarios", "hamlet.yaml"), monday)
		helpers.ParkedWeekIn(t, home, 1000, monday, reopenTitle)
		a := runAnswer(t, home)

		t.Run("it should still open no transcripts", func(t *testing.T) {
			assert.Zero(t, a.work[measure.TranscriptsOpened])
		})
	})
}
