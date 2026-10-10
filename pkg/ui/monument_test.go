package ui

import (
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonumentMeetsTheGround(t *testing.T) {
	m, ok := LayoutMonument("botropolis", city.Point{X: 200, Y: 300}, 160, measureFixed)
	require.True(t, ok)

	t.Run("when a sign is built as wide as its base course", func(t *testing.T) {
		t.Run("it should put all of its width on the ground, not the two fifths the code allows", func(t *testing.T) {
			assert.InDelta(t, 1.0, MonumentOnGround(m.Base.Width(), m.Whole().Width()), 0.0001)
		})
	})

	t.Run("when a sign is built to the widest the code allows", func(t *testing.T) {
		t.Run("it should sit exactly on the two-fifths limit", func(t *testing.T) {
			assert.InDelta(t, MonumentGround, MonumentOnGround(80, MonumentWidest(80)), 0.0001)
		})
	})
}

func TestLayoutMonument(t *testing.T) {
	foot := city.Point{X: 200, Y: 300}
	m, ok := LayoutMonument("botropolis", foot, 160, measureFixed)
	require.True(t, ok)

	t.Run("when the sign stands on the ground", func(t *testing.T) {
		t.Run("it should be twice as wide as it is tall", func(t *testing.T) {
			assert.InDelta(t, MonumentAspect, m.Whole().Width()/m.Whole().Height(), 0.0001)
		})

		t.Run("it should stand its base course on the ground point", func(t *testing.T) {
			assert.InDelta(t, foot.Y, m.Base.Max.Y, 0.0001)
		})

		t.Run("it should make the base course the widest thing it has", func(t *testing.T) {
			assert.GreaterOrEqual(t, m.Base.Width(), m.Cornice.Width())
		})

		t.Run("it should set the body in from the base course", func(t *testing.T) {
			assert.Greater(t, m.Left.Min.X, m.Base.Min.X)
		})
	})

	t.Run("when the cornice is set over the piers", func(t *testing.T) {
		t.Run("it should oversail them rather than sit flush", func(t *testing.T) {
			assert.Less(t, m.Cornice.Min.X, m.Left.Min.X)
		})

		t.Run("it should show an underside below itself", func(t *testing.T) {
			assert.InDelta(t, m.Cornice.Max.Y, m.Soffit.Min.Y, 0.0001)
		})

		t.Run("it should sit the piers below that underside", func(t *testing.T) {
			assert.InDelta(t, m.Soffit.Max.Y, m.Left.Min.Y, 0.0001)
		})
	})

	t.Run("when the panel is set between the piers", func(t *testing.T) {
		t.Run("it should start where the left pier ends", func(t *testing.T) {
			assert.InDelta(t, m.Left.Max.X, m.Panel.Min.X, 0.0001)
		})

		t.Run("it should end where the right pier begins", func(t *testing.T) {
			assert.InDelta(t, m.Right.Min.X, m.Panel.Max.X, 0.0001)
		})

		t.Run("it should carry a reveal along its top, so it reads recessed", func(t *testing.T) {
			assert.InDelta(t, m.Panel.Min.Y, m.Reveal.Min.Y, 0.0001)
		})
	})

	t.Run("when the stone is coursed", func(t *testing.T) {
		t.Run("it should score the piers and the base, not the panel", func(t *testing.T) {
			for _, c := range m.Courses {
				assert.False(t, m.Panel.Contains(c.Center()), c)
			}
		})
	})

	t.Run("when the sign faces the viewer", func(t *testing.T) {
		t.Run("it should set the copy level rather than shear it into the world", func(t *testing.T) {
			assert.Zero(t, m.Copy.Lean)
		})
	})

	t.Run("when the sign stands on the earth", func(t *testing.T) {
		t.Run("it should cast a contact shadow at its foot", func(t *testing.T) {
			assert.True(t, m.Shadow.Contains(city.Point{X: foot.X, Y: foot.Y}), m.Shadow)
		})
	})

	t.Run("when the sign is too small for the copy to read", func(t *testing.T) {
		t.Run("it should decline, leaving the name to the hover plate", func(t *testing.T) {
			_, ok := LayoutMonument("botropolis", foot, 6, measureFixed)
			assert.False(t, ok)
		})
	})

	t.Run("when the project has no name", func(t *testing.T) {
		t.Run("it should decline rather than stand an empty sign", func(t *testing.T) {
			_, ok := LayoutMonument("", foot, 160, measureFixed)
			assert.False(t, ok)
		})
	})
}
