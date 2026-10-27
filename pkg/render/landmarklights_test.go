package render

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 185: the landmarks light their own windows at night, as a
// session's building does (item 166): the library and City Hall all
// night, the power plant while any session is working.
func TestLandmarkLights(t *testing.T) {
	landmarks := []struct {
		name, kit string
	}{
		{"the library", kitLibrary},
		{"city hall", kitHall},
		{"the clinic", kitClinic},
		{"the power plant", kitPlant},
	}

	for _, c := range []struct {
		when  string
		night bool
		plant float64
		want  map[string]float64
	}{
		{"it is night and a session is working", true, 1, map[string]float64{kitLibrary: 1, kitHall: 1, kitClinic: 1, kitPlant: 1}},
		{"it is night and nobody is working", true, 0, map[string]float64{kitLibrary: 1, kitHall: 1, kitClinic: 1, kitPlant: 0}},
		{"it is night and the plant's lights are fading in", true, 0.4, map[string]float64{kitLibrary: 1, kitHall: 1, kitClinic: 1, kitPlant: 0.4}},
		{"it is day and a session is working", false, 1, map[string]float64{kitLibrary: 0, kitHall: 0, kitClinic: 0, kitPlant: 0}},
	} {
		t.Run("when "+c.when, func(t *testing.T) {
			for _, l := range landmarks {
				t.Run("and "+l.name+" stands", func(t *testing.T) {
					on := landmarkOn(&city.City{Night: c.night}, l.kit, c.plant)

					t.Run(fmt.Sprintf("it should have its lights %g on", c.want[l.kit]), func(t *testing.T) {
						assert.Equal(t, c.want[l.kit], on)
					})
				})
			}
		})
	}

	t.Run("when the light is forced to night at noon", func(t *testing.T) {
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC) }, time.UTC)
		require.False(t, s.City().Night)
		s.CycleLight()
		require.Equal(t, city.LightNight, s.Light())

		t.Run("it should light the library", func(t *testing.T) {
			assert.Equal(t, 1.0, landmarkOn(s.City(), kitLibrary, 0))
		})
	})

	g := kitGame(t)
	lot := plantFixture()
	for _, l := range landmarks {
		for _, heading := range assets.Headings {
			for _, zoom := range []float64{1, 2} {
				cam := isoCamera(heading, zoom)
				atlas := g.kits.pick(cam.Zoom)
				sprite, ok := atlas.Sprite(l.kit, cam.Heading-landmarkTurn)
				require.True(t, ok)
				t.Run(fmt.Sprintf("when %s is lit, seen from %d at zoom %g", l.name, heading, zoom), func(t *testing.T) {
					vertices, indices, page := g.landmarkTriangles(cam, lot, l.kit, 1)

					if len(sprite.Panes) == 0 {
						t.Run("it should light nothing, having no windows on this side", func(t *testing.T) {
							assert.Nil(t, page)
						})
						return
					}

					t.Run("it should light every one of its panes", func(t *testing.T) {
						assert.Len(t, indices, 6*len(sprite.Panes))
					})

					t.Run("it should light them from its own sprite's page", func(t *testing.T) {
						assert.Same(t, atlas.pages[sprite.Page], page)
					})

					t.Run("it should put each pane where the sprite's own pane is drawn", func(t *testing.T) {
						_, origin, scale, _, _ := g.kitPlace(cam, l.kit, landmarkTurn, lot.Center(), 0)
						q := sprite.Panes[0][0]
						assert.InDeltaSlice(t,
							[]float64{origin.X + float64(q.X)*scale, origin.Y + float64(q.Y)*scale},
							[]float64{float64(vertices[0].DstX), float64(vertices[0].DstY)}, 1e-3)
					})
				})
			}
		}
	}

	t.Run("when the plant's lights are fading in", func(t *testing.T) {
		vertices, _, _ := g.landmarkTriangles(isoCamera(0, 1), lot, kitPlant, 0.4)
		require.NotEmpty(t, vertices)

		t.Run("it should draw them at that strength", func(t *testing.T) {
			assert.InDelta(t, 0.4, vertices[0].ColorA, 1e-6)
		})
	})

	t.Run("when a landmark is lit at night", func(t *testing.T) {
		g.beginLights(64, 64)
		g.lightLandmark(isoCamera(0, 1), lot, kitHall, 1)
		g.lightsOn = false

		t.Run("it should draw its windows in a single call", func(t *testing.T) {
			assert.Equal(t, 1, g.windowDraws)
		})
	})

	t.Run("when a landmark's lights are off", func(t *testing.T) {
		g.landmarkDraws = 0
		g.beginLights(64, 64)
		g.lightLandmark(isoCamera(0, 1), lot, kitPlant, 0)
		g.lightsOn = false

		t.Run("it should draw nothing", func(t *testing.T) {
			assert.Zero(t, g.windowDraws)
		})
	})
}

// Item 185: the plant's round glow went at night (Aria, 2026-10-20);
// its spend stays on the strip's "$… 24h" chip and in the Cost lens.
func TestThePlantHasNoGlow(t *testing.T) {
	t.Run("when it is night and the plant stands", func(t *testing.T) {
		now := time.Date(2026, 10, 12, 22, 30, 0, 0, time.Local)
		d, err := NewDrive(DriveOptions{Home: helpers.ParkedWeek(t, 20, now, "The plant at night"), Now: now})
		require.NoError(t, err)
		require.NoError(t, d.Run(10))
		c := d.game.scene.City()
		require.Positive(t, c.Plant.Rect.Area())
		cam := d.game.scene.Camera()
		_, _, _, plant, ok := d.game.kitPlace(cam, kitPlant, landmarkTurn, c.Plant.Rect.Center(), 0)
		require.True(t, ok)
		var on []city.Point
		for _, p := range d.Shown().Glows {
			if plant.Contains(p) {
				on = append(on, p)
			}
		}

		t.Run("it should draw no glow on the plant", func(t *testing.T) {
			assert.Empty(t, on)
		})
	})
}

// Item 185: the map itself lights the landmarks, not only the helper.
func TestLandmarksLitOnTheMap(t *testing.T) {
	for _, c := range []struct {
		when string
		hour int
		want int
	}{
		{"it is day", 14, 0},
		{"it is night and nobody is working, with statistics off, so no City Hall stands", 22, 2},
	} {
		t.Run("when "+c.when, func(t *testing.T) {
			now := time.Date(2026, 10, 12, c.hour, 30, 0, 0, time.Local)
			d, err := NewDrive(DriveOptions{Home: helpers.ParkedWeek(t, 20, now, "Landmarks at night"), Now: now})
			require.NoError(t, err)
			require.NoError(t, d.Run(10))

			t.Run(fmt.Sprintf("it should light %d of the landmarks: the library and the clinic at night, and not the plant", c.want), func(t *testing.T) {
				assert.Equal(t, c.want, d.Shown().LandmarkDraws)
			})
		})
	}
}
