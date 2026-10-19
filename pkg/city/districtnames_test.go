package city_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
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

func TestBreakdownNamesDistricts(t *testing.T) {
	twin := "/home/avesta/workspaces/github/botropolis-city/botropolis"
	sessions := []state.Session{
		spender("a", botropolis, "claude-opus-5", map[int]int64{0: 100}),
		spender("x", "", "claude-opus-5", map[int]int64{0: 50}),
	}

	t.Run("when the store splits by an empty root and two roots with one base name", func(t *testing.T) {
		window := stats.Window{ByProject: []stats.Slice{
			{Key: botropolis, Cost: cost(1)},
			{Key: twin, Cost: cost(2)},
			{Key: "", Cost: cost(3)},
		}}
		c := city.BuildWithLedger(snapshot(sessions...), city.NewLayout(), &fakeLedger{window: window})
		b := c.Breakdown(city.LastDay)

		t.Run("it should keep each a share of its own", func(t *testing.T) {
			assert.Len(t, b.ByProject, 3)
		})

		for i, want := range []string{"auroq/botropolis", "botropolis-city/botropolis", city.FolderUnknown} {
			t.Run("it should name share "+want, func(t *testing.T) {
				assert.Equal(t, want, b.ByProject[i].Key)
			})
		}
	})

	t.Run("when the store cannot be read and a session has no folder", func(t *testing.T) {
		c := city.BuildWithLedger(snapshot(sessions...), city.NewLayout(), &fakeLedger{reason: "off"})
		b := c.Breakdown(city.LastDay)

		t.Run("it should still count it in the total", func(t *testing.T) {
			assert.Equal(t, int64(150), b.Counters.Output)
		})

		t.Run("it should list it as "+city.FolderUnknown, func(t *testing.T) {
			var keys []string
			for _, s := range b.ByProject {
				keys = append(keys, s.Key)
			}
			assert.Contains(t, keys, city.FolderUnknown)
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
