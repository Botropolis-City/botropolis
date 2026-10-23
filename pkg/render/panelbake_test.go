package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 161: on the real history the journal and the group review drew
// every glyph of their text every frame, most of the text the city drew
// at fit. A panel that opts in is recorded instead, baked once its
// record holds for a frame, and blitted while it holds.
func TestPanelsAreBakedWhileTheyHold(t *testing.T) {
	now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.Local)
	seen := now.Add(-64 * time.Hour)
	home := helpers.ParkedWeek(t, 40, now, "Panels hold")
	// drawn runs frames, doing each before every one, and counts the
	// frames on which an open panel's words were
	// drawn, straight or into its bake.
	drawn := func(t *testing.T, d *Drive, frames int, each func()) int {
		t.Helper()
		n := 0
		for range frames {
			if each != nil {
				each()
			}
			require.NoError(t, d.Tick())
			if d.game.panels.stats.straight+d.game.panels.stats.bakes > 0 {
				n++
			}
		}
		return n
	}
	once := func(do func()) func() {
		return func() {
			if do != nil {
				do()
				do = nil
			}
		}
	}
	// settled lets a city's open panel be drawn for some frames first.
	settled := func(t *testing.T, d *Drive) *Drive {
		t.Helper()
		require.NoError(t, d.Run(60))
		return d
	}
	journalOpen := func(t *testing.T) (*Drive, *journal) {
		t.Helper()
		d, err := NewDrive(DriveOptions{Home: home, Now: now, Seen: seen})
		require.NoError(t, err)
		j, ok := d.game.overlays.top().(*journal)
		require.True(t, ok, "the journal did not open")
		return settled(t, d), j
	}

	t.Run("when the journal is open over a settled city", func(t *testing.T) {
		t.Run("and 10 frames pass with nothing changing", func(t *testing.T) {
			d, j := journalOpen(t)

			t.Run("it should draw its words on none of them", func(t *testing.T) {
				assert.Zero(t, drawn(t, d, 10, nil))
			})

			// The scrim over the city is the whole frame, and baked with
			// the panel it made each bake a frame-sized texture, 16 MiB at
			// 1920×1200 once Ebitengine rounds it to a power of two.
			t.Run("it should bake the panel, not the scrim over the whole frame", func(t *testing.T) {
				p := d.game.panels.held[j]
				require.NotNil(t, p, "the journal was not baked")
				plate, _, _ := j.plate(d.game, float64(DefaultWidth), float64(DefaultHeight))
				assert.LessOrEqual(t, float64(p.box.Dx()), plate.Width()+2*inkPad+2)
			})
		})

		for _, change := range []struct {
			name string
			do   func(t *testing.T, d *Drive, j *journal)
		}{
			{"the selection moves down", func(t *testing.T, d *Drive, _ *journal) {
				require.NoError(t, d.Press("ArrowDown"))
			}},
			{"the pointer moves onto its restore button", func(t *testing.T, d *Drive, j *journal) {
				b := j.restoreButton(d.game, float64(DefaultWidth), float64(DefaultHeight))
				require.NoError(t, d.PointAtPoint(b.Rect.Center()))
			}},
			{"the window is resized", func(t *testing.T, d *Drive, _ *journal) {
				d.screen = ebiten.NewImage(DefaultWidth-40, DefaultHeight-30)
				d.game.scene.Resize(float64(DefaultWidth)-40, float64(DefaultHeight)-30)
			}},
			{"its words change, as a relative time ticking over would", func(t *testing.T, d *Drive, j *journal) {
				j.note = "a minute later"
			}},
		} {
			t.Run("and "+change.name, func(t *testing.T) {
				d, j := journalOpen(t)
				n := drawn(t, d, 4, once(func() { change.do(t, d, j) }))

				t.Run("it should draw it again", func(t *testing.T) {
					assert.Positive(t, n)
				})
			})
		}
	})

	t.Run("when the group-review panel is open", func(t *testing.T) {
		d, err := NewDrive(DriveOptions{Home: home, Now: now})
		require.NoError(t, err)
		snapshot, err := driveSnapshot(home, now)
		require.NoError(t, err)
		var members []string
		for i := range snapshot.Sessions[:4] {
			snapshot.Sessions[i].Group = "g1"
			members = append(members, snapshot.Sessions[i].ID)
		}
		snapshot.Groups = []state.Group{{ID: "g1", Name: "PROJ-1 · review", Short: "PROJ-1", Reason: "mentions PROJ-1", Members: members}}
		d.game.Offer(snapshot)
		require.NoError(t, d.Run(1))
		require.True(t, d.game.openFirstRun(), "the group review did not open")
		settled(t, d)

		t.Run("and 10 frames pass with nothing changing", func(t *testing.T) {
			t.Run("it should draw its words on none of them", func(t *testing.T) {
				assert.Zero(t, drawn(t, d, 10, nil))
			})
		})
	})

	t.Run("when a panel that does not opt in is open (the groups board)", func(t *testing.T) {
		d, err := NewDrive(DriveOptions{Home: home, Now: now})
		require.NoError(t, err)
		d.game.clickLandmark(city.LandmarkHall)
		require.NotNil(t, d.game.board(), "the board did not open")
		settled(t, d)

		t.Run("it should keep no bake of it", func(t *testing.T) {
			assert.NotContains(t, d.game.panels.held, any(d.game.board()))
		})
	})
}
