package render

import (
	"fmt"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 239, Aria 2026-10-24: the deck fades into fog. A still bank over
// the haze and a thin mist along that coast, drawn over the deck's
// fading span and the trucks in it, hoverable by nobody, and clear of
// the river's mouths, the boats' run and the depot.

const fogWidth, fogHeight = 1280, 800

var fogHeadings = []int{0, 90, 180, 270}

func fogSnapshot(at time.Time, pr string) state.Snapshot {
	s := func(id, cwd string, st state.State) state.Session {
		return state.Session{ID: id, CWD: cwd, State: st, Alive: st != state.Resting, StartedAt: at.Add(-time.Hour), LastActivity: at}
	}
	a := s("a", "/p/alpha", state.Working)
	a.PRs = []claude.PR{{Number: 1, URL: "https://example.com/o/r/pull/1", Repository: "o/r", State: pr}}
	packed := s("c", "/p/gamma", state.Resting)
	packed.Packed = true
	return state.Snapshot{At: at, Sessions: []state.Session{a, s("b", "/p/beta", state.Resting), packed}}
}

func (e fogEllipse) contains(p city.Point) bool {
	dx, dy := (p.X-e.Centre.X)/(e.W/2), (p.Y-e.Centre.Y)/(e.H/2)
	return dx*dx+dy*dy <= 1
}

func (e fogEllipse) box() city.Rect {
	return city.Rect{Min: city.Point{X: e.Centre.X - e.W/2, Y: e.Centre.Y - e.H/2}, Max: city.Point{X: e.Centre.X + e.W/2, Y: e.Centre.Y + e.H/2}}
}

// screenBox is a world rectangle on screen, standing lift pixels tall.
func screenBox(cam *city.Camera, r city.Rect, lift float64) city.Rect {
	corners := []city.Point{r.Min, {X: r.Max.X, Y: r.Min.Y}, r.Max, {X: r.Min.X, Y: r.Max.Y}}
	s := cam.WorldToScreen(corners[0])
	box := city.Rect{Min: s, Max: s}
	for _, p := range corners[1:] {
		s := cam.WorldToScreen(p)
		box.Min.X, box.Min.Y = min(box.Min.X, s.X), min(box.Min.Y, s.Y)
		box.Max.X, box.Max.Y = max(box.Max.X, s.X), max(box.Max.Y, s.Y)
	}
	box.Min.Y -= lift
	return box
}

func fogMeets(a, b city.Rect) bool {
	return a.Min.X < b.Max.X && b.Min.X < a.Max.X && a.Min.Y < b.Max.Y && b.Min.Y < a.Max.Y
}

func TestFogInTheDrawing(t *testing.T) {
	k := testKits(t)
	g := &Game{kits: k}
	start := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	c := city.Build(fogSnapshot(start, claude.PROpen), city.NewLayout())
	b := c.Bridge()
	require.NotEmpty(t, b.Deck, "the bridge has no deck: nothing below can fail")
	fog := c.Fog()
	require.NotEmpty(t, fog, "no fog: nothing below can fail")
	var bank []city.FogPuff
	for _, p := range fog {
		if !p.Mist {
			bank = append(bank, p)
		}
	}
	var depot city.Rect
	for _, d := range c.Districts {
		if d.Depot {
			depot = d.Rect
		}
	}
	require.Positive(t, depot.Area(), "precondition: the city has a depot")
	head, mouth, width, ok := c.RiverBand()
	require.True(t, ok, "precondition: the city has a river")
	from, to, gaugeWidth, ok := c.GaugeRun(gaugeEndroom)
	require.True(t, ok, "precondition: the river has a run for the boats")
	var hazy []bridgePart
	for _, p := range bridgeParts(b) {
		if p.hazy() {
			hazy = append(hazy, p)
		}
	}
	require.NotEmpty(t, hazy, "no part of the deck is in the haze: nothing below can fail")
	out := math.Copysign(1, b.Far.X-b.Land.X)
	inHaze := []city.Point{
		{X: b.Haze.X + out*city.HazeCells*city.CellSize/3, Y: b.Land.Y},
		{X: b.Haze.X + out*city.HazeCells*city.CellSize*2/3, Y: b.Land.Y},
	}

	for _, heading := range fogHeadings {
		cam := city.NewCamera()
		cam.Projection, cam.Zoom, cam.Heading = city.Isometric, 1, heading
		top := g.deckLift(cam)

		t.Run(fmt.Sprintf("when the fog is drawn at heading %d", heading), func(t *testing.T) {
			got := namedOrder(cam, func(rec func(string) func()) []drawable {
				items := bridgeItems(cam, b, func(p bridgePart) func() {
					if p.hazy() {
						return rec(fmt.Sprintf("haze %v", p.Rect))
					}
					return func() {}
				})
				for i, at := range inHaze {
					items = append(items, deliveryAt(cam, b, at, rec(fmt.Sprintf("truck %d", i))))
				}
				fogged, ok := fogItem(cam, c, rec("fog"))
				require.True(t, ok, "precondition: the fog is in the sort")
				return append(items, fogged)
			})
			at := func(name string) int { return slices.Index(got, name) }

			t.Run("it should draw the fog after every haze piece of the deck and every truck in the haze", func(t *testing.T) {
				var over []string
				for _, name := range got {
					if name != "fog" && at(name) > at("fog") {
						over = append(over, name)
					}
				}
				assert.Empty(t, over)
			})

			t.Run("it should cover each haze piece's middle with a puff", func(t *testing.T) {
				ellipses := g.fogEllipses(cam, bank)
				var bare []city.Rect
				for _, p := range hazy {
					mid := raised(cam, p.Rect.Center(), top)
					if !slices.ContainsFunc(ellipses, func(e fogEllipse) bool { return e.contains(mid) }) {
						bare = append(bare, p.Rect)
					}
				}
				assert.Empty(t, bare)
			})

			t.Run("it should not cover the river's mouths, the boats' run or the depot", func(t *testing.T) {
				north, south := c.Island().Mouths()
				around := func(p city.Point) city.Rect {
					return city.Rect{Min: city.Point{X: p.X - width, Y: p.Y - width}, Max: city.Point{X: p.X + width, Y: p.Y + width}}
				}
				lifted := func(r city.Rect, lift float64) city.Rect {
					return screenBox(cam, r, lift)
				}
				keep := map[string]city.Rect{
					"the river's north mouth": lifted(around(north), 0),
					"the river's south mouth": lifted(around(south), 0),
					"the river's head":        lifted(around(head), 0),
					"the river's mouth":       lifted(around(mouth), 0),
					"the boats' run":          lifted(city.Rect{Min: city.Point{X: from.X - gaugeWidth/2, Y: from.Y}, Max: city.Point{X: to.X + gaugeWidth/2, Y: to.Y}}, top),
					"the depot":               lifted(depot, top),
				}
				var covered []string
				for _, e := range g.fogEllipses(cam, fog) {
					for name, r := range keep {
						if fogMeets(e.box(), r) {
							covered = append(covered, name)
						}
					}
				}
				slices.Sort(covered)
				assert.Empty(t, slices.Compact(covered))
			})
		})
	}

	t.Run("when the fog is drawn on its own", func(t *testing.T) {
		cam := city.NewCamera()
		cam.Projection, cam.Zoom = city.Isometric, 1
		screen := ebiten.NewImage(64, 64)
		defer screen.Deallocate()
		g := &Game{kits: k}
		g.drawFog(screen, cam, fog, false)

		t.Run("it should note no hover target of its own", func(t *testing.T) {
			assert.Empty(t, g.frameHits)
		})
	})

	t.Run("when the puff texture is made", func(t *testing.T) {
		g := &Game{kits: k}
		size := g.fogImage().Bounds().Size()

		t.Run("it should take at most 16 KiB", func(t *testing.T) {
			assert.LessOrEqual(t, 4*size.X*size.Y, 16<<10)
		})
	})

	for _, heading := range fogHeadings {
		t.Run(fmt.Sprintf("when a whole frame is drawn at heading %d with a truck in the fog", heading), func(t *testing.T) {
			hits, ellipses, trucks := fogFrame(t, start, heading)
			require.NotEmpty(t, ellipses, "precondition: the fog is on screen")
			require.Positive(t, trucks, "precondition: a truck is on screen in the fog")

			t.Run("it should find no hover target under the fog but a delivery truck on the bridge", func(t *testing.T) {
				var under []string
				for _, h := range hits {
					if h.hit.Delivery != nil {
						continue
					}
					for _, e := range ellipses {
						if fogMeets(h.rect, e.box()) {
							under = append(under, fmt.Sprintf("%+v at %v", h.hit, h.rect))
							break
						}
					}
				}
				assert.Empty(t, under)
			})
		})
	}
}

// fogFrame draws one whole frame centred on the haze at a heading, with
// a merged pull request's truck halfway into it, and returns the frame's
// hover targets, the fog's puffs on screen, and how many of the hover
// targets are trucks under the fog.
func fogFrame(t *testing.T, start time.Time, heading int) (hits []spriteHit, ellipses []fogEllipse, trucks int) {
	t.Helper()
	th := ui.NewTheme(1)
	f, err := newFaces(th)
	require.NoError(t, err)
	sp, err := shareArt()
	require.NoError(t, err)
	k, err := shareKits()
	require.NoError(t, err)
	clock := start
	s := city.NewScene(city.NewLayout())
	s.SetClock(func() time.Time { return clock }, time.UTC)
	s.SetProjection(city.Isometric)
	s.Resize(fogWidth, fogHeight)
	s.SetSnapshot(fogSnapshot(start, claude.PROpen))
	s.SetSnapshot(fogSnapshot(start, claude.PRMerged))
	trips := s.Deliveries()
	require.Len(t, trips, 1, "precondition: the merge sent a truck")
	b := s.City().Bridge()
	out := math.Copysign(1, b.Far.X-b.Land.X)
	mid := b.Haze.X + out*city.HazeCells*city.CellSize/2
	for at := time.Duration(0); at < trips[0].For(); at += 10 * time.Millisecond {
		if (trips[0].At(start.Add(at)).X-mid)*out >= 0 {
			clock = start.Add(at)
			break
		}
	}
	require.NotEqual(t, start, clock, "precondition: the truck reaches the middle of the haze")
	for s.Camera().Heading != heading {
		s.Turn()
	}
	s.CenterOn(city.Point{X: mid, Y: b.Land.Y})
	g := NewGame(s, &recordingActor{}, th, f, nil, sp)
	g.kits = k
	g.driven = true
	g.scripted, g.clicked = -1, -1
	w, h := g.LayoutF(fogWidth, fogHeight)
	screen := ebiten.NewImage(int(w), int(h))
	defer screen.Deallocate()
	g.ticks++
	g.Draw(screen)
	ellipses = g.fogEllipses(s.Camera(), s.City().Fog())
	for _, hit := range g.hits {
		if hit.hit.Delivery == nil {
			continue
		}
		for _, e := range ellipses {
			if fogMeets(hit.rect, e.box()) {
				trucks++
				break
			}
		}
	}
	return g.hits, ellipses, trucks
}
