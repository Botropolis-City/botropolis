package render

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 247, Aria 2026-10-25: "The roads don't attach to the ramps.
// They're half a road width off." The ramp climbs from the landing's
// centre along its row; the road piece laid on the landing must carry
// its carriageway along that same line, so where the ramp starts the
// road's middle and the ramp's middle are one point on screen.
func TestRoadMeetsTheRamp(t *testing.T) {
	atlases, err := assets.LoadKits()
	require.NoError(t, err)
	start := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	cityOf := func(districts int) *city.City {
		var sessions []state.Session
		for i := 0; i < districts; i++ {
			sessions = append(sessions, state.Session{ID: fmt.Sprint(i), CWD: fmt.Sprintf("/p/d%02d", i), State: state.Working, StartedAt: start})
		}
		return city.Build(state.Snapshot{At: start, Sessions: sessions}, city.NewLayout())
	}
	names := []string{"first", "second"}

	for _, f := range []struct {
		name      string
		districts int
	}{
		{"when an island of two districts has one bridge", 2},
		{"when an island of seventeen districts has two", 17},
	} {
		t.Run(f.name, func(t *testing.T) {
			c := cityOf(f.districts)
			masks := map[city.Cell]int{}
			for _, sc := range c.StreetCells {
				masks[sc.Cell] = sc.Mask
			}
			for i, b := range c.Bridges() {
				land := city.CellOf(b.Land)
				mask, laid := masks[land]
				require.True(t, laid, "precondition: road is laid on the %s landing", names[i])
				for _, atlas := range atlases {
					for _, heading := range []int{0, 90, 180, 270} {
						cam := city.NewCamera()
						cam.Projection, cam.Zoom, cam.Heading = city.Isometric, atlas.Zoom, heading
						off, ok := roadOffRamp(atlas, cam, mask, b)

						t.Run(fmt.Sprintf("and the %s bridge at zoom %g and heading %d, the road's middle should meet the ramp's within %g pixels", names[i], atlas.Zoom, heading, rampMeetSlack), func(t *testing.T) {
							require.True(t, ok, "no road crosses the line the ramp starts on")
							assert.LessOrEqual(t, off, rampMeetSlack)
						})
					}
				}
			}
		})
	}
}

// rampMeetSlack is how far apart on screen the road's middle and the
// ramp's may be. The kit's own straight piece, measured this way, sits
// up to 2.5 pixels off its cell's middle at some headings, from where
// the atlas rounds its anchor, so a pixel is finer than the art; the
// bend that was there measured 5 to 23.
const rampMeetSlack = 3.0

// roadOffRamp is how far apart on screen, in pixels, the middle of the
// carriageway the landing's road piece draws across the line the ramp
// starts on and the middle of the ramp's start are, read off the atlas.
func roadOffRamp(atlas assets.KitAtlas, cam *city.Camera, mask int, b city.Bridge) (float64, bool) {
	name, turn := roadPiece(mask)
	s, ok := atlas.Sprite(name, cam.Heading-turn)
	if !ok {
		return 0, false
	}
	scale := cam.Zoom / atlas.Zoom * groundBleed
	foot := cam.WorldToScreen(b.Land)
	row := city.CellOf(b.Land).Rect()
	// The road is the longest unbroken run of it across the row, kerb to
	// kerb: the slab's dark edge past the verge is not joined to it.
	first, last := 0.0, -1.0
	runFrom, inRun := 0.0, false
	const samples = 200
	for k := 0; k <= samples+1; k++ {
		y := row.Min.Y + (row.Max.Y-row.Min.Y)*float64(k)/samples
		on := false
		if k <= samples {
			p := cam.WorldToScreen(city.Point{X: b.Land.X, Y: y}).Sub(foot).Scale(1 / scale)
			at := image.Pt(s.Rect.Min.X+s.Anchor.X+int(math.Round(p.X)), s.Rect.Min.Y+s.Anchor.Y+int(math.Round(p.Y)))
			on = at.In(s.Rect) && paved(color.NRGBAModel.Convert(atlas.Pages[s.Page].At(at.X, at.Y)).(color.NRGBA))
		}
		switch {
		case on && !inRun:
			runFrom, inRun = y, true
		case !on && inRun:
			end := y - (row.Max.Y-row.Min.Y)/samples
			if end-runFrom > last-first {
				first, last = runFrom, end
			}
			inRun = false
		}
	}
	if first > last {
		return 0, false
	}
	middle := cam.WorldToScreen(city.Point{X: b.Land.X, Y: (first + last) / 2})
	return math.Hypot(middle.X-foot.X, middle.Y-foot.Y), true
}

// paved is a pixel of the road, its kerbs and lane line included: opaque
// and not the green verge.
func paved(c color.NRGBA) bool {
	return c.A > 64 && (c.R >= 100 || c.G <= 100)
}
