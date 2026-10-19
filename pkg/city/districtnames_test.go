package city_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

func TestDistrictNames(t *testing.T) {
	t.Run("when a root's base name is unique", func(t *testing.T) {
		names := city.DistrictNames([]string{cinders, botropolis})

		t.Run("it should be the base name", func(t *testing.T) {
			assert.Equal(t, "cinders", names[cinders])
		})
	})

	t.Run("when two roots share a base name", func(t *testing.T) {
		moved := "/home/avesta/workspaces/github/botropolis-city/botropolis"
		old := "/home/avesta/workspaces/github/botropolis"
		names := city.DistrictNames([]string{moved, old, cinders})

		for root, want := range map[string]string{old: "github/botropolis", moved: "botropolis-city/botropolis"} {
			t.Run("it should label "+root+" "+want, func(t *testing.T) {
				assert.Equal(t, want, names[root])
			})
		}

		t.Run("and they also share a parent's name", func(t *testing.T) {
			names := city.DistrictNames([]string{"/a/x/p", "/b/x/p"})

			for root, want := range map[string]string{"/a/x/p": "a/x/p", "/b/x/p": "b/x/p"} {
				t.Run("it should go up until they differ: "+want, func(t *testing.T) {
					assert.Equal(t, want, names[root])
				})
			}
		})

		t.Run("and one is a suffix of the other", func(t *testing.T) {
			names := city.DistrictNames([]string{"/x/p", "/y/x/p"})

			t.Run("it should still give them different labels", func(t *testing.T) {
				assert.NotEqual(t, names["/x/p"], names["/y/x/p"])
			})
		})
	})

	t.Run("when the root is empty", func(t *testing.T) {
		names := city.DistrictNames([]string{""})

		t.Run("it should be "+city.FolderUnknown, func(t *testing.T) {
			assert.Equal(t, city.FolderUnknown, names[""])
		})
	})
}

func TestBuildNamesDistricts(t *testing.T) {
	t.Run("when a session has an empty cwd", func(t *testing.T) {
		packed := session("p", "", state.Resting)
		packed.Packed = true
		c := build(t, city.NewLayout(), session("a", cinders, state.Working), session("x", "", state.Resting), packed)

		t.Run("it should make no district named \".\"", func(t *testing.T) {
			assert.NotContains(t, districtNames(c), ".")
		})

		t.Run("it should make no building for it", func(t *testing.T) {
			assert.Len(t, c.Buildings(), 1)
		})

		t.Run("and it is packed, it should make no depot", func(t *testing.T) {
			assert.NotContains(t, districtNames(c), city.DepotName)
		})
	})

	t.Run("when two sessions' roots share a base name", func(t *testing.T) {
		twin := "/home/avesta/workspaces/github/botropolis-city/botropolis"
		layout := city.NewLayout()
		c := build(t, layout, session("a", botropolis, state.Working), session("b", twin, state.Working))
		names := districtNames(c)

		t.Run("each district's name should differ", func(t *testing.T) {
			assert.NotEqual(t, names[0], names[1])
		})

		for _, name := range names {
			t.Run("it should end "+name+" in the shared name", func(t *testing.T) {
				assert.True(t, strings.HasSuffix(name, "/botropolis"))
			})
		}

		t.Run("each sidebar row's name should differ", func(t *testing.T) {
			rows := c.Projects(layout)
			assert.NotEqual(t, rows[0].Name, rows[1].Name)
		})

		for id, want := range map[string]bool{"a": false, "b": true} {
			t.Run("and the search names the twin's parent, it should decide for "+id, func(t *testing.T) {
				var got bool
				for _, b := range c.Buildings() {
					if b.Session.ID == id {
						got = city.ParseFilter("botropolis-city").Matches(b)
					}
				}
				assert.Equal(t, want, got)
			})
		}

		t.Run("and one's card should say which", func(t *testing.T) {
			for _, b := range c.Buildings() {
				if b.Session.ID == "b" {
					assert.Equal(t, "session in botropolis-city/botropolis", b.Card(now).Kind)
				}
			}
		})

		t.Run("and one is hidden", func(t *testing.T) {
			layout := city.NewLayout()
			layout.SetHidden(twin, true)
			c := build(t, layout, session("a", botropolis, state.Working), session("b", twin, state.Working))

			t.Run("it should still name the shown one apart from it", func(t *testing.T) {
				assert.Equal(t, "auroq/botropolis", districtNames(c)[0])
			})

			t.Run("it should name the hidden one apart too", func(t *testing.T) {
				assert.Equal(t, "botropolis-city/botropolis", c.DistrictName(twin))
			})
		})
	})
}

func districtNames(c *city.City) []string {
	var names []string
	for _, d := range c.Districts {
		names = append(names, d.Name)
	}
	return names
}
