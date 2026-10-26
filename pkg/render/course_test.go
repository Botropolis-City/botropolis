package render

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 109, slice 4: each compaction slides a course under the
// building, in its own material, up to three drawn, with the count on
// the bottom course past that.
func TestCourses(t *testing.T) {
	cases := []struct{ compactions, want int }{{0, 0}, {1, 1}, {3, 3}, {4, 3}, {7, 3}}
	for _, c := range cases {
		t.Run(fmt.Sprintf("when a session has compacted %d times", c.compactions), func(t *testing.T) {
			t.Run(fmt.Sprintf("it should draw %d courses", c.want), func(t *testing.T) {
				assert.Equal(t, c.want, courseCount(c.compactions))
			})
		})
	}

	t.Run("when it has compacted more than three times", func(t *testing.T) {
		t.Run("it should show the count on the bottom course", func(t *testing.T) {
			assert.Equal(t, [3]string{"", "", "4"}, [3]string{coursePlaque(0), coursePlaque(3), coursePlaque(4)})
		})
	})

	t.Run("when a building stands on courses", func(t *testing.T) {
		g := kitGame(t)
		cam := city.NewCamera()
		ground := kindBuilding(city.ResidentialDistrict, 2)
		raised := kindBuilding(city.ResidentialDistrict, 2)
		raised.Session.Compactions = 3

		t.Run("it should stand that many course heights higher than it would on the ground", func(t *testing.T) {
			assert.InDelta(t, 3*courseHeight(cam.Zoom), g.footOf(cam, ground).Y-g.footOf(cam, raised).Y, 1e-9)
		})

		t.Run("it should keep its panes on its windows", func(t *testing.T) {
			low := g.facadePanes(cam, buildingPiece(ground), g.footOf(cam, ground))
			high := g.facadePanes(cam, buildingPiece(raised), g.footOf(cam, raised))
			require.NotEmpty(t, low)
			assert.InDelta(t, 3*courseHeight(cam.Zoom), low[0][0].Y-high[0][0].Y, 1e-9)
		})

		t.Run("it should lift the scaffold with it", func(t *testing.T) {
			a, _ := g.workFrame(cam, ground)
			b, _ := g.workFrame(cam, raised)
			assert.InDelta(t, 3*courseHeight(cam.Zoom), a.lot[0].Y-b.lot[0].Y, 1e-9)
		})
	})

	t.Run("when a compaction has just happened", func(t *testing.T) {
		now := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
		s := state.Session{Compactions: 2, LastCompactionAt: now.Add(-5 * time.Second)}

		t.Run("it should light the newest course", func(t *testing.T) {
			assert.True(t, courseLit(&s, now))
		})

		t.Run("and the light's time has passed", func(t *testing.T) {
			t.Run("it should be in the building's own material", func(t *testing.T) {
				assert.False(t, courseLit(&s, now.Add(courseGlow)))
			})
		})
	})
}

func TestCourseKinds(t *testing.T) {
	atlases, err := assets.LoadKits()
	require.NoError(t, err)
	atlas := atlases[len(atlases)-1]

	for _, kind := range city.DistrictTypes {
		for n := 1; n <= state.MaxStoreys; n++ {
			name := buildingPiece(&city.Building{Type: kind, Session: state.Session{Storeys: n}})
			t.Run("when "+name+" stands on courses", func(t *testing.T) {
				for k := 1; k <= maxCourses; k++ {
					t.Run(fmt.Sprintf("it should have course %d", k), func(t *testing.T) {
						_, ok := atlas.Sprite(coursePiece(name, k), 0)
						assert.True(t, ok)
					})
				}
			})
		}
	}
}

// The courses step in as they rise, like steps up to a door: the top one
// a step wider than the building on every side, each below a step wider
// again (Aria, 2026-10-18).
func TestCourseSteps(t *testing.T) {
	atlases, err := assets.LoadKits()
	require.NoError(t, err)
	atlas := atlases[len(atlases)-1]
	const step = 0.025

	for _, kind := range city.DistrictTypes {
		name := buildingPiece(&city.Building{Type: kind, Session: state.Session{Storeys: 3}})
		building, _ := atlas.Sprite(name, 0)
		t.Run("when "+name+" stands on courses", func(t *testing.T) {
			top, _ := atlas.Sprite(coursePiece(name, 1), 0)

			t.Run("it should make the top course one step wider than the building on every side", func(t *testing.T) {
				assert.InDelta(t, building.Body.W+2*step, top.Body.W, 1e-3)
			})

			for k := 2; k <= maxCourses; k++ {
				above, _ := atlas.Sprite(coursePiece(name, k-1), 0)
				below, _ := atlas.Sprite(coursePiece(name, k), 0)
				t.Run(fmt.Sprintf("it should make course %d one step wider than the one above", k), func(t *testing.T) {
					assert.InDelta(t, above.Body.W+2*step, below.Body.W, 1e-3)
				})
			}
		})
	}
}
