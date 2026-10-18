package render

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// storeyPixels is how tall a storey stands on screen in an atlas cut at
// zoom, worked from buildings.STOREY (0.36 kit units) here rather than
// taken from storeyHeight, so the art is checked against its own
// number: a kit unit is the tile's diagonal across, and a vertical unit
// is foreshortened by the camera's thirty degrees.
func storeyPixels(zoom float64) float64 {
	return 0.36 * 132 / math.Sqrt2 * zoom * math.Cos(math.Pi/6)
}

// Item 109: floors can be counted, so each storey adds one storey's
// height, and every kind carries panes to darken when it stops.
func TestKindsCountTheirFloors(t *testing.T) {
	atlases, err := assets.LoadKits()
	require.NoError(t, err)
	require.NotEmpty(t, atlases)
	atlas := atlases[len(atlases)-1]
	storey := storeyPixels(atlas.Zoom)

	for _, kind := range city.DistrictTypes {
		t.Run("when the "+string(kind)+" kinds are measured at heading 0", func(t *testing.T) {
			heights := make([]float64, state.MaxStoreys)
			for n := 1; n <= state.MaxStoreys; n++ {
				sprite, ok := atlas.Sprite(fmt.Sprintf("botropolis/%s-%d", kind, n), 0)
				require.True(t, ok)
				heights[n-1] = float64(sprite.Anchor.Y)
			}

			t.Run("it should be taller at each storey than the one below", func(t *testing.T) {
				rising := true
				for i := 1; i < len(heights); i++ {
					rising = rising && heights[i] > heights[i-1]
				}
				assert.True(t, rising, "%v", heights)
			})

			t.Run("it should add about one storey's height per storey", func(t *testing.T) {
				var off []string
				for i := 1; i < len(heights); i++ {
					if step := (heights[i] - heights[i-1]) / storey; step < 0.75 || step > 1.5 {
						off = append(off, fmt.Sprintf("%d to %d storeys adds %.2f storeys", i, i+1, step))
					}
				}
				assert.Empty(t, off)
			})
		})

		for n := 1; n <= state.MaxStoreys; n++ {
			name := fmt.Sprintf("botropolis/%s-%d", kind, n)
			t.Run("when "+name+" is cut", func(t *testing.T) {
				sprite, ok := atlas.Sprite(name, 0)
				require.True(t, ok)

				t.Run("it should carry at least one pane per storey, so a stopped building can darken", func(t *testing.T) {
					assert.GreaterOrEqual(t, len(sprite.Panes), n)
				})
			})
		}
	}
}

// Item 109: frames drawn by the drive put sessions on their own scale,
// as the daemon does.
func TestDriveStoreys(t *testing.T) {
	const light, heavy = "11111111-0000-0000-0000-000000000109", "22222222-0000-0000-0000-000000000109"
	now := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	home := helpers.NewHome(t)
	for _, s := range []struct {
		id      string
		replies int
	}{{light, 1}, {heavy, 40}} {
		lines := []string{helpers.UserPrompt(s.id, "/w/kinds", "2026-10-17T10:00:00.000Z")}
		for i := range s.replies {
			lines = append(lines, helpers.AssistantReply(s.id, fmt.Sprintf("msg_%s_%d", s.id[:4], i), fmt.Sprintf("2026-10-17T10:%02d:00.000Z", i+1)))
		}
		home.Transcript(s.id, "/w/kinds", lines...)
	}

	t.Run("when the drive loads a home with sessions of different hefts", func(t *testing.T) {
		snapshot, err := driveSnapshot(home.Path, now)
		require.NoError(t, err)
		storeys := map[string]int{}
		for _, s := range snapshot.Sessions {
			storeys[s.ID] = s.Storeys
		}

		t.Run("it should give the heaviest session more storeys than the lightest", func(t *testing.T) {
			assert.Greater(t, storeys[heavy], storeys[light])
		})
	})
}
