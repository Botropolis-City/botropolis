package acceptance_test

import (
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/demo"
	"botropolis.city/botropolis/pkg/measure"
	"botropolis.city/botropolis/pkg/render"
)

// The harness the bar's tasks are measured with: the city run a tick at a
// time on a staged demo home, driven with its own keys and clicks, with
// what it drew read back. Item 91. These tests are of the harness itself;
// they do not run in parallel, because the work counters are global.

func stageDemo(t testing.TB, name string) string {
	t.Helper()
	return stage(t, filepath.Join("..", "..", "demo", "scenarios", name+".yaml"), time.Now())
}

// stage stages a scenario from the demo corpus as if it were now.
func stage(t testing.TB, path string, now time.Time) string {
	t.Helper()
	scenario, err := demo.LoadScenario(path)
	require.NoError(t, err)
	home := t.TempDir()
	corpus := demo.Corpus{Dir: filepath.Join("..", "..", "demo", "corpus")}
	require.NoError(t, demo.Stage(corpus, scenario, home, now, demo.Sleepers(home, 10*time.Minute)))
	t.Cleanup(func() { _ = demo.Unstage(home) })
	return home
}

func newDrive(t *testing.T, home string) *render.Drive {
	t.Helper()
	d, err := render.NewDrive(render.DriveOptions{Home: home})
	require.NoError(t, err)
	return d
}

func transcriptFiles(t *testing.T, home string) int {
	t.Helper()
	n := 0
	require.NoError(t, filepath.WalkDir(filepath.Join(home, ".claude", "projects"), func(path string, e fs.DirEntry, err error) error {
		if err == nil && !e.IsDir() && filepath.Ext(path) == ".jsonl" {
			n++
		}
		return err
	}))
	return n
}

const settleTicks = 10

func TestDrive(t *testing.T) {
	home := stageDemo(t, "hamlet")

	t.Run("when the city is driven on the hamlet demo", func(t *testing.T) {
		t.Run("and nothing has been sent", func(t *testing.T) {
			d := newDrive(t, home)
			require.NotEmpty(t, d.Sessions(), "no sessions staged: the check below passes on nothing")

			t.Run("it should have counted no steps", func(t *testing.T) {
				assert.Equal(t, 0, d.Steps())
			})

			t.Run("it should draw a building for every session in the fixture", func(t *testing.T) {
				assert.Len(t, d.Shown().Buildings, len(d.Sessions()))
			})
		})

		t.Run("and a key the city ignores is pressed", func(t *testing.T) {
			d := newDrive(t, home)
			before := d.Shown()
			require.NoError(t, d.Press("k"))
			require.NoError(t, d.Run(settleTicks))

			t.Run("it should count one step", func(t *testing.T) {
				assert.Equal(t, 1, d.Steps())
			})

			t.Run("it should leave nothing opened or selected after ten ticks", func(t *testing.T) {
				// Labels are where the map put its names, which move as the
				// city settles; they are neither opened nor selected (item 102).
				after := d.Shown()
				before.Labels, after.Labels = nil, nil
				assert.Equal(t, before, after)
			})
		})

		t.Run("and ? is pressed", func(t *testing.T) {
			d := newDrive(t, home)
			require.NoError(t, d.Press("?"))
			ticks, err := d.Until(func(s render.Shown) bool { return s.Help }, settleTicks)
			require.NoError(t, err)

			t.Run("it should count one step", func(t *testing.T) {
				assert.Equal(t, 1, d.Steps())
			})

			t.Run("it should draw the help within one tick", func(t *testing.T) {
				assert.Equal(t, 1, ticks)
			})
		})

		t.Run("and ? is pressed, then escape", func(t *testing.T) {
			d := newDrive(t, home)
			require.NoError(t, d.Press("?"))
			require.NoError(t, d.Press("Escape"))
			require.NoError(t, d.Run(settleTicks))

			t.Run("it should count two steps", func(t *testing.T) {
				assert.Equal(t, 2, d.Steps())
			})

			t.Run("it should no longer draw the help", func(t *testing.T) {
				assert.False(t, d.Shown().Help)
			})
		})

		t.Run("and the view is zoomed in with +", func(t *testing.T) {
			d := newDrive(t, home)
			require.NoError(t, d.Press("equal"))
			ticks, err := d.Until(func(render.Shown) bool { return true }, 4*settleTicks)
			require.NoError(t, err)

			t.Run("it should not count the frame as shown until the camera has settled", func(t *testing.T) {
				assert.Greater(t, ticks, 1)
			})
		})

		t.Run("and a building is clicked", func(t *testing.T) {
			d := newDrive(t, home)
			id := d.Sessions()[0]
			require.NoError(t, d.Click(id))
			ticks, err := d.Until(func(s render.Shown) bool { return s.Selected == id }, settleTicks)
			require.NoError(t, err)

			t.Run("it should count one step", func(t *testing.T) {
				assert.Equal(t, 1, d.Steps())
			})

			t.Run("it should select that building's session within two ticks", func(t *testing.T) {
				assert.LessOrEqual(t, ticks, 2)
			})
		})

		t.Run("and the same inputs are sent to two fresh cities", func(t *testing.T) {
			var ticks [2]int
			for i := range ticks {
				d := newDrive(t, home)
				require.NoError(t, d.Click(d.Sessions()[0]))
				require.NoError(t, d.Press("?"))
				n, err := d.Until(func(s render.Shown) bool { return s.Help && s.Selected != "" }, settleTicks)
				require.NoError(t, err)
				ticks[i] = n
			}

			t.Run("it should take the same number of ticks in both", func(t *testing.T) {
				assert.Equal(t, ticks[0], ticks[1])
			})
		})
	})

	t.Run("when the fixture is loaded", func(t *testing.T) {
		before := measure.Take()
		newDrive(t, home)
		work := measure.Take().Since(before)

		// Once for the map and once for the finder's index, as the
		// daemon reads them.
		t.Run("it should count two transcripts opened per transcript in the fixture", func(t *testing.T) {
			assert.Equal(t, 2*int64(transcriptFiles(t, home)), work[measure.TranscriptsOpened])
		})
	})
}
