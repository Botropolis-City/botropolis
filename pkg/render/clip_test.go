package render

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/ui"
)

// clipByRune is the search clipTo replaced: the whole string, then every
// prefix a rune shorter, until one fits.
func clipByRune(s string, width float64, size ui.Size, measure ui.Measure) string {
	if w, _ := measure(s, size); w <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 {
		r = r[:len(r)-1]
		if w, _ := measure(string(r)+"…", size); w <= width {
			return string(r) + "…"
		}
	}
	return ""
}

// Item 162: the group-review panel clipped its rows a rune at a time, so
// every frame shaped hundreds of near-whole copies of a long row, and
// Ebitengine's shaping cache kept 51 MB of them.
func TestClipTo(t *testing.T) {
	const advance = 10.0
	type stub struct {
		asked []string
	}
	newStub := func() (*stub, ui.Measure) {
		s := &stub{}
		return s, func(text string, _ ui.Size) (float64, float64) {
			s.asked = append(s.asked, text)
			return advance * float64(utf8.RuneCountInString(text)), advance
		}
	}

	t.Run("when the measure is a fixed advance a rune", func(t *testing.T) {
		cases := []struct {
			name  string
			s     string
			width float64
		}{
			{"the string fits", "abcdef", 60},
			{"the string is empty", "", 60},
			{"nothing fits", "abcdef", 5},
			{"only the ellipsis fits", "abcdef", 10},
			{"one rune short", "abcdef", 50},
			{"a long string", strings.Repeat("abcdefghij", 200), 400},
		}
		for _, c := range cases {
			t.Run("and "+c.name, func(t *testing.T) {
				_, measure := newStub()
				t.Run("it should return the same clip as the rune-at-a-time search", func(t *testing.T) {
					assert.Equal(t, clipByRune(c.s, c.width, ui.Body, measure), clipTo(c.s, c.width, ui.Body, measure))
				})
			})
		}
	})

	t.Run("when a long string is clipped", func(t *testing.T) {
		const runes, width = 2000, 400.0
		fit := int(width/advance) - 1
		s, measure := newStub()
		clipTo(strings.Repeat("x", runes), width, ui.Body, measure)

		t.Run("it should never measure a string of more than twice the runes that fit, plus one", func(t *testing.T) {
			longest := 0
			for _, a := range s.asked {
				longest = max(longest, utf8.RuneCountInString(a))
			}
			assert.LessOrEqual(t, longest, max(2*fit, 2*clipFirstProbe)+1)
		})

		t.Run("it should measure at most twice the log of its length", func(t *testing.T) {
			assert.LessOrEqual(t, len(s.asked), 2*int(math.Ceil(math.Log2(runes)))+2)
		})
	})

	t.Run("when the measure is the city's own faces", func(t *testing.T) {
		f, err := newFaces(ui.NewTheme(1))
		require.NoError(t, err)
		texts := map[string]string{
			"kerning pairs":   strings.Repeat("AVAWAYTo Ta Ty Wa Yo LT ", 12),
			"combining marks": strings.Repeat("éàö ñ ", 20),
			"ligatures":       strings.Repeat("office fifty affluent ", 10),
			"a long path":     strings.Repeat("~/workspaces/github/botropolis-city/", 8),
		}
		for name, s := range texts {
			t.Run("and the text has "+name, func(t *testing.T) {
				for _, size := range []ui.Size{ui.Small, ui.Body} {
					for width := 0.0; width <= 600; width += 7 {
						t.Run(fmt.Sprintf("at %v wide in size %v it should return the same clip as the rune-at-a-time search", width, size), func(t *testing.T) {
							assert.Equal(t, clipByRune(s, width, size, f.Measure), clipTo(s, width, size, f.Measure))
						})
					}
				}
			})
		}
	})

	t.Run("when the same row is drawn twice", func(t *testing.T) {
		s, measure := newStub()
		c := newClipper(measure)
		row := strings.Repeat("abcdefghij", 50)
		c.clip(row, 200, ui.Body)
		first := len(s.asked)
		c.clip(row, 200, ui.Body)

		t.Run("it should measure nothing the second time", func(t *testing.T) {
			assert.Equal(t, first, len(s.asked))
		})
	})

	t.Run("when more rows are clipped than the memo holds", func(t *testing.T) {
		_, measure := newStub()
		c := newClipper(measure)
		for i := range clipMemoLimit + 10 {
			c.clip(fmt.Sprint("row ", i), 200, ui.Body)
		}

		t.Run("it should hold no more than its limit", func(t *testing.T) {
			assert.LessOrEqual(t, len(c.memo), clipMemoLimit)
		})
	})
}
