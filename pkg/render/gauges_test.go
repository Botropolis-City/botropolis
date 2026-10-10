package render

import (
	"fmt"
	"math"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/plan"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bug 52. The axes are the other way round from item 49: the percentage
// is read ALONG the river and the boats are separated ACROSS it.
//
// Aria: "The boats should be going bottom to top and face that
// direction. They shouldn't all be in the same line. They should be
// spread out left to right as well."
//
// Every test here that loops the four headings is guarding the same
// ruling as bug 45's: a gauge whose direction reverses when the camera
// turns is worse than no gauge. The along-river axis genuinely does
// point up the screen at two headings and down at the other two, so
// which end reads 0% is chosen per heading rather than fixed to a bank.

// A run of river to read against: north end first, as city.RiverBand
// returns it.
func testRun() (from, to city.Point, width float64) {
	return city.Point{X: 100, Y: 0}, city.Point{X: 100, Y: 400}, 3 * city.CellSize
}

func headings(t *testing.T, f func(t *testing.T, cam *city.Camera)) {
	t.Helper()
	for _, heading := range []int{0, 90, 180, 270} {
		t.Run(fmt.Sprintf("when the camera faces %d", heading), func(t *testing.T) {
			cam := city.NewCamera()
			cam.Projection = city.Isometric
			cam.Heading = heading
			f(t, cam)
		})
	}
}

func TestGaugeRunsUpTheScreen(t *testing.T) {
	from, to, _ := testRun()

	headings(t, func(t *testing.T, cam *city.Camera) {
		empty := gaugeAt(cam, from, to, 0, city.Gauge{Percent: 0})
		half := gaugeAt(cam, from, to, 0, city.Gauge{Percent: 50})
		full := gaugeAt(cam, from, to, 0, city.Gauge{Percent: 100})

		// This is the question Aria asked that the frame could not
		// answer: "Where is 0% and 100%?"
		t.Run("it should put 0% below 100% on the screen", func(t *testing.T) {
			assert.Greater(t, cam.Project(empty).Y, cam.Project(full).Y)
		})

		t.Run("it should carry a rising reading up the screen", func(t *testing.T) {
			assert.Less(t, cam.Project(half).Y, cam.Project(empty).Y)
		})

		t.Run("it should keep the boat on the river's centre line in its lane", func(t *testing.T) {
			assert.InDelta(t, 100.0, half.X, 0.0001)
		})
	})
}

func TestGaugeHoldsStationRatherThanTravelling(t *testing.T) {
	from, to, _ := testRun()
	cam := city.NewCamera()
	cam.Projection = city.Isometric

	t.Run("when the same reading is placed twice", func(t *testing.T) {
		// The boats used to drift along the river as ambience, which is
		// the axis the reading now occupies. A gauge that moves on its
		// own cannot be read.
		t.Run("it should land in the same place both times", func(t *testing.T) {
			a := gaugeAt(cam, from, to, 0, city.Gauge{Percent: 40})
			b := gaugeAt(cam, from, to, 0, city.Gauge{Percent: 40})
			assert.Equal(t, a, b)
		})
	})
}

func TestGaugeFacesTheWayItReads(t *testing.T) {
	from, to, _ := testRun()

	headings(t, func(t *testing.T, cam *city.Camera) {
		// "and face that direction" — the bow points at 100%.
		t.Run("it should point the bow up the screen", func(t *testing.T) {
			assert.Less(t, cam.Project(gaugeUpstream(cam, from, to)).Y, 0.0)
		})

		t.Run("it should turn the hull to match, rather than leaving it broadside", func(t *testing.T) {
			assert.Equal(t, bowTurn(gaugeUpstream(cam, from, to)), gaugeFacing(cam, from, to))
		})
	})
}

// bowHome is art and cannot be derived from the model, so what is
// pinned here is the mapping from a world direction to a rotation given
// that home — not the home itself, which only a frame can settle. r254
// had the home backwards and every one of these cases was self
// consistently wrong, which is exactly what a test of a convention
// against itself cannot catch.
func TestBowTurn(t *testing.T) {
	for _, tc := range []struct {
		dir  city.Point
		turn int
		what string
	}{
		{city.Point{X: 0, Y: 1}, 0, "south, which is the way the hull is modelled"},
		{city.Point{X: 1, Y: 0}, 90, "east"},
		{city.Point{X: 0, Y: -1}, 180, "north"},
		{city.Point{X: -1, Y: 0}, 270, "west"},
	} {
		t.Run("when the bow should point "+tc.what, func(t *testing.T) {
			t.Run(fmt.Sprintf("it should draw the hull turned %d", tc.turn), func(t *testing.T) {
				assert.Equal(t, tc.turn, bowTurn(tc.dir))
			})
		})
	}
}

func TestGaugeLanesSeparateTheBoats(t *testing.T) {
	from, to, width := testRun()
	cam := city.NewCamera()
	cam.Projection = city.Isometric
	lanes := gaugeLanes(width, 3)
	require.Len(t, lanes, 3)

	t.Run("when three boats read exactly the same", func(t *testing.T) {
		// The case the lanes exist for: readings converge precisely
		// when you most want to compare them.
		same := city.Gauge{Percent: 60}
		var at []city.Point
		for _, lane := range lanes {
			at = append(at, gaugeAt(cam, from, to, lane, same))
		}

		t.Run("it should hold them apart rather than stack them", func(t *testing.T) {
			assert.NotEqual(t, at[0], at[1])
		})

		t.Run("it should separate them across the river, not along it", func(t *testing.T) {
			assert.InDelta(t, at[0].Y, at[1].Y, 0.0001)
		})

		t.Run("it should read all three at the same distance up the run", func(t *testing.T) {
			assert.InDelta(t, cam.Project(at[0]).Y-cam.Project(at[1]).Y,
				cam.Project(at[1]).Y-cam.Project(at[2]).Y, 0.0001)
		})
	})
}

func TestGaugeMarksTheEnds(t *testing.T) {
	from, to, width := testRun()
	cam := city.NewCamera()
	cam.Projection = city.Isometric
	marks := gaugeMarks(cam, from, to, width)

	t.Run("when the run is marked out", func(t *testing.T) {
		// Item 49's marks sat at halfway and the limit only, across the
		// river. The ends are what answers "where is 0% and 100%".
		t.Run("it should set a mark at each end and one at halfway", func(t *testing.T) {
			assert.Len(t, marks, 3)
		})

		t.Run("it should put the 0% mark lowest on the screen", func(t *testing.T) {
			assert.Greater(t, cam.Project(marks[0].At).Y, cam.Project(marks[2].At).Y)
		})

		t.Run("it should flag the limit and leave the others plain", func(t *testing.T) {
			assert.Equal(t, []bool{false, false, true},
				[]bool{marks[0].Limit, marks[1].Limit, marks[2].Limit})
		})

		t.Run("it should set the marks off the lanes so they never foul a boat", func(t *testing.T) {
			assert.Greater(t, math.Abs(marks[0].At.X-100), 1.0)
		})
	})
}

// The guard that keeps RiverCols, the hull beams and the lanes honest.
// The river's width and the hulls' sizes are two things that must
// agree, and this project has paid for that shape repeatedly — so
// nothing here is asserted as a number, it is re-derived from the art
// every run.
func TestRiverFitsThreeHullsAbreast(t *testing.T) {
	atlases, err := assets.LoadKits()
	require.NoError(t, err)
	var z1 assets.KitAtlas
	for _, a := range atlases {
		if a.Zoom == 1 {
			z1 = a
		}
	}
	require.NotZero(t, z1.Zoom)

	measured := func(size city.GaugeSize) float64 {
		hull := gaugeHulls[size]
		s, ok := z1.Sprite(hull.piece, 0)
		require.True(t, ok, hull.piece)
		w, h := float64(s.Rect.Dx()), float64(s.Rect.Dy())
		scale := hull.extent / math.Max(w, h)
		return math.Min(w, h) * scale / (2 * city.IsoScale * z1.Zoom)
	}

	t.Run("when each hull's declared beam is checked against its sprite", func(t *testing.T) {
		// The lanes are laid out from these numbers, so if the art
		// moves and they do not, the boats quietly start overlapping.
		for _, size := range []city.GaugeSize{city.GaugeBig, city.GaugeMedium, city.GaugeSmall} {
			t.Run("it should still be what the art measures", func(t *testing.T) {
				assert.InDelta(t, measured(size), gaugeHulls[size].beam, 0.5)
			})
		}
	})

	width := float64(plan.RiverCols) * city.CellSize
	lanes := gaugeLanes(width, 3)
	require.Len(t, lanes, 3)

	t.Run("when the big boat rides beside the medium one", func(t *testing.T) {
		// The binding pair. Big and small are never adjacent, so the
		// widest two that ever sit side by side are these.
		need := (measured(city.GaugeBig) + measured(city.GaugeMedium)) / 2

		t.Run("it should leave water between their hulls", func(t *testing.T) {
			assert.Greater(t, lanes[1]-lanes[0], need,
				"a %.0f-unit river gives %.1f units of lane spacing, and the two hulls need %.1f",
				width, lanes[1]-lanes[0], need)
		})
	})

	// Bug 55: the outer lanes used to be pinned by the widest hull's
	// half beam, which put that hull exactly on the bank — zero
	// clearance, by construction rather than by choice.
	t.Run("when the outermost boats ride their lanes", func(t *testing.T) {
		t.Run("it should keep the widest hull off the near bank", func(t *testing.T) {
			assert.InDelta(t, gaugeBankClear, width/2+lanes[0]-measured(city.GaugeBig)/2, 0.5)
		})

		t.Run("it should keep the narrowest hull off the far bank", func(t *testing.T) {
			assert.InDelta(t, gaugeBankClear, width/2-lanes[2]-measured(city.GaugeSmall)/2, 0.5)
		})
	})
}

// Bug 55. The scale used to run the river's whole length, so a low
// reading — which is most readings — drew every boat in the map's
// corner, where Aria could not find them.
func TestGaugeRunsBesideTheCityNotTheMapsEdge(t *testing.T) {
	c := city.Build(state.Snapshot{At: time.Now(), Sessions: []state.Session{
		{ID: "a", CWD: "/home/avesta/workspaces/github/auroq/botropolis", State: state.Working},
		{ID: "b", CWD: "/home/avesta/workspaces/github/mCedar/mullet", State: state.Working},
	}}, city.NewLayout())
	require.NotNil(t, c)

	river, mouth, _, ok := c.RiverBand()
	require.True(t, ok)
	from, to, _, ok := riverRun(c)
	require.True(t, ok)

	t.Run("when the scale is laid along the river", func(t *testing.T) {
		t.Run("it should start well inside the river's own head", func(t *testing.T) {
			assert.Greater(t, from.Y, river.Y+gaugeEndroom-0.001)
		})

		t.Run("it should stop well short of the river's mouth at the map's corner", func(t *testing.T) {
			assert.Less(t, to.Y, mouth.Y-gaugeEndroom+0.001)
		})

		t.Run("it should still be long enough to read a percentage along", func(t *testing.T) {
			assert.Greater(t, to.Y-from.Y, 10*city.CellSize)
		})

		t.Run("it should keep the boats on the river's own centre line", func(t *testing.T) {
			assert.InDelta(t, river.X, from.X, 0.0001)
		})
	})
}

func TestGaugeInTheFlatView(t *testing.T) {
	from, to, width := testRun()

	t.Run("when the map is drawn flat", func(t *testing.T) {
		cam := city.NewCamera()

		// Across the river projects onto one screen row in the flat
		// view, so there is no "right" to fan the lanes along. It still
		// must not flip, so the tie breaks the same way every time.
		t.Run("it should still pick a direction to lane the boats along", func(t *testing.T) {
			assert.Equal(t, 1.0, gaugeRightward(cam).X)
		})

		t.Run("it should still read the run up the screen", func(t *testing.T) {
			assert.Less(t, cam.Project(gaugeUpstream(cam, from, to)).Y, 0.0)
		})

		t.Run("it should still mark both ends", func(t *testing.T) {
			assert.Len(t, gaugeMarks(cam, from, to, width), 3)
		})
	})
}

// Item 79: the boats drive between readings and sail in and out, so a
// track carries on past both ends of the run to wherever the river ends.
func TestGaugeTrack(t *testing.T) {
	from, to, _ := testRun()
	const back, ahead = 50.0, 70.0

	headings(t, func(t *testing.T, cam *city.Camera) {
		_, up, _ := gaugeRun(cam, from, to)
		empty := gaugeAt(cam, from, to, 0, city.Gauge{Percent: 0})
		full := gaugeAt(cam, from, to, 0, city.Gauge{Percent: 100})

		t.Run("it should hold a reading's track at the reading", func(t *testing.T) {
			assert.Equal(t, gaugeAt(cam, from, to, 0, city.Gauge{Percent: 35}), gaugeTrackAt(cam, from, to, 0, 0.35, back, ahead))
		})

		t.Run("it should put the way in below the run by its reach", func(t *testing.T) {
			at := gaugeTrackAt(cam, from, to, 0, city.GaugeOffLow, back, ahead)
			assert.InDelta(t, -back, at.Sub(empty).X*up.X+at.Sub(empty).Y*up.Y, 1e-9)
		})

		t.Run("it should put the way out above the run by its reach", func(t *testing.T) {
			at := gaugeTrackAt(cam, from, to, 0, city.GaugeOffHigh, back, ahead)
			assert.InDelta(t, ahead, at.Sub(full).X*up.X+at.Sub(full).Y*up.Y, 1e-9)
		})
	})
}
