package render

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 99: the landmarks' names are painted on their walls, and the
// landmarks are turned so their front doors face the home heading.

var landmarkKits = []struct{ name, kit string }{
	{"power plant", kitPlant},
	{"library", kitLibrary},
	{"city hall", kitHall},
}

// quadsOverlap reports whether two convex quads share any area, by the
// separating axis test.
func quadsOverlap(a, b [4]city.Point) bool {
	for _, poly := range [][4]city.Point{a, b} {
		for i := range poly {
			p, q := poly[i], poly[(i+1)%4]
			axis := city.Point{X: q.Y - p.Y, Y: p.X - q.X}
			project := func(s [4]city.Point) (float64, float64) {
				lo, hi := math.Inf(1), math.Inf(-1)
				for _, v := range s {
					d := v.X*axis.X + v.Y*axis.Y
					lo, hi = math.Min(lo, d), math.Max(hi, d)
				}
				return lo, hi
			}
			alo, ahi := project(a)
			blo, bhi := project(b)
			if ahi <= blo+1e-6 || bhi <= alo+1e-6 {
				return false
			}
		}
	}
	return true
}

// landmarkArtHash is a hash of every pixel of a piece's sprites, in every
// atlas at every heading: what its walls were measured from.
func landmarkArtHash(t *testing.T, kit string) string {
	t.Helper()
	loaded, err := assets.LoadKits()
	require.NoError(t, err)
	sort.Slice(loaded, func(i, j int) bool { return loaded[i].Zoom < loaded[j].Zoom })
	h := sha256.New()
	for _, a := range loaded {
		for _, heading := range assets.Headings {
			s, ok := a.Sprite(kit, heading)
			require.True(t, ok, kit)
			page := a.Pages[s.Page]
			for y := s.Rect.Min.Y; y < s.Rect.Max.Y; y++ {
				for x := s.Rect.Min.X; x < s.Rect.Max.X; x++ {
					r, g, b, al := page.At(x, y).RGBA()
					fmt.Fprintf(h, "%d,%d,%d,%d;", r>>8, g>>8, b>>8, al>>8)
				}
			}
			fmt.Fprintf(h, "|%v|", image.Pt(s.Rect.Dx(), s.Rect.Dy()))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func TestLandmarkWallsAreMeasuredFromTheArtInTheAtlas(t *testing.T) {
	for _, l := range landmarkKits {
		t.Run(fmt.Sprintf("when the %s's walls are read", l.name), func(t *testing.T) {
			t.Run("it should have been measured from the pixels the atlas holds now, or be measured again with tools/landmark-walls", func(t *testing.T) {
				assert.Equal(t, landmarkTable[l.kit].Art, landmarkArtHash(t, l.kit))
			})
		})
	}
}

func TestLandmarkLettering(t *testing.T) {
	g := landmarkLabelGame(t)
	lot := plantFixture()

	for _, l := range landmarkKits {
		for _, heading := range []int{0, 90, 180, 270} {
			for _, zoom := range []float64{1, 2} {
				t.Run(fmt.Sprintf("when the %s's name is lettered with the camera facing %d at zoom %g", l.name, heading, zoom), func(t *testing.T) {
					cam := isoCamera(heading, zoom)
					view, ok := g.landmarkView(cam, lot, l.kit)
					require.True(t, ok, "precondition: the landmark is drawn")
					sign, ok := g.landmarkLettering(cam, lot, l.kit, l.name)
					require.True(t, ok, "precondition: there is a wall to letter")

					t.Run("it should lie inside a wall face the camera sees", func(t *testing.T) {
						b := sign.Box
						assert.True(t, b[0] >= 0 && b[1] <= 1 && b[2] >= 0 && b[3] <= 1 && b[0] < b[1] && b[2] < b[3], "box %v", b)
					})

					t.Run("it should cover none of the landmark's doors", func(t *testing.T) {
						var hit []wallBox
						for _, o := range view.walls.Openings[sign.Side] {
							if sign.Box.overlaps(o) {
								hit = append(hit, o)
							}
						}
						assert.Empty(t, hit)
					})

					t.Run("it should cover none of its windows", func(t *testing.T) {
						var hit [][4]city.Point
						for _, p := range view.panes {
							if quadsOverlap(sign.Quad, p) {
								hit = append(hit, p)
							}
						}
						assert.Empty(t, hit)
					})
				})
			}
		}
	}
}

func TestLandmarksFaceTheDefaultCamera(t *testing.T) {
	g := landmarkLabelGame(t)
	lot := plantFixture()

	for _, l := range landmarkKits {
		t.Run(fmt.Sprintf("when the camera faces 0 and the %s is drawn", l.name), func(t *testing.T) {
			view, ok := g.landmarkView(isoCamera(0, 1), lot, l.kit)
			require.True(t, ok, "precondition: the landmark is drawn")

			t.Run("it should show the side with its door", func(t *testing.T) {
				assert.NotEmpty(t, view.walls.Entrance)
			})
		})
	}
}

func TestPlantChimneyTurnsWithThePlant(t *testing.T) {
	plant := plantFixture()
	side := footprintSide(plantSpriteW, atlasZoom)
	perch := landmarkPerch(plant, side)

	t.Run("when the plant is drawn turned", func(t *testing.T) {
		t.Run("it should stand the chimney on the slab", func(t *testing.T) {
			assert.True(t, slabUnder(plant).Contains(perch))
		})

		t.Run("it should stand it on the opposite side of the plot's centre from the unturned perch", func(t *testing.T) {
			unturned := plantStackPerch(plant, side)
			c := plant.Center()
			assert.Less(t, (perch.X-c.X)*(unturned.X-c.X), 0.0)
		})
	})
}

// screenQuad is a world rect's footprint on screen.
func screenQuad(cam *city.Camera, r city.Rect) [4]city.Point {
	return [4]city.Point{
		cam.WorldToScreen(r.Min),
		cam.WorldToScreen(city.Point{X: r.Max.X, Y: r.Min.Y}),
		cam.WorldToScreen(r.Max),
		cam.WorldToScreen(city.Point{X: r.Min.X, Y: r.Max.Y}),
	}
}

// base is the ground the drawn landmark stands on, from its measured
// walls: their ground corners, and the back corner they imply.
func (v landmarkView) base() [4]city.Point {
	left, near, right := v.at("left", 0, 0), v.at("left", 1, 0), v.at("right", 1, 0)
	back := city.Point{X: left.X + right.X - near.X, Y: left.Y + right.Y - near.Y}
	return [4]city.Point{left, near, right, back}
}

func TestPlazaFixturesClearTheTurnedLandmarks(t *testing.T) {
	g := landmarkLabelGame(t)
	scene := city.NewScene(city.NewLayout())
	scene.Resize(1600, 1000)
	scene.SetSnapshot(state.Snapshot{Sessions: []state.Session{
		{ID: "a", CWD: "/p/a", State: state.Working},
		{ID: "b", CWD: "/p/b", State: state.Waiting},
	}})
	c := scene.City()
	require.Greater(t, c.Fountain.Area(), 0.0, "precondition: the plaza has its fountain")
	fixtures := map[string]city.Rect{"fountain": c.Fountain}
	for i, l := range c.Lamps {
		fixtures[fmt.Sprintf("lamp %d", i)] = l.Rect()
	}
	lots := map[string]struct {
		lot city.Rect
		kit string
	}{
		"power plant": {c.Plant.Rect, kitPlant},
		"library":     {c.Library.Rect, kitLibrary},
		"city hall":   {c.Hall.Rect, kitHall},
	}

	for _, heading := range []int{0, 90, 180, 270} {
		cam := isoCamera(heading, 1)
		for name, l := range lots {
			if l.lot.Area() == 0 {
				continue
			}
			view, ok := g.landmarkView(cam, l.lot, l.kit)
			require.True(t, ok, "precondition: the %s is drawn", name)
			t.Run(fmt.Sprintf("when the camera faces %d and the %s is turned", heading, name), func(t *testing.T) {
				t.Run("it should stand clear of every plaza fixture", func(t *testing.T) {
					var under []string
					for f, r := range fixtures {
						if quadsOverlap(view.base(), screenQuad(cam, r)) {
							under = append(under, f)
						}
					}
					assert.Empty(t, under)
				})
			})
		}
	}
}

func TestLandmarkPlate(t *testing.T) {
	g := landmarkLabelGame(t)
	g.scene = city.NewScene(city.NewLayout())
	c := &city.City{}
	c.Library.Rect = plantFixture()
	cam := isoCamera(0, 1)

	t.Run("when no landmark is hovered", func(t *testing.T) {
		g.mapLabels = g.mapLabels[:0]
		g.queueLandmarkLabels(cam, c, city.Hit{})

		t.Run("it should queue no landmark plate", func(t *testing.T) {
			assert.Empty(t, g.mapLabels)
		})
	})

	t.Run("when the library is hovered", func(t *testing.T) {
		g.mapLabels = g.mapLabels[:0]
		g.queueLandmarkLabels(cam, c, city.Hit{Landmark: city.LandmarkLibrary})

		t.Run("it should queue its plate on its roof", func(t *testing.T) {
			require.Len(t, g.mapLabels, 1)
			d := g.mapLabels[0].Rect.Center().Sub(g.landmarkRoof(cam, c.Library.Rect, kitLibrary))
			assert.LessOrEqual(t, math.Hypot(d.X, d.Y), g.theme.Grid())
		})
	})
}
