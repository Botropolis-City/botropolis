package ui_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// everyKey is the help's list as the city builds it, chords spelled out,
// so the layout is tried against the length it actually is. Item 127.
var everyKey = []ui.Key{
	{"drag", "pan"},
	{"arrows", "pan"},
	{"wheel", "zoom"},
	{"+ / -", "zoom"},
	{"click", "select"},
	{"enter", "attach the selection"},
	{"i", "the selection's details"},
	{"o", "read the selection's conversation"},
	{"tab", "next needs-you"},
	{"w", "triage: walk what waits on you, longest-waiting first"},
	{"shift+w", "the journal: where you were, and what is next"},
	{"l", "the stream: what is happening, newest first"},
	{"v", "the lenses, and the lens's panel in full"},
	{"1-6", "a lens; press again for its next view"},
	{"c", "new session here"},
	{"d d", "demolish the selection"},
	{"f", "Skyline"},
	{"wheel out past Skyline", "the City Plan: every district at a glance; wheel in to return"},
	{"m", "the City Plan: every district on one screen; click a badge to fly in"},
	{"z", "labels off and on"},
	{"0", "reset the view"},
	{"r", "turn the camera"},
	{"n", "night, day, auto"},
	{"[ ]", "scrub the clock an hour"},
	{"s", "settings"},
	{"x", "power breakdown (also click the plant)"},
	{"g", "groups board (also click City Hall)"},
	{"shift+n", "New session, in any directory (also on the board)"},
	{"t", "timeline; enter jumps to the session"},
	{"b", "sidebar"},
	{"h", "hide the UI"},
	{"p", "save a screenshot"},
	{"shift+p", "pack the selected session up, or unpack a crate"},
	{"shift+q", "close up: note what is open and what to start with next time; pause only what you choose"},
	{"/ or ctrl+k", "find a session by what was said; the map dims the rest"},
	{"ctrl+c", "in the finder: copy how to resume the session"},
	{"ctrl+enter", "in the finder: show the session on the map"},
	{"?", "the reference: what everything on the map is"},
	{"hold e", "caption everything on the map"},
	{"q", "quit (asks; y or enter confirms)"},
}

func TestLayoutHelp(t *testing.T) {
	th := ui.NewTheme(1)
	keys := []ui.Key{{"drag", "pan"}, {"?", "help"}, {"tab", "next needs-you"}}

	t.Run("when the help is laid out", func(t *testing.T) {
		h := ui.LayoutHelp(th, 800, 600, "Keys", keys, measure7, 0)

		t.Run("it should centre the panel in the window", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 400, Y: 300}, h.Rect.Center())
		})

		t.Run("it should title it at the title size", func(t *testing.T) {
			assert.Equal(t, ui.Text{Text: "Keys", At: h.Rect.Min.Add(city.Point{X: 24, Y: 24}), Size: ui.Title}, h.Title)
		})

		t.Run("it should stack one row per key", func(t *testing.T) {
			require.Len(t, h.Rows, 3)
			assert.Equal(t, 16+8+4.0, h.Rows[1].Chip.Min.Y-h.Rows[0].Chip.Min.Y)
		})

		t.Run("it should align the labels in one column past the widest chip", func(t *testing.T) {
			assert.Equal(t, h.Rows[0].Label[0].At.X, h.Rows[2].Label[0].At.X)
		})

		t.Run("it should put that column one grid after the widest chip", func(t *testing.T) {
			assert.Equal(t, h.Rect.Min.X+24+4*7+8+8, h.Rows[0].Label[0].At.X)
		})

		t.Run("it should size the panel to the longest row plus padding", func(t *testing.T) {
			assert.Equal(t, 24+4*7+8+8+14*7+24.0, h.Rect.Width())
		})
	})

	for _, scale := range []float64{1, 2} {
		th := ui.NewTheme(scale)
		for _, size := range []city.Point{{X: 700, Y: 600}, {X: 800, Y: 600}, {X: 1024, Y: 768}, {X: 1280, Y: 800}, {X: 1600, Y: 1000}, {X: 1920, Y: 1080}, {X: 2560, Y: 1440}} {
			t.Run(fmt.Sprintf("when the window is %vx%v at scale %v", size.X, size.Y, scale), func(t *testing.T) {
				window := city.RectAt(0, 0, size.X, size.Y)
				first := ui.LayoutHelp(th, size.X, size.Y, "Keys", everyKey, measure7, 0)
				var laid []ui.Help
				for skip := 0; skip <= first.Max; skip++ {
					laid = append(laid, ui.LayoutHelp(th, size.X, size.Y, "Keys", everyKey, measure7, skip))
				}

				t.Run("it should keep the panel inside the window", func(t *testing.T) {
					for _, h := range laid {
						assert.True(t, window.Union(h.Rect) == window, "skip %d: %v", h.Skip, h.Rect)
					}
				})

				t.Run("it should keep every placed row inside the panel", func(t *testing.T) {
					for _, h := range laid {
						for _, r := range h.Rows {
							assert.True(t, h.Rect.Union(r.Band) == h.Rect, "skip %d: %q at %v", h.Skip, r.Key.Key, r.Band)
						}
					}
				})

				t.Run("it should keep every label line inside the panel's width", func(t *testing.T) {
					for _, h := range laid {
						for _, r := range h.Rows {
							for _, l := range r.Label {
								w, _ := measure7(l.Text, l.Size)
								assert.LessOrEqual(t, l.At.X+w, h.Rect.Max.X, "skip %d: %q", h.Skip, l.Text)
							}
						}
					}
				})

				t.Run("it should keep the marks clear of the rows", func(t *testing.T) {
					for _, h := range laid {
						for _, r := range h.Rows {
							for _, m := range []ui.Text{h.Above, h.Below} {
								if m.Text == "" {
									continue
								}
								_, lh := measure7(m.Text, m.Size)
								mark := city.RectAt(m.At.X, m.At.Y, 1, lh)
								assert.False(t, r.Band.Overlaps(mark), "skip %d: %q under %q", h.Skip, r.Key.Key, m.Text)
							}
						}
					}
				})

				t.Run("it should place no two rows overlapping", func(t *testing.T) {
					for _, h := range laid {
						for i, a := range h.Rows {
							for _, b := range h.Rows[i+1:] {
								assert.False(t, a.Band.Overlaps(b.Band), "skip %d: %q and %q", h.Skip, a.Key.Key, b.Key.Key)
							}
						}
					}
				})

				t.Run("it should reach every key at some scroll", func(t *testing.T) {
					seen := map[ui.Key]bool{}
					for _, h := range laid {
						for _, r := range h.Rows {
							seen[r.Key] = true
						}
					}
					assert.Len(t, seen, len(everyKey))
				})
			})
		}
	}

	t.Run("when the window is wide and tall enough for every key", func(t *testing.T) {
		h := ui.LayoutHelp(th, 1600, 1000, "Keys", everyKey, measure7, 0)

		t.Run("it should not scroll", func(t *testing.T) {
			assert.Equal(t, 0, h.Max)
		})

		t.Run("it should lay the keys out in more than one column", func(t *testing.T) {
			assert.Greater(t, h.Columns, 1)
		})

		t.Run("it should start each column's labels one grid past that column's widest chip", func(t *testing.T) {
			widest := map[float64]float64{}
			for _, r := range h.Rows {
				widest[r.Band.Min.X] = max(widest[r.Band.Min.X], r.Chip.Width())
			}
			for _, r := range h.Rows {
				assert.Equal(t, r.Band.Min.X+widest[r.Band.Min.X]+8, r.Label[0].At.X, "%q", r.Key.Key)
			}
		})

		t.Run("it should keep the bindings' order down each column, then across", func(t *testing.T) {
			var got []ui.Key
			for i, r := range h.Rows {
				got = append(got, r.Key)
				if i > 0 {
					prev := h.Rows[i-1]
					down := r.Band.Min.X == prev.Band.Min.X && r.Band.Min.Y > prev.Band.Min.Y
					across := r.Band.Min.X > prev.Band.Min.X
					assert.True(t, down || across, "%q after %q", r.Key.Key, prev.Key.Key)
				}
			}
			assert.Equal(t, everyKey, got)
		})
	})

	t.Run("when the list does not fit the window", func(t *testing.T) {
		top := ui.LayoutHelp(th, 700, 600, "Keys", everyKey, measure7, 0)

		t.Run("and it is not scrolled", func(t *testing.T) {
			t.Run("it should show no mark above", func(t *testing.T) {
				assert.Empty(t, top.Above.Text)
			})

			t.Run("it should say how many more are below", func(t *testing.T) {
				assert.Equal(t, fmt.Sprintf("%d more below", len(everyKey)-len(top.Rows)), top.Below.Text)
			})
		})

		t.Run("and it is scrolled to the end", func(t *testing.T) {
			end := ui.LayoutHelp(th, 700, 600, "Keys", everyKey, measure7, top.Max)

			t.Run("it should say how many more are above", func(t *testing.T) {
				assert.Equal(t, fmt.Sprintf("%d more above", top.Max), end.Above.Text)
			})

			t.Run("it should show no mark below", func(t *testing.T) {
				assert.Empty(t, end.Below.Text)
			})

			t.Run("it should end on the last key", func(t *testing.T) {
				assert.Equal(t, everyKey[len(everyKey)-1], end.Rows[len(end.Rows)-1].Key)
			})
		})

		t.Run("and it is scrolled past the end", func(t *testing.T) {
			past := ui.LayoutHelp(th, 700, 600, "Keys", everyKey, measure7, top.Max+5)

			t.Run("it should stop at the end", func(t *testing.T) {
				assert.Equal(t, top.Max, past.Skip)
			})
		})

		t.Run("and it is scrolled before the start", func(t *testing.T) {
			before := ui.LayoutHelp(th, 700, 600, "Keys", everyKey, measure7, -3)

			t.Run("it should stop at the start", func(t *testing.T) {
				assert.Equal(t, 0, before.Skip)
			})
		})
	})
}

func TestHelpMoved(t *testing.T) {
	th := ui.NewTheme(1)
	by := city.Point{X: 0, Y: 32}

	t.Run("when a scrolled list is moved", func(t *testing.T) {
		h := ui.LayoutHelp(th, 700, 600, "Keys", everyKey, measure7, 3)
		moved := h.Moved(by)

		t.Run("it should move the panel", func(t *testing.T) {
			assert.Equal(t, h.Rect.Min.Add(by), moved.Rect.Min)
		})

		t.Run("it should move the title", func(t *testing.T) {
			assert.Equal(t, h.Title.At.Add(by), moved.Title.At)
		})

		t.Run("it should move the marks", func(t *testing.T) {
			assert.Equal(t, []city.Point{h.Above.At.Add(by), h.Below.At.Add(by)}, []city.Point{moved.Above.At, moved.Below.At})
		})

		t.Run("it should move every row's band, chip, key and label", func(t *testing.T) {
			var want, got []city.Point
			for i, r := range h.Rows {
				m := moved.Rows[i]
				want = append(want, r.Band.Min.Add(by), r.Chip.Min.Add(by), r.KeyAt.Add(by), r.Label[len(r.Label)-1].At.Add(by))
				got = append(got, m.Band.Min, m.Chip.Min, m.KeyAt, m.Label[len(m.Label)-1].At)
			}
			assert.Equal(t, want, got)
		})

		t.Run("it should leave the list it was moved from where it was", func(t *testing.T) {
			again := ui.LayoutHelp(th, 700, 600, "Keys", everyKey, measure7, 3)
			assert.Equal(t, again.Rows[0].Label[0].At, h.Rows[0].Label[0].At)
		})
	})
}

func TestHelpHit(t *testing.T) {
	th := ui.NewTheme(1)

	t.Run("when the keys are laid out in columns", func(t *testing.T) {
		h := ui.LayoutHelp(th, 1600, 1000, "Keys", everyKey, measure7, 0)
		require.Greater(t, h.Columns, 1)
		last := h.Rows[len(h.Rows)-1]

		t.Run("and the point is on a row in the last column", func(t *testing.T) {
			got, ok := h.Hit(last.Band.Center())

			t.Run("it should find that row's key", func(t *testing.T) {
				assert.True(t, ok && got == last.Key, "got %v", got)
			})
		})
	})
}
