package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/ui"
)

func TestFaces(t *testing.T) {
	t.Run("when measuring at 1x", func(t *testing.T) {
		f, err := newFaces(ui.NewTheme(1))
		require.NoError(t, err)
		short, _ := f.Measure("city", ui.Body)
		long, _ := f.Measure("city hall", ui.Body)

		t.Run("it should measure a longer string wider", func(t *testing.T) {
			assert.Greater(t, long, short)
		})

		t.Run("it should measure the display size taller than the small size", func(t *testing.T) {
			_, small := f.Measure("city", ui.Small)
			_, display := f.Measure("city", ui.Display)
			assert.Greater(t, display, small)
		})

		t.Run("it should measure nothing as no width", func(t *testing.T) {
			w, _ := f.Measure("", ui.Body)
			assert.Zero(t, w)
		})

		t.Run("it should set figures tabular, so numbers line up", func(t *testing.T) {
			ones, _ := f.Measure("111", ui.Body)
			eights, _ := f.Measure("888", ui.Body)
			assert.Equal(t, eights, ones)
		})
	})

	t.Run("when measuring a reply's styles", func(t *testing.T) {
		f, err := newFaces(ui.NewTheme(1))
		require.NoError(t, err)
		plain, plainH := f.Styled("kiln", ui.Body, ui.SpanPlain)

		t.Run("it should set strong wider than plain", func(t *testing.T) {
			strong, _ := f.Styled("kiln", ui.Body, ui.SpanStrong)
			assert.Greater(t, strong, plain)
		})

		t.Run("it should set code in a monospace face", func(t *testing.T) {
			narrow, _ := f.Styled("iiii", ui.Body, ui.SpanCode)
			wide, _ := f.Styled("mmmm", ui.Body, ui.SpanCode)
			assert.Equal(t, narrow, wide)
		})

		t.Run("it should set bold code in a monospace face", func(t *testing.T) {
			narrow, _ := f.Styled("iiii", ui.Body, ui.SpanCodeStrong)
			wide, _ := f.Styled("mmmm", ui.Body, ui.SpanCodeStrong)
			assert.Equal(t, narrow, wide)
		})

		t.Run("it should give every style the plain line's height", func(t *testing.T) {
			var heights []float64
			for _, style := range []ui.SpanStyle{ui.SpanStrong, ui.SpanEmphasis, ui.SpanCode} {
				_, h := f.Styled("kiln", ui.Body, style)
				heights = append(heights, h)
			}
			assert.Equal(t, []float64{plainH, plainH, plainH}, heights)
		})

		t.Run("it should sit every style on the plain face's baseline", func(t *testing.T) {
			base := f.Face(ui.Body).Metrics().HAscent
			var baselines []float64
			for _, style := range []ui.SpanStyle{ui.SpanPlain, ui.SpanStrong, ui.SpanCode, ui.SpanCodeStrong} {
				face, drop := f.Styling(ui.Body, style)
				baselines = append(baselines, drop+face.Metrics().HAscent)
			}
			assert.Equal(t, []float64{base, base, base, base}, baselines)
		})
	})

	t.Run("when measuring at 2x", func(t *testing.T) {
		one, err := newFaces(ui.NewTheme(1))
		require.NoError(t, err)
		two, err := newFaces(ui.NewTheme(2))
		require.NoError(t, err)
		w1, h1 := one.Measure("2 working", ui.Body)
		w2, h2 := two.Measure("2 working", ui.Body)

		t.Run("it should measure the same string twice as wide", func(t *testing.T) {
			assert.InDelta(t, 2*w1, w2, 1)
		})

		t.Run("it should measure the same string twice as tall", func(t *testing.T) {
			assert.InDelta(t, 2*h1, h2, 1)
		})
	})
}
