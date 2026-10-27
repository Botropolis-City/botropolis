package render

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/script"
	"botropolis.city/botropolis/pkg/state"
)

func TestPasteLine(t *testing.T) {
	for _, c := range []struct{ name, in, want string }{
		{"two lines", "a\nb", "a b"},
		{"Windows line ends", "a\r\nb", "a b"},
		{"old Mac line ends", "a\rb", "a b"},
		{"a trailing newline, as pwd | xclip leaves", "/w/dir\n", "/w/dir"},
		{"a tab", "a\tb", "a b"},
		{"another control character", "a\x00b", "ab"},
		{"plain text", "/w/api", "/w/api"},
	} {
		t.Run("when the clipboard holds "+c.name, func(t *testing.T) {
			t.Run("it should paste "+c.want, func(t *testing.T) {
				assert.Equal(t, c.want, pasteLine(c.in))
			})
		})
	}
}

func TestPasteChords(t *testing.T) {
	for _, c := range []struct {
		goos  string
		chord config.Chord
		want  bool
	}{
		{"linux", config.Chord{Ctrl: true, Key: "v"}, true},
		{"linux", config.Chord{Shift: true, Key: "insert"}, true},
		{"linux", config.Chord{Cmd: true, Key: "v"}, false},
		{"darwin", config.Chord{Cmd: true, Key: "v"}, true},
		{"darwin", config.Chord{Ctrl: true, Key: "v"}, false},
	} {
		t.Run("when the city runs on "+c.goos+" and "+c.chord.String()+" is pressed", func(t *testing.T) {
			t.Run("it should paste only where that is the convention", func(t *testing.T) {
				assert.Equal(t, c.want, containsChord(pasteChords(c.goos), c.chord))
			})
		})
	}
}

func containsChord(chords []config.Chord, c config.Chord) bool {
	for _, have := range chords {
		if have == c {
			return true
		}
	}
	return false
}

type fakeClipboard struct {
	text    string
	failure error
	release chan struct{}
	reads   int
}

func (c *fakeClipboard) paste(context.Context) (string, error) {
	c.reads++
	if c.release != nil {
		<-c.release
	}
	return c.text, c.failure
}

func TestPaste(t *testing.T) {
	now := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	newGame := func(t *testing.T, clip *fakeClipboard) *Game {
		t.Helper()
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(state.Snapshot{At: now,
			Groups: []state.Group{{ID: "g1", Name: "PROJ-1"}},
			Sessions: []state.Session{
				{ID: "a", CWD: "/w/lib", State: state.NeedsYou, Group: "g1", LastActivity: now.Add(-time.Hour)},
			}})
		g := &Game{scene: s, actor: &recordingActor{}, scripted: -1, clicked: -1}
		if clip != nil {
			g.paste = clip.paste
		}
		t.Cleanup(func() {
			if clip != nil && clip.release != nil {
				select {
				case <-clip.release:
				default:
					close(clip.release)
				}
			}
			g.settle()
		})
		return g
	}
	press := func(t *testing.T, g *Game, key ebiten.Key) {
		t.Helper()
		g.scripted = key
		require.NoError(t, g.handleKeys())
		g.scripted = -1
	}
	typed := func(t *testing.T, g *Game, s string) {
		t.Helper()
		for _, r := range s {
			g.scriptedRune = r
			require.NoError(t, g.handleKeys())
		}
		g.scriptedRune = 0
	}
	chords := pasteChords(runtime.GOOS)
	chord := chords[0]
	pressChord := func(t *testing.T, g *Game, c config.Chord) {
		t.Helper()
		key, ok := keyByName(c.Key)
		require.True(t, ok)
		g.scriptedCtrl, g.scriptedCmd, g.scriptedShift = c.Ctrl, c.Cmd, c.Shift
		press(t, g, key)
		g.scriptedCtrl, g.scriptedCmd, g.scriptedShift = false, false, false
	}
	pressPaste := func(t *testing.T, g *Game) {
		t.Helper()
		pressChord(t, g, chord)
	}
	// pasted presses the platform's paste chord, lets the clipboard
	// answer, and takes the frame after, which is the one the answer
	// arrives on.
	pasted := func(t *testing.T, g *Game) {
		t.Helper()
		pressPaste(t, g)
		g.settle()
		require.NoError(t, g.handleKeys())
	}

	fields := []struct {
		name string
		open func(t *testing.T, g *Game)
		text func(g *Game) string
	}{
		{"the finder",
			func(t *testing.T, g *Game) { g.openFinder() },
			func(g *Game) string { return g.overlays.top().(*finder).field.query() }},
		{"New session's directory",
			func(t *testing.T, g *Game) { g.openNewSession() },
			func(g *Game) string { return g.newSession().fields[fieldDir].query() }},
		{"New session's prompt",
			func(t *testing.T, g *Game) { g.openNewSession(); press(t, g, ebiten.KeyArrowDown) },
			func(g *Game) string { return g.newSession().fields[fieldPrompt].query() }},
		{"New session's group name",
			func(t *testing.T, g *Game) {
				g.openNewSession()
				press(t, g, ebiten.KeyArrowDown)
				press(t, g, ebiten.KeyArrowDown)
			},
			func(g *Game) string { return g.newSession().fields[fieldGroup].query() }},
		{"the picker's new group name",
			func(t *testing.T, g *Game) {
				g.act(city.Action{Kind: city.ActionMoveToGroup, SessionID: "a"})
				press(t, g, ebiten.KeyEnter)
			},
			func(g *Game) string { return g.picker().name.query() }},
		{"triage's snooze",
			func(t *testing.T, g *Game) {
				press(t, g, ebiten.KeyW)
				press(t, g, ebiten.KeyS)
				for range g.triage().snoozing.choices {
					press(t, g, ebiten.KeyArrowDown)
				}
			},
			func(g *Game) string { return g.triage().snoozing.typed.query() }},
	}

	for _, f := range fields {
		t.Run("when "+f.name+" is focused", func(t *testing.T) {
			t.Run("and "+chord.String()+" is pressed with /w/api on the clipboard", func(t *testing.T) {
				g := newGame(t, &fakeClipboard{text: "/w/api"})
				f.open(t, g)
				typed(t, g, "x")
				pasted(t, g)

				t.Run("it should insert the clipboard at the caret", func(t *testing.T) {
					assert.Equal(t, "x/w/api", f.text(g))
				})
			})

			t.Run("and the clipboard holds two lines", func(t *testing.T) {
				g := newGame(t, &fakeClipboard{text: "fix the\nheaders\n"})
				f.open(t, g)
				pasted(t, g)

				t.Run("it should paste them as one line", func(t *testing.T) {
					assert.Equal(t, "fix the headers", f.text(g))
				})
			})
		})
	}

	t.Run("when New session's prompt is focused and "+chord.String()+" is pressed", func(t *testing.T) {
		g := newGame(t, &fakeClipboard{text: "/w/api"})
		g.openNewSession()
		press(t, g, ebiten.KeyArrowDown)
		pasted(t, g)

		t.Run("it should leave the directory and the group name as they were", func(t *testing.T) {
			n := g.newSession()
			assert.Equal(t, [2]string{"", ""}, [2]string{n.fields[fieldDir].query(), n.fields[fieldGroup].query()})
		})
	})

	for _, c := range chords {
		t.Run("when "+c.String()+" is pressed in the finder", func(t *testing.T) {
			g := newGame(t, &fakeClipboard{text: "/w/api"})
			g.openFinder()
			pressChord(t, g, c)
			g.settle()
			require.NoError(t, g.handleKeys())

			t.Run("it should paste", func(t *testing.T) {
				assert.Equal(t, "/w/api", g.overlays.top().(*finder).field.query())
			})
		})
	}

	t.Run("when the clipboard tool does not answer", func(t *testing.T) {
		clip := &fakeClipboard{release: make(chan struct{}), failure: errors.New("the clipboard did not answer within 2s (xclip)")}
		g := newGame(t, clip)
		g.openFinder()
		returned := make(chan struct{})
		go func() {
			pressPaste(t, g)
			close(returned)
		}()

		t.Run("it should not hold up the loop", func(t *testing.T) {
			select {
			case <-returned:
			case <-time.After(time.Second):
				t.Fatal("handleKeys waited on the clipboard")
			}
		})

		t.Run("and then fails", func(t *testing.T) {
			close(clip.release)
			g.settle()
			require.NoError(t, g.handleKeys())

			t.Run("it should say why", func(t *testing.T) {
				assert.Equal(t, "the clipboard did not answer within 2s (xclip)", g.notice.Text(timeNow()))
			})

			t.Run("it should leave the field as it was", func(t *testing.T) {
				assert.Empty(t, g.overlays.top().(*finder).field.query())
			})
		})
	})

	t.Run("when "+chord.String()+" is pressed again before the clipboard answers", func(t *testing.T) {
		clip := &fakeClipboard{text: "/w/api", release: make(chan struct{})}
		g := newGame(t, clip)
		g.openFinder()
		pressPaste(t, g)
		pressPaste(t, g)
		close(clip.release)
		g.settle()

		t.Run("it should read the clipboard once", func(t *testing.T) {
			assert.Equal(t, 1, clip.reads)
		})
	})

	t.Run("when the panel closes before the clipboard answers", func(t *testing.T) {
		clip := &fakeClipboard{text: "/w/api", release: make(chan struct{})}
		g := newGame(t, clip)
		g.openFinder()
		pressPaste(t, g)
		press(t, g, ebiten.KeyEscape)
		g.openNewSession()
		close(clip.release)
		g.settle()
		require.NoError(t, g.handleKeys())

		t.Run("it should paste into no other field", func(t *testing.T) {
			assert.Empty(t, g.newSession().fields[fieldDir].query())
		})
	})

	t.Run("when no clipboard is wired, as in a scripted city", func(t *testing.T) {
		g := newGame(t, nil)
		g.openFinder()
		pasted(t, g)

		t.Run("it should say pasting is not available here", func(t *testing.T) {
			assert.Equal(t, "pasting is not available here", g.notice.Text(timeNow()))
		})
	})
}

func TestClipboardFor(t *testing.T) {
	real := func(context.Context) (string, error) { return "what Aria copied", nil }
	for _, c := range []struct {
		name string
		opts Options
		want bool
	}{
		{"the city is opened by hand", Options{}, true},
		{"the city takes a screenshot", Options{Screenshot: "frame.png"}, false},
		{"the city records a film", Options{Record: "frames"}, false},
		{"the city is scripted by --keys", Options{Keys: []string{"ctrl+v"}}, false},
		{"the city plays a clip's script", Options{Script: []script.Cue{{}}}, false},
	} {
		t.Run("when "+c.name, func(t *testing.T) {
			c.opts.Paste = real
			should := "it should read the clipboard"
			if !c.want {
				should = "it should not read the clipboard"
			}

			t.Run(should, func(t *testing.T) {
				assert.Equal(t, c.want, clipboardFor(c.opts) != nil)
			})
		})
	}
}
