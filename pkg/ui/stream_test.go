package ui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/ui"
)

func TestLayoutStream(t *testing.T) {
	th := ui.NewTheme(1)
	const width, height, top, footer = 1600.0, 1000.0, 32.0, 40.0
	at := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	list := []events.Event{
		{At: at.Add(-3 * time.Minute), Kind: events.Started, SessionID: "a", Title: "Add reset headers"},
		{At: at.Add(-time.Minute), Kind: events.NeedsYou, SessionID: "b", Title: "Allow the headers through"},
		{At: at.Add(-2 * time.Minute), Kind: events.Error, SessionID: "c", Title: "Cap the log buffer"},
	}
	area := ui.StreamArea(th, ui.TopRight, width, height, top, footer)
	s := ui.LayoutStream(th, list, area, measure7)

	t.Run("it should put the newest first", func(t *testing.T) {
		require.Len(t, s.Rows, 3)
		assert.Equal(t, []string{"b", "c", "a"}, []string{s.Rows[0].Event.SessionID, s.Rows[1].Event.SessionID, s.Rows[2].Event.SessionID})
	})

	t.Run("it should mute routine entries and not flagged ones", func(t *testing.T) {
		assert.Equal(t, []bool{false, false, true}, []bool{s.Rows[0].Muted, s.Rows[1].Muted, s.Rows[2].Muted})
	})

	t.Run("when there are more than fit", func(t *testing.T) {
		var many []events.Event
		for i := range 200 {
			many = append(many, events.Event{At: at.Add(-time.Duration(i) * time.Minute), Kind: events.Started, SessionID: fmt.Sprint(i)})
		}
		laid := ui.LayoutStream(th, many, area, measure7)

		t.Run("it should keep the newest", func(t *testing.T) {
			require.NotEmpty(t, laid.Rows)
			assert.True(t, laid.Rows[0].Event.SessionID == "0" && len(laid.Rows) < 200 && laid.Rows[len(laid.Rows)-1].Rect.Max.Y <= area.Max.Y)
		})
	})

	t.Run("when nothing has happened yet", func(t *testing.T) {
		empty := ui.LayoutStream(th, nil, area, measure7)

		t.Run("it should say so", func(t *testing.T) {
			assert.Equal(t, ui.StreamEmpty, empty.Empty.Text)
		})
	})

	t.Run("when a click lands on an entry", func(t *testing.T) {
		e, ok := s.Hit(s.Rows[1].Rect.Center())

		t.Run("it should give back that event", func(t *testing.T) {
			assert.True(t, ok && e.SessionID == "c")
		})
	})
}

func TestStreamArea(t *testing.T) {
	th := ui.NewTheme(1)
	const height, top, footer = 800.0, 32.0, 40.0
	for _, corner := range []ui.Corner{ui.TopRight, ui.BottomRight} {
		for _, width := range []float64{600, 700, 768, 960, 1280, 1600, 2560} {
			t.Run(fmt.Sprintf("when the panel is in the %v corner, and the window is %v wide", corner, width), func(t *testing.T) {
				area := ui.StreamArea(th, corner, width, height, top, footer)
				mini, hasMini := ui.LayoutMinimap(th, width, height, footer)
				streamWidthRules(t, th, area, width)

				t.Run("it should be its panel's height", func(t *testing.T) {
					assert.Equal(t, ui.StreamPanelGrids*th.Grid(), area.Height())
				})

				t.Run("it should sit against the right edge", func(t *testing.T) {
					assert.Equal(t, width, area.Max.X)
				})

				switch {
				case corner == ui.TopRight:
					t.Run("it should sit under the strip", func(t *testing.T) {
						assert.Equal(t, top, area.Min.Y)
					})
				case hasMini:
					t.Run("it should stop just above the minimap", func(t *testing.T) {
						assert.Equal(t, mini.Min.Y-th.Grid(), area.Max.Y)
					})
				default:
					t.Run("it should stop just above the footer", func(t *testing.T) {
						assert.Equal(t, height-footer, area.Max.Y)
					})
				}
			})
		}

		t.Run(fmt.Sprintf("when the panel is in the %v corner, and the window is too short for it", corner), func(t *testing.T) {
			const short = 180.0
			area := ui.StreamArea(th, corner, 1600, short, top, footer)

			t.Run("it should fit between the strip and the footer", func(t *testing.T) {
				assert.True(t, area.Min.Y >= top && area.Max.Y <= short-footer && area.Height() > 0, "panel %v", area)
			})
		})
	}
}

// streamWidthRules holds the column's width rules (item 100) in either
// corner.
func streamWidthRules(t *testing.T, th ui.Theme, area city.Rect, width float64) {
	t.Helper()
	fits := ui.StreamFits(th, width)
	if width/4 >= ui.StreamMinGrids*th.Grid() {
		t.Run("and the panel fits", func(t *testing.T) {
			t.Run("it should be no wider than a quarter of the window", func(t *testing.T) {
				assert.LessOrEqual(t, area.Width(), width/4)
			})

			t.Run("it should be no wider than its full width", func(t *testing.T) {
				assert.LessOrEqual(t, area.Width(), ui.StreamGrids*th.Grid())
			})

			t.Run("it should be shown by default", func(t *testing.T) {
				assert.True(t, ui.StreamView{}.Shown(fits))
			})
		})
		return
	}

	t.Run("and the panel does not fit", func(t *testing.T) {
		t.Run("it should be hidden by default", func(t *testing.T) {
			assert.False(t, ui.StreamView{}.Shown(fits))
		})

		t.Run("and its key is pressed", func(t *testing.T) {
			t.Run("it should be shown", func(t *testing.T) {
				assert.True(t, ui.StreamView{}.Toggle(fits).Shown(fits))
			})

			t.Run("it should be its minimum width, within the window", func(t *testing.T) {
				assert.Equal(t, min(ui.StreamMinGrids*th.Grid(), width), area.Width())
			})
		})
	})
}

func TestToggleStream(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		t.Run(fmt.Sprintf("when the remembered hide is %v", hidden), func(t *testing.T) {
			t.Run("and the column fits", func(t *testing.T) {
				t.Run("it should flip the remembered hide", func(t *testing.T) {
					assert.Equal(t, !hidden, ui.StreamView{Hidden: hidden}.Toggle(true).Hidden)
				})
			})

			t.Run("and the column does not fit", func(t *testing.T) {
				t.Run("and it is down", func(t *testing.T) {
					up := ui.StreamView{Hidden: hidden}.Toggle(false)

					t.Run("it should show it for this run", func(t *testing.T) {
						assert.True(t, up.Shown(false))
					})

					t.Run("it should leave the remembered hide as it was", func(t *testing.T) {
						assert.Equal(t, hidden, up.Hidden)
					})
				})

				t.Run("and it is up", func(t *testing.T) {
					down := ui.StreamView{Hidden: hidden}.Toggle(false).Toggle(false)

					t.Run("it should take it down", func(t *testing.T) {
						assert.False(t, down.Shown(false))
					})

					t.Run("it should not remember a hide", func(t *testing.T) {
						assert.Equal(t, hidden, down.Hidden)
					})
				})
			})
		})
	}
}

func TestLayoutStreamWraps(t *testing.T) {
	th := ui.NewTheme(1)
	at := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	long := "Allow the rate limit headers through the proxy and the cache on every route"
	list := []events.Event{
		{At: at, Kind: events.NeedsYou, SessionID: "a", Title: long},
		{At: at.Add(-time.Minute), Kind: events.Started, SessionID: "b", Title: "Short"},
	}
	area := ui.StreamArea(th, ui.TopRight, 960, 800, 32, 40)
	s := ui.LayoutStream(th, list, area, measure7)
	require.Len(t, s.Rows, 2)
	first := s.Rows[0]

	t.Run("when an entry is longer than the column", func(t *testing.T) {
		t.Run("it should wrap onto more than one line", func(t *testing.T) {
			assert.Greater(t, len(first.Lines), 1)
		})

		t.Run("it should keep every word", func(t *testing.T) {
			var words []string
			for _, l := range first.Lines {
				words = append(words, strings.Fields(l.Text)...)
			}
			assert.Equal(t, strings.Fields(string(events.NeedsYou)+" · "+long), words)
		})

		t.Run("it should keep each line within the column", func(t *testing.T) {
			for _, l := range first.Lines {
				w, _ := measure7(l.Text, l.Size)
				assert.LessOrEqual(t, l.At.X+w, area.Max.X, l.Text)
			}
		})

		t.Run("it should push the next entry below the wrapped one", func(t *testing.T) {
			last := first.Lines[len(first.Lines)-1]
			assert.Greater(t, s.Rows[1].Lines[0].At.Y, last.At.Y)
		})
	})
}
