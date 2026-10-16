package acceptance_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/measure"
	"botropolis.city/botropolis/pkg/render"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Starting a session in a directory with nothing running: two clicks and
// a prompt. Today it is a click on a building in the directory, Shift+N
// for New session with the directory filled in, and the prompt with its
// Enter. Item 91.

const (
	startSteps  = 3
	startTicks  = 1
	startPrompt = "add a test for the empty plot"
)

type started struct {
	steps int
	ticks int
	acted city.Action
	want  string
	work  measure.Work
}

func runStart(t testing.TB, home string) started {
	t.Helper()
	d, err := render.NewDrive(render.DriveOptions{Home: home, Now: monday})
	require.NoError(t, err)
	ids := d.Sessions()
	require.NotEmpty(t, ids)
	target := ids[0]
	dir := city.ProjectRoot(d.Dir(target))
	for _, id := range ids {
		if city.ProjectRoot(d.Dir(id)) == dir {
			require.Equal(t, string(state.Resting), d.State(id), "a session is running in the directory, so it is not one with nothing running")
		}
	}
	before := measure.Take()
	require.NoError(t, d.Click(target))
	require.NoError(t, d.Press("shift+n"))
	require.NoError(t, d.Prompt(startPrompt))
	var s started
	s.ticks, err = d.UntilActed(func(a city.Action) bool { return a.Kind == city.ActionNewSession }, findLimit)
	require.NoError(t, err)
	s.work = measure.Take().Since(before)
	s.steps = d.Steps()
	s.acted = d.Acted()[len(d.Acted())-1]
	s.want = dir
	return s
}

func TestStartASessionAnywhere(t *testing.T) {
	t.Run("when New session is opened on a directory with nothing running and a prompt is typed", func(t *testing.T) {
		s := runStart(t, stage(t, filepath.Join("testdata", "week.yaml"), monday))

		t.Run("it should take at most three steps", func(t *testing.T) {
			assert.LessOrEqual(t, s.steps, startSteps)
		})

		t.Run("it should start the session in that directory with that prompt", func(t *testing.T) {
			assert.Equal(t, [2]string{s.want, startPrompt}, [2]string{s.acted.Dir, s.acted.Prompt})
		})

		t.Run("it should hand the start over within one tick of Enter", func(t *testing.T) {
			assert.LessOrEqual(t, s.ticks, startTicks)
		})

		t.Run("it should open no transcripts on the way", func(t *testing.T) {
			assert.Zero(t, s.work[measure.TranscriptsOpened])
		})
	})

	t.Run("when the same is done on 1,000 generated sessions", func(t *testing.T) {
		s := runStart(t, helpers.ParkedWeek(t, 1000, monday, reopenTitle))

		t.Run("it should still open no transcripts", func(t *testing.T) {
			assert.Zero(t, s.work[measure.TranscriptsOpened])
		})
	})
}
