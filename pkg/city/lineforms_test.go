package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 348, slice 3: the three forms a group's line can take, drawn on
// one city for Aria to pick from.
func TestLineForms(t *testing.T) {
	other := func(id string) state.Session {
		s := session(id, mullet, state.Working)
		s.Group = "g1"
		return s
	}
	// a, b and c stand side by side in cinders, d and e in mullet.
	spread := inOrder(in("a", "g1"), in("b", "g1"), in("c", "g1"), loose("n"), other("d"), other("e"))
	shapeOf := func(t *testing.T, sessions []state.Session, form city.LineForm) (*city.City, city.LineShape) {
		t.Helper()
		c := city.Build(withGroups(sessions), city.NewLayout())
		require.Len(t, c.Lines, 1)
		return c, c.Shape(c.Lines[0], form)
	}
	centres := func(t *testing.T, c *city.City, ids ...string) map[city.Point]bool {
		t.Helper()
		out := map[city.Point]bool{}
		for _, id := range ids {
			out[buildingIn(t, c, id).Rect.Center()] = true
		}
		return out
	}
	districtOf := func(c *city.City, p city.Point) string {
		for _, d := range c.Districts {
			if d.Rect.Contains(p) {
				return d.Root
			}
		}
		return ""
	}

	t.Run("when the form is stations", func(t *testing.T) {
		_, shape := shapeOf(t, spread, city.FormStations)

		t.Run("it should give no segments, leaving today's line as it is", func(t *testing.T) {
			assert.Empty(t, shape.Links)
		})
	})

	t.Run("when the form is session to session", func(t *testing.T) {
		c, shape := shapeOf(t, spread, city.FormSessions)
		at := centres(t, c, "a", "b", "c", "d", "e")

		t.Run("it should draw one segment fewer than the group has buildings", func(t *testing.T) {
			assert.Len(t, shape.Links, 4)
		})

		t.Run("it should start and end every segment at a building's centre", func(t *testing.T) {
			for _, l := range shape.Links {
				assert.True(t, at[l[0]] && at[l[1]], "%v", l)
			}
		})

		t.Run("it should draw a segment inside a district", func(t *testing.T) {
			inside := 0
			for _, l := range shape.Links {
				if districtOf(c, l[0]) == districtOf(c, l[1]) {
					inside++
				}
			}
			assert.Positive(t, inside)
		})
	})

	t.Run("when the form is only between districts", func(t *testing.T) {
		c, shape := shapeOf(t, spread, city.FormDistricts)
		require.Len(t, shape.Links, 1)

		t.Run("it should join the nearest pair of their buildings", func(t *testing.T) {
			best, bestDist := [2]city.Point{}, -1.0
			for p := range centres(t, c, "a", "b", "c") {
				for q := range centres(t, c, "d", "e") {
					if d := p.Sub(q); bestDist < 0 || d.X*d.X+d.Y*d.Y < bestDist {
						best, bestDist = [2]city.Point{p, q}, d.X*d.X+d.Y*d.Y
					}
				}
			}
			assert.Equal(t, best, shape.Links[0])
		})
	})

	t.Run("when the form is neighbourhoods and links", func(t *testing.T) {
		c, shape := shapeOf(t, spread, city.FormNeighbourhoods)

		t.Run("it should outline each neighbourhood", func(t *testing.T) {
			assert.Len(t, shape.Outlines, 2)
		})

		t.Run("it should draw one link between the two districts", func(t *testing.T) {
			assert.Len(t, shape.Links, 1)
		})

		t.Run("it should start the link at the cinders block's centre", func(t *testing.T) {
			var cinders city.District
			for _, d := range c.Districts {
				if d.Root == cindersRoot() {
					cinders = *d
				}
			}
			hoods := c.Neighbourhoods(&cinders)
			require.Len(t, hoods, 1)
			assert.Equal(t, hoods[0].Centre(), shape.Links[0][0])
		})
	})

	t.Run("when a group's sessions are all in one district", func(t *testing.T) {
		alone := inOrder(in("a", "g1"), in("b", "g1"), in("c", "g1"), loose("n"))
		forms := []struct {
			name            string
			form            city.LineForm
			links, outlines int
		}{
			{"session to session", city.FormSessions, 2, 0},
			{"only between districts", city.FormDistricts, 0, 0},
			{"neighbourhoods and links", city.FormNeighbourhoods, 0, 1},
		}
		for _, f := range forms {
			t.Run("and the form is "+f.name, func(t *testing.T) {
				_, shape := shapeOf(t, alone, f.form)

				t.Run("it should draw what the form draws inside a district", func(t *testing.T) {
					assert.Equal(t, []int{f.links, f.outlines}, []int{len(shape.Links), len(shape.Outlines)})
				})
			})
		}
	})

	t.Run("when a site has no session, only the coordinator's subagents", func(t *testing.T) {
		coordinator := in("a", "g1")
		coordinator.Visits = map[string]int{cinders + "-elsewhere": 1, mullet: 1}
		sessions := inOrder(coordinator, in("b", "g1"), other("d"), session("z", cinders+"-elsewhere", state.Resting))
		for _, form := range []city.LineForm{city.FormSessions, city.FormDistricts, city.FormNeighbourhoods} {
			c, shape := shapeOf(t, sessions, form)
			var elsewhere *city.District
			for _, d := range c.Districts {
				if d.Root == cinders+"-elsewhere" {
					elsewhere = d
				}
			}
			require.NotNil(t, elsewhere)

			t.Run("it should draw no segment to it in form "+form.String(), func(t *testing.T) {
				for _, l := range shape.Links {
					assert.False(t, elsewhere.Rect.Contains(l[0]) || elsewhere.Rect.Contains(l[1]), "%v", l)
				}
			})
		}
	})
}

func cindersRoot() string { return cinders }
