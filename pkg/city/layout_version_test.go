package city_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/statedir"
)

func slotsOf(layout *city.Layout, root string) map[string]int {
	if layout == nil || layout.Districts[root] == nil {
		return nil
	}
	return layout.Districts[root].Slots
}

// Item 149: a layout of another version is migrated forward or refused
// by name, and never reset.
func TestLayoutVersions(t *testing.T) {
	write := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "layout.json")
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
		return path
	}
	saved := func(t *testing.T, layout *city.Layout) map[string]any {
		t.Helper()
		path := filepath.Join(t.TempDir(), "layout.json")
		require.NoError(t, layout.Save(path))
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		var fields map[string]any
		require.NoError(t, json.Unmarshal(data, &fields))
		return fields
	}

	t.Run("when the layout file has no version, from before versions", func(t *testing.T) {
		layout, err := city.LoadLayout(write(t, `{"districts":{"/p":{"slots":{"s1":3}}}}`))
		require.NoError(t, err)

		t.Run("it should keep its districts", func(t *testing.T) {
			assert.Equal(t, map[string]int{"s1": 3}, slotsOf(layout, "/p"))
		})
	})

	t.Run("when the layout file is at this build's version", func(t *testing.T) {
		t.Run("and it has no districts", func(t *testing.T) {
			layout, err := city.LoadLayout(write(t, `{"version":1,"starred":{"/p":true}}`))
			require.NoError(t, err)

			t.Run("it should keep its other fields rather than reset", func(t *testing.T) {
				assert.True(t, layout.IsStarred("/p"))
			})

			t.Run("it should give it an empty district map to fill", func(t *testing.T) {
				assert.NotNil(t, layout.Districts)
			})
		})
	})

	t.Run("when the layout file is from a newer version", func(t *testing.T) {
		t.Run("and its readableFrom is at or below this build's version", func(t *testing.T) {
			layout, err := city.LoadLayout(write(t, `{"version":7,"readableFrom":1,"districts":{"/p":{"slots":{"s1":3}}}}`))
			require.NoError(t, err)

			t.Run("it should load its districts", func(t *testing.T) {
				assert.Equal(t, map[string]int{"s1": 3}, slotsOf(layout, "/p"))
			})

			t.Run("and it is saved", func(t *testing.T) {
				fields := saved(t, layout)

				t.Run("it should keep the newer version", func(t *testing.T) {
					assert.Equal(t, float64(7), fields["version"])
				})

				t.Run("it should keep the newer readableFrom", func(t *testing.T) {
					assert.Equal(t, float64(1), fields["readableFrom"])
				})
			})
		})

		unreadable := []struct {
			scenario string
			body     string
		}{
			{"and its readableFrom is above this build's version", `{"version":7,"readableFrom":7,"districts":{}}`},
			{"and it has no readableFrom", `{"version":7,"districts":{}}`},
		}
		for _, c := range unreadable {
			t.Run(c.scenario, func(t *testing.T) {
				path := write(t, c.body)
				_, err := city.LoadLayout(path)
				require.Error(t, err)

				t.Run("it should be a newer-file error", func(t *testing.T) {
					assert.True(t, errors.Is(err, statedir.ErrNewer))
				})

				t.Run("it should name the file", func(t *testing.T) {
					assert.Contains(t, err.Error(), path)
				})

				t.Run("it should name both versions", func(t *testing.T) {
					assert.Contains(t, err.Error(), "version 7, which this build's version 1 cannot read")
				})

				t.Run("it should say to update Botropolis", func(t *testing.T) {
					assert.Contains(t, err.Error(), "update Botropolis")
				})
			})
		}
	})

	t.Run("when a new layout is saved", func(t *testing.T) {
		fields := saved(t, city.NewLayout())
		for key, want := range map[string]float64{"version": 1, "readableFrom": 1} {
			t.Run("it should write "+key, func(t *testing.T) {
				assert.Equal(t, want, fields[key])
			})
		}
	})
}
