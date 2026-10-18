package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutViewKey(t *testing.T) {
	th := ui.NewTheme(1)
	const width, height = 1200.0, 800.0
	k := ui.LayoutViewKey(th, width, height, city.LensConnections, measure7)

	t.Run("when the key is opened", func(t *testing.T) {
		t.Run("it should list the six lenses rather than the views", func(t *testing.T) {
			assert.Len(t, k.Rows, len(city.Lenses))
		})

		t.Run("it should number them the way the keyboard does, from one", func(t *testing.T) {
			assert.Equal(t, "1", k.Rows[0].Key)
			assert.Equal(t, "6", k.Rows[len(k.Rows)-1].Key)
		})

		t.Run("it should say what each lens shows, which is the reason for having it", func(t *testing.T) {
			for _, r := range k.Rows {
				require.NotEmpty(t, r.Question, r.Name)
			}
		})

		t.Run("it should mark the lens that is up", func(t *testing.T) {
			for _, r := range k.Rows {
				require.Equal(t, r.Lens == city.LensConnections, r.Current, r.Name)
			}
		})
	})

	t.Run("when it says what is on the river", func(t *testing.T) {
		t.Run("it should name the refresh key as a key to press, not a word in the sentence", func(t *testing.T) {
			assert.Contains(t, k.Note.Text, "the launch fetches fresh figures when you press "+city.RefreshKey)
		})
	})

	t.Run("when it names the two ways to see the whole city", func(t *testing.T) {
		planned := ui.LayoutViewKeyFor(th, width, height, city.LensConnections, "j", measure7)

		t.Run("it should name the City Plan's key as configured", func(t *testing.T) {
			assert.Equal(t, ui.ViewsNote("f", "j"), planned.Views.Text)
		})

		t.Run("it should hold the line inside the panel", func(t *testing.T) {
			_, h := measure7(planned.Views.Text, ui.Small)
			assert.LessOrEqual(t, planned.Views.At.Y+h, planned.Rect.Max.Y)
		})
	})

	t.Run("when the rows are laid out", func(t *testing.T) {
		t.Run("it should give each one a band wide enough to click", func(t *testing.T) {
			for _, r := range k.Rows {
				require.Greater(t, r.Row.Width(), 0.0, r.Name)
				require.Greater(t, r.Row.Height(), 0.0, r.Name)
			}
		})

		t.Run("it should not let two rows overlap", func(t *testing.T) {
			for i := 1; i < len(k.Rows); i++ {
				require.GreaterOrEqual(t, k.Rows[i].Row.Min.Y, k.Rows[i-1].Row.Max.Y, k.Rows[i].Name)
			}
		})

		t.Run("it should keep every row inside the panel", func(t *testing.T) {
			for _, r := range k.Rows {
				require.True(t, k.Rect.Contains(r.Row.Min) && k.Rect.Contains(r.Row.Max), r.Name)
			}
		})
	})

	t.Run("when a row is clicked", func(t *testing.T) {
		t.Run("it should give back that row's lens", func(t *testing.T) {
			want := k.Rows[3]
			got, ok := k.Hit(want.Row.Center())
			require.True(t, ok)
			assert.Equal(t, want.Lens, got)
		})
	})

	t.Run("when the click lands off the panel", func(t *testing.T) {
		t.Run("it should choose nothing", func(t *testing.T) {
			_, ok := k.Hit(city.Point{X: 5, Y: 5})
			assert.False(t, ok)
		})
	})

	t.Run("when the click lands on the panel but between rows", func(t *testing.T) {
		t.Run("it should choose nothing rather than the nearest", func(t *testing.T) {
			_, ok := k.Hit(city.Point{X: k.Rect.Min.X + 2, Y: k.Rect.Min.Y + 2})
			assert.False(t, ok)
		})
	})
}
