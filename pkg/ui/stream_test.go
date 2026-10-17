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
	s := ui.LayoutStream(th, list, area, measure7, 0)

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
		laid := ui.LayoutStream(th, many, area, measure7, 0)

		t.Run("it should keep the newest", func(t *testing.T) {
			require.NotEmpty(t, laid.Rows)
			assert.True(t, laid.Rows[0].Event.SessionID == "0" && len(laid.Rows) < 200 && laid.Rows[len(laid.Rows)-1].Rect.Max.Y <= area.Max.Y)
		})
	})

	t.Run("when nothing has happened yet", func(t *testing.T) {
		empty := ui.LayoutStream(th, nil, area, measure7, 0)

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
				streamWidthRules(t, th, area, width)

				t.Run("it should be its panel's height", func(t *testing.T) {
					assert.Equal(t, ui.StreamPanelGrids*th.Grid(), area.Height())
				})

				t.Run("it should sit against the right edge", func(t *testing.T) {
					assert.Equal(t, width, area.Max.X)
				})

				if corner == ui.TopRight {
					t.Run("it should sit under the strip", func(t *testing.T) {
						assert.Equal(t, top, area.Min.Y)
					})
				} else {
					t.Run("it should sit on the footer", func(t *testing.T) {
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
	s := ui.LayoutStream(th, list, area, measure7, 0)
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

func TestMinimapBesideStream(t *testing.T) {
	th := ui.NewTheme(1)
	const width, height, top, footer = 1600.0, 900.0, 32.0, 40.0
	mini, ok := ui.LayoutMinimap(th, width, height, footer)
	require.True(t, ok)

	t.Run("when the panel is in the bottom right and the minimap is drawn", func(t *testing.T) {
		panel := ui.StreamArea(th, ui.BottomRight, width, height, top, footer)
		moved := ui.MinimapBesideStream(th, mini, panel, ui.BottomRight)

		t.Run("it should sit above the panel, clear of it", func(t *testing.T) {
			assert.True(t, moved.Max.Y < panel.Min.Y && moved.Height() == mini.Height(), "minimap %v, panel %v", moved, panel)
		})

		t.Run("it should keep against the right edge", func(t *testing.T) {
			assert.Equal(t, mini.Max.X, moved.Max.X)
		})
	})

	t.Run("when the panel is in the top right", func(t *testing.T) {
		panel := ui.StreamArea(th, ui.TopRight, width, height, top, footer)

		t.Run("it should not move", func(t *testing.T) {
			assert.Equal(t, mini, ui.MinimapBesideStream(th, mini, panel, ui.TopRight))
		})
	})
}

func TestLayoutStreamScrolled(t *testing.T) {
	th := ui.NewTheme(1)
	at := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	var list []events.Event
	for i := range 40 {
		list = append(list, events.Event{At: at.Add(-time.Duration(i) * time.Minute), Kind: events.Started, SessionID: fmt.Sprint(i)})
	}
	area := ui.StreamArea(th, ui.TopRight, 1600, 900, 32, 40)
	laid := func(skip int) ui.Stream { return ui.LayoutStream(th, list, area, measure7, skip) }
	first := func(s ui.Stream) string {
		require.NotEmpty(t, s.Rows)
		return s.Rows[0].Event.SessionID
	}

	t.Run("when it is not scrolled", func(t *testing.T) {
		s := laid(0)

		t.Run("it should start at the newest entry", func(t *testing.T) {
			assert.Equal(t, "0", first(s))
		})

		t.Run("it should show no newer mark", func(t *testing.T) {
			assert.Empty(t, s.Newer.Text)
		})

		t.Run("it should draw a scroll bar", func(t *testing.T) {
			assert.Positive(t, s.Thumb.Area())
		})

		t.Run("it should put the thumb at the top of the track", func(t *testing.T) {
			assert.Equal(t, s.Track.Min.Y, s.Thumb.Min.Y)
		})
	})

	t.Run("when it is scrolled back by 5", func(t *testing.T) {
		s := laid(5)

		t.Run("it should start at the fifth-newest entry", func(t *testing.T) {
			assert.Equal(t, "5", first(s))
		})

		t.Run("it should say how many newer entries are above", func(t *testing.T) {
			assert.Contains(t, s.Newer.Text, "5 newer")
		})

		t.Run("it should move the thumb down the track", func(t *testing.T) {
			assert.Greater(t, s.Thumb.Min.Y, laid(0).Thumb.Min.Y)
		})
	})

	t.Run("when it is scrolled past the oldest", func(t *testing.T) {
		s := laid(1000)

		t.Run("it should stop where the oldest entry is the last one shown", func(t *testing.T) {
			require.NotEmpty(t, s.Rows)
			assert.True(t, s.Rows[len(s.Rows)-1].Event.SessionID == "39" && laid(s.Skip - 1).Rows[len(laid(s.Skip-1).Rows)-1].Event.SessionID != "39", "skip %d", s.Skip)
		})
	})

	t.Run("when it is scrolled before the newest", func(t *testing.T) {
		t.Run("it should start at the newest", func(t *testing.T) {
			assert.Equal(t, "0", first(laid(-3)))
		})
	})

	t.Run("when everything fits", func(t *testing.T) {
		s := ui.LayoutStream(th, list[:2], area, measure7, 0)

		t.Run("it should draw no scroll bar", func(t *testing.T) {
			assert.Zero(t, s.Thumb.Area())
		})
	})

	t.Run("when the bar is dragged", func(t *testing.T) {
		s := laid(0)
		for _, c := range []struct {
			name string
			y    float64
			want int
		}{
			{"to the top of the track", s.Track.Min.Y, 0},
			{"to the bottom of the track", s.Track.Max.Y, laid(1000).Skip},
		} {
			t.Run(c.name, func(t *testing.T) {
				t.Run("it should scroll to the entry at that point", func(t *testing.T) {
					assert.Equal(t, c.want, s.SkipAt(c.y))
				})
			})
		}
	})
}

func TestFollowStream(t *testing.T) {
	t.Run("when at the newest and entries arrive", func(t *testing.T) {
		t.Run("it should stay at the newest", func(t *testing.T) {
			assert.Equal(t, 0, ui.FollowStream(0, 10, 13))
		})
	})

	t.Run("when scrolled back and entries arrive", func(t *testing.T) {
		t.Run("it should move back by as many, keeping the same entries in view", func(t *testing.T) {
			assert.Equal(t, 7, ui.FollowStream(4, 10, 13))
		})
	})
}
