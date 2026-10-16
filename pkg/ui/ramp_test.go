package ui_test

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/ui"
)

// stateTones is every colour that already carries a meaning. A ramp stop
// or a category colour landing on one would make that meaning ambiguous.
var stateTones = map[string]color.NRGBA{
	"needs-you":  ui.DefaultPalette.NeedsYou,
	"working":    ui.DefaultPalette.Working,
	"waiting":    ui.DefaultPalette.Waiting,
	"unattended": ui.DefaultPalette.Unattended,
	"resting":    ui.DefaultPalette.Resting,
	"error":      ui.DefaultPalette.Error,
	"merged":     ui.DefaultPalette.Merged,
}

func TestSequentialRamp(t *testing.T) {
	t.Run("when the ramp is walked from end to end", func(t *testing.T) {
		t.Run("it should climb in luminance the whole way, so it reads with no hue at all", func(t *testing.T) {
			last := -1.0
			for i := 0; i <= 20; i++ {
				at := relativeLuminance(ui.Sequential.At(float64(i) / 20))
				require.Greater(t, at, last, "step %d", i)
				last = at
			}
		})

		t.Run("it should span enough luminance to tell its ends apart", func(t *testing.T) {
			assert.Greater(t, relativeLuminance(ui.Sequential.At(1))-relativeLuminance(ui.Sequential.At(0)), 0.5)
		})
	})

	t.Run("when a value falls outside the scale", func(t *testing.T) {
		t.Run("it should clamp rather than wrap", func(t *testing.T) {
			assert.Equal(t, ui.Sequential.At(0), ui.Sequential.At(-3))
			assert.Equal(t, ui.Sequential.At(1), ui.Sequential.At(7))
		})
	})

	// A ramp that carries its signal in lightness survives the loss of a
	// cone type. Checking it under the simulation is what says so.
	for _, deficiency := range deficiencies {
		t.Run("when the ramp is read with "+deficiency+"opia", func(t *testing.T) {
			t.Run("it should still climb the whole way", func(t *testing.T) {
				last := -1.0
				for i := 0; i <= 20; i++ {
					seen := simulate(linearOf(ui.Sequential.At(float64(i)/20)), deficiency)
					at := 0.2126*seen[0] + 0.7152*seen[1] + 0.0722*seen[2]
					require.Greater(t, at, last, "step %d", i)
					last = at
				}
			})
		})
	}

	t.Run("when the ramp is compared with the state tones", func(t *testing.T) {
		// Condition 1 — no ramp uses a state tone — cannot be met in colour
		// alone, and this is where that shows. With amber, blue, teal,
		// violet, slate, red and green already spent, a six-stop ramp of any
		// hue passes near something: this one comes within delta E 11.1 of
		// the error red for a full-colour reader, and within 0.3 of the
		// waiting teal under deuteranopia, where plum and teal collapse onto
		// each other. The same squeeze caps the categorical palette; see the
		// note on ui.Categorical for the search that established it.
		//
		// What keeps the condition honest is not the palette but the mode: a
		// view is subtractive, so the tones are off the screen while a ramp
		// is on it. That invariant is the one worth testing, and it is tested
		// where it lives — in the chrome, not here.
		//
		// This is a regression guard on the gap that does exist, so neither
		// palette can drift quietly closer to the other.
		const measuredFloor = 11.0

		t.Run("it should not come any closer than it already does", func(t *testing.T) {
			for i := 0; i <= 40; i++ {
				at := ui.Sequential.At(float64(i) / 40)
				for name, tone := range stateTones {
					require.Greater(t, deltaE(at, tone, ""), measuredFloor, "stop %d sits on %s", i, name)
				}
			}
		})
	})
}

func TestCategorical(t *testing.T) {
	// A map is an all-pairs surface: any two districts can end up side by
	// side, so every pair has to hold, not just the neighbours in a
	// legend. That is what caps the palette at three.
	t.Run("when every pair of categories is compared, as a map demands", func(t *testing.T) {
		t.Run("it should hold them apart for a reader with full colour vision", func(t *testing.T) {
			for i := range ui.Categorical {
				for j := i + 1; j < len(ui.Categorical); j++ {
					require.GreaterOrEqual(t, deltaE(ui.Categorical[i], ui.Categorical[j], ""), normalFloor, "%d and %d", i, j)
				}
			}
		})

		t.Run("it should hold them apart for the commonest deficiencies too", func(t *testing.T) {
			for i := range ui.Categorical {
				for j := i + 1; j < len(ui.Categorical); j++ {
					require.GreaterOrEqual(t, worstCVD(ui.Categorical[i], ui.Categorical[j]), cvdFloor, "%d and %d", i, j)
				}
			}
		})
	})

	t.Run("when a category colour is put on the dark map", func(t *testing.T) {
		t.Run("it should sit inside the lightness band the surface allows", func(t *testing.T) {
			for i, c := range ui.Categorical {
				require.GreaterOrEqual(t, lightness(c), darkBandLow, "category %d", i)
				require.LessOrEqual(t, lightness(c), darkBandHigh, "category %d", i)
			}
		})

		t.Run("it should carry enough chroma to read as a hue rather than grey", func(t *testing.T) {
			for i, c := range ui.Categorical {
				require.GreaterOrEqual(t, chroma(c), chromaFloor, "category %d", i)
			}
		})
	})

	t.Run("when more kinds turn up than there are colours", func(t *testing.T) {
		t.Run("it should hand out the neutral rather than repeat a hue", func(t *testing.T) {
			assert.Equal(t, ui.Uncategorised, ui.Category(len(ui.Categorical)))
		})

		t.Run("it should do the same below the first slot", func(t *testing.T) {
			assert.Equal(t, ui.Uncategorised, ui.Category(-1))
		})
	})

	t.Run("when the neutral is put beside the hues it stands in for", func(t *testing.T) {
		t.Run("it should stay apart from every one of them", func(t *testing.T) {
			for i, c := range ui.Categorical {
				require.GreaterOrEqual(t, deltaE(ui.Uncategorised, c, ""), normalFloor, "category %d", i)
			}
		})

		t.Run("it should stay apart from them under the commonest deficiencies too", func(t *testing.T) {
			for i, c := range ui.Categorical {
				require.GreaterOrEqual(t, worstCVD(ui.Uncategorised, c), cvdFloor, "category %d", i)
			}
		})
	})
}
