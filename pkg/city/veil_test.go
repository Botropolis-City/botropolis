package city_test

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 168: in sharing mode the city wears neutral names.

func veiledDistrictNames(c *city.City) []string {
	var names []string
	for _, d := range c.Districts {
		names = append(names, d.Name)
	}
	slices.Sort(names)
	return names
}

func buildingTitles(c *city.City) map[string]string {
	titles := map[string]string{}
	for _, b := range c.Buildings() {
		titles[b.Session.ID] = b.Card(c.Time).Title
	}
	return titles
}

func towerNames(c *city.City) []string {
	var names []string
	for _, t := range c.Towers {
		names = append(names, t.Server.Name)
	}
	return names
}

func TestVeil(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

	t.Run("when a city whose every name is a unique string is veiled", func(t *testing.T) {
		scene := city.NewScene(city.NewLayout())
		scene.Resize(1280, 800)
		scene.SetSnapshot(helpers.ShareSnapshot(now))
		scene.SetVeil(city.NewVeil())
		c := scene.City()
		titles := buildingTitles(c)

		t.Run("it should name each district District A, B", func(t *testing.T) {
			assert.Equal(t, []string{"District A", "District B"}, veiledDistrictNames(c))
		})

		t.Run("it should title each building Session N", func(t *testing.T) {
			assert.ElementsMatch(t, []string{"Session 1", "Session 2", "Session 3"}, slices.Collect(maps.Values(titles)))
		})

		t.Run("it should name each tower Tower N", func(t *testing.T) {
			assert.Equal(t, []string{"Tower 1"}, towerNames(c))
		})

		t.Run("it should say it is veiled", func(t *testing.T) {
			assert.True(t, scene.Veiled())
		})

		t.Run("and a second snapshot arrives", func(t *testing.T) {
			names := map[string]string{}
			for _, d := range c.Districts {
				names[d.Root] = d.Name
			}
			scene.SetSnapshot(helpers.ShareSnapshot(now.Add(time.Second)))
			again := map[string]string{}
			for _, d := range scene.City().Districts {
				again[d.Root] = d.Name
			}

			t.Run("it should keep every district's alias", func(t *testing.T) {
				assert.Equal(t, names, again)
			})

			t.Run("it should keep every building's alias", func(t *testing.T) {
				assert.Equal(t, titles, buildingTitles(scene.City()))
			})
		})

		t.Run("and a new district appears", func(t *testing.T) {
			snap := helpers.ShareSnapshot(now.Add(2 * time.Second))
			snap.Sessions = append(snap.Sessions, state.Session{
				ID: "dddd-4", Title: "Qzdeltatitle", CWD: helpers.ShareHome + "/Qzdeltaproject", State: state.Working, Alive: true,
				StartedAt: now, LastActivity: now,
			})
			scene.SetSnapshot(snap)

			t.Run("it should give it the next letter", func(t *testing.T) {
				assert.Equal(t, "District C", scene.City().DistrictName(helpers.ShareHome+"/Qzdeltaproject"))
			})
		})

		t.Run("and the veil is lifted", func(t *testing.T) {
			scene.SetVeil(nil)
			lifted := scene.City()

			t.Run("it should restore every district's name", func(t *testing.T) {
				assert.Contains(t, veiledDistrictNames(lifted), "Qzalphaproject")
			})

			t.Run("it should restore every building's title", func(t *testing.T) {
				assert.Equal(t, "Qzalphatitle", buildingTitles(lifted)["aaaa-1"])
			})

			t.Run("it should say it is not veiled", func(t *testing.T) {
				assert.False(t, scene.Veiled())
			})
		})
	})

	t.Run("when a snapshot is veiled", func(t *testing.T) {
		real := helpers.ShareSnapshot(now)
		veiled := city.NewVeil().Snapshot(real)

		t.Run("it should leave no secret in any word it is told to veil", func(t *testing.T) {
			var leaks []string
			walkStrings(reflect.ValueOf(veiled), "Snapshot", func(path, s string) {
				if kind := city.VeilTable[path]; kind != city.VeilKeep && kind != city.VeilPath && strings.Contains(s, "Qz") {
					leaks = append(leaks, path+"="+s)
				}
			})
			assert.Empty(t, leaks)
		})

		t.Run("it should keep every session's id, so the layout still finds it", func(t *testing.T) {
			assert.Equal(t, real.Sessions[0].ID, veiled.Sessions[0].ID)
		})

		t.Run("it should keep every session's folder, so its district stands where it stood", func(t *testing.T) {
			assert.Equal(t, real.Sessions[0].CWD, veiled.Sessions[0].CWD)
		})

		t.Run("it should leave the snapshot it was given untouched", func(t *testing.T) {
			assert.Equal(t, helpers.ShareSnapshot(now), real)
		})

		t.Run("it should keep a session's calls to a server tied to that server", func(t *testing.T) {
			require.Len(t, veiled.Servers, 1)
			assert.Equal(t, 4, veiled.Sessions[0].MCPCalls[veiled.Servers[0].Name])
		})
	})

	// Item 169: the scene's own rebuilds re-applied the snapshot it
	// showed, which while veiled is the veiled one, and kept that as
	// the snapshot as it came.
	t.Run("when a veiled city is rebuilt from inside the scene", func(t *testing.T) {
		scene := city.NewScene(city.NewLayout())
		scene.Resize(1280, 800)
		scene.SetSnapshot(helpers.ShareSnapshot(now))
		scene.SetVeil(city.NewVeil())
		scene.SetLedger(&fakeLedger{})

		t.Run("and the veil is lifted", func(t *testing.T) {
			scene.SetVeil(nil)

			t.Run("it should restore every building's title", func(t *testing.T) {
				assert.Equal(t, "Qzalphatitle", buildingTitles(scene.City())["aaaa-1"])
			})
		})
	})
}

// TestVeilAccountsForEveryWord fails on any string a snapshot can carry
// that the veil's table does not say whether to keep or to veil: a new
// field is a new way to leak a name, and the table has to be told.
func TestVeilAccountsForEveryWord(t *testing.T) {
	var missing []string
	walkTypes(reflect.TypeOf(state.Snapshot{}), "Snapshot", map[reflect.Type]bool{}, func(path string) {
		if _, ok := city.VeilTable[path]; !ok {
			missing = append(missing, path)
		}
	})

	t.Run("when every string a snapshot can carry is listed", func(t *testing.T) {
		t.Run("it should find each in the veil's table", func(t *testing.T) {
			assert.Empty(t, missing)
		})
	})
}

var timeType = reflect.TypeOf(time.Time{})

func walkTypes(tp reflect.Type, path string, seen map[reflect.Type]bool, visit func(string)) {
	switch tp.Kind() {
	case reflect.String:
		visit(path)
	case reflect.Pointer, reflect.Slice, reflect.Array:
		walkTypes(tp.Elem(), path, seen, visit)
	case reflect.Map:
		walkTypes(tp.Key(), path+"{key}", seen, visit)
		walkTypes(tp.Elem(), path, seen, visit)
	case reflect.Struct:
		if tp == timeType || seen[tp] {
			return
		}
		seen[tp] = true
		for i := range tp.NumField() {
			f := tp.Field(i)
			walkTypes(f.Type, tp.Name()+"."+f.Name, seen, visit)
		}
	}
}

func walkStrings(v reflect.Value, path string, visit func(path, s string)) {
	switch v.Kind() {
	case reflect.String:
		visit(path, v.String())
	case reflect.Pointer:
		if !v.IsNil() {
			walkStrings(v.Elem(), path, visit)
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			walkStrings(v.Index(i), path, visit)
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			walkStrings(iter.Key(), path+"{key}", visit)
			walkStrings(iter.Value(), path, visit)
		}
	case reflect.Struct:
		if v.Type() == timeType {
			return
		}
		for i := range v.NumField() {
			walkStrings(v.Field(i), v.Type().Name()+"."+v.Type().Field(i).Name, visit)
		}
	}
}
