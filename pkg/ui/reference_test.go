package ui_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func TestReference(t *testing.T) {
	th := ui.NewTheme(1)
	keys := []ui.Key{{Key: "tab", Action: "next needs-you"}, {Key: "v", Action: "lenses"}}
	ref := ui.NewReference(keys, "")

	t.Run("it should have an entry for every kind", func(t *testing.T) {
		for _, k := range city.Kinds {
			_, ok := ref.Entry(k.Name())
			require.True(t, ok, k.Name())
		}
	})

	t.Run("it should have an entry for the keys", func(t *testing.T) {
		e, ok := ref.Entry(ui.KeysEntry)
		require.True(t, ok)
		assert.Contains(t, e.Lines, "tab — next needs-you")
	})

	t.Run("when a term is clicked", func(t *testing.T) {
		ref := ui.NewReference(keys, "")
		ref.Open(city.KindSession.Name())
		laid := ui.LayoutReference(th, 1200, 800, ref, measure7)
		var drone ui.Button
		for _, b := range laid.Terms {
			if b.Label.Text == city.KindDrone.Name() {
				drone = b
			}
		}
		require.NotZero(t, drone.Rect.Area(), "the session entry should link to drone")
		term, ok := laid.Hit(drone.Rect.Center())
		require.True(t, ok)
		ref.Open(term)

		t.Run("it should open that term's entry", func(t *testing.T) {
			assert.Equal(t, city.KindDrone.Name(), ref.Current().ID)
		})

		t.Run("and back is pressed", func(t *testing.T) {
			ref.Back()

			t.Run("it should return to the entry before", func(t *testing.T) {
				assert.Equal(t, city.KindSession.Name(), ref.Current().ID)
			})
		})
	})
}

func TestLayoutReferenceCredit(t *testing.T) {
	keys := []ui.Key{{Key: "tab", Action: "next needs-you"}}
	credit := strings.Repeat("Kenney ", 100)

	for _, scale := range []float64{1, 2} {
		th := ui.NewTheme(scale)
		grid := th.Grid()
		pad := 3 * grid
		bare := ui.LayoutReference(th, 2000, 4000, ui.NewReference(keys, ""), measure7)

		t.Run(fmt.Sprintf("when the index is laid out with a credit at scale %v", scale), func(t *testing.T) {
			p := ui.LayoutReference(th, 2000, 4000, ui.NewReference(keys, credit), measure7)

			t.Run("it should put the credit one grid under the hint", func(t *testing.T) {
				require.NotEmpty(t, p.Credit)
				assert.Equal(t, p.Hint.At.Y+16+grid, p.Credit[0].At.Y)
			})

			t.Run("it should start the credit at the panel's left padding", func(t *testing.T) {
				require.NotEmpty(t, p.Credit)
				assert.Equal(t, p.Rect.Min.X+pad, p.Credit[0].At.X)
			})

			t.Run("it should set the credit in the Small size", func(t *testing.T) {
				require.NotEmpty(t, p.Credit)
				assert.Equal(t, ui.Small, p.Credit[0].Size)
			})

			t.Run("it should wrap the credit onto more than one line", func(t *testing.T) {
				assert.Greater(t, len(p.Credit), 1)
			})

			t.Run("it should keep every line within the panel's padding", func(t *testing.T) {
				widest := 0.0
				for _, l := range p.Credit {
					widest = max(widest, float64(len(l.Text))*7)
				}
				assert.LessOrEqual(t, widest, p.Rect.Width()-2*pad)
			})

			t.Run("it should step each line down by one Small line", func(t *testing.T) {
				require.Greater(t, len(p.Credit), 1)
				assert.Equal(t, 16.0, p.Credit[1].At.Y-p.Credit[0].At.Y)
			})

			t.Run("it should grow the panel by a grid plus one Small line per line", func(t *testing.T) {
				assert.Equal(t, bare.Rect.Height()+grid+16*float64(len(p.Credit)), p.Rect.Height())
			})

			t.Run("it should not make the credit a term", func(t *testing.T) {
				require.NotEmpty(t, p.Credit)
				_, ok := p.Hit(p.Credit[0].At.Add(city.Point{X: 2, Y: 2}))
				assert.False(t, ok)
			})
		})

		t.Run(fmt.Sprintf("when another entry is open at scale %v", scale), func(t *testing.T) {
			ref := ui.NewReference(keys, credit)
			ref.Open(city.KindSession.Name())

			t.Run("it should lay out no credit", func(t *testing.T) {
				assert.Empty(t, ui.LayoutReference(th, 2000, 4000, ref, measure7).Credit)
			})
		})

		t.Run(fmt.Sprintf("when there is no credit at scale %v", scale), func(t *testing.T) {
			t.Run("it should lay out no credit", func(t *testing.T) {
				assert.Empty(t, bare.Credit)
			})
		})
	}
}
