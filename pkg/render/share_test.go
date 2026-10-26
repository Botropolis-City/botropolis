package render

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 168: sharing mode shows the city as it would be shared, with
// whatever identifies the person hidden, and takes pictures of it.

const shareWidth, shareHeight = 1280, 800

type shareRig struct {
	g      *Game
	actor  *recordingActor
	screen *ebiten.Image
	drawn  []string
	dir    string
	opened []string
}

// shareArt is the art every rig draws with, read once: it is only read
// from, and reading it is most of what a rig costs.
var shareKits = sync.OnceValues(func() (*kits, error) { return loadKits() })

func newShareRig(t *testing.T) *shareRig {
	t.Helper()
	at := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	th := ui.NewTheme(1)
	f, err := newFaces(th)
	require.NoError(t, err)
	k, err := shareKits()
	require.NoError(t, err)
	s := city.NewScene(city.NewLayout())
	s.SetClock(func() time.Time { return at }, time.UTC)
	s.SetProjection(city.Isometric)
	s.SetInstant(true)
	s.Resize(shareWidth, shareHeight)
	s.SetSnapshot(helpers.ShareSnapshot(at))
	r := &shareRig{actor: &recordingActor{}, screen: ebiten.NewImage(shareWidth, shareHeight), dir: t.TempDir()}
	g := NewGame(s, r.actor, th, f, nil)
	g.kits = k
	g.driven = true
	g.scripted, g.clicked = -1, -1
	g.textSeen = func(s string) { r.drawn = append(r.drawn, s) }
	g.grab = func(screen *ebiten.Image) image.Image {
		r.drawn = append(r.drawn, grabbed)
		return image.NewRGBA(screen.Bounds())
	}
	g.shareDir = func(userDirKind) string { return r.dir }
	g.openFolder = func(dir string) error {
		r.opened = append(r.opened, dir)
		return nil
	}
	r.g = g
	r.layout()
	return r
}

// grabbed marks, among the strings drawn, the moment the frame was read.
const grabbed = "\x00grabbed"

// draw paints one frame and lays out the next, and returns the strings
// it drew. Ebitengine lays out, then updates, then draws, so a key
// pressed between two draws reaches the screen's size one frame late.
func (r *shareRig) draw() []string {
	r.drawn = nil
	r.g.ticks++
	r.g.Draw(r.screen)
	r.layout()
	return r.drawn
}

// layout asks the game for its screen in a window of the rig's size,
// and gives it one of the size it asks for.
func (r *shareRig) layout() {
	w, h := r.g.LayoutF(shareWidth, shareHeight)
	if b := r.screen.Bounds(); b.Dx() != int(w) || b.Dy() != int(h) {
		r.screen = ebiten.NewImage(int(w), int(h))
	}
}

func (r *shareRig) press(t *testing.T, key ebiten.Key, shift bool) {
	t.Helper()
	r.g.scripted, r.g.scriptedShift = key, shift
	require.NoError(t, r.g.handleKeys())
	r.g.scripted, r.g.scriptedShift = -1, false
}

// openEverything opens what a person might have open when they decide
// to share: the sidebar, a building's card, the reference.
func (r *shareRig) openEverything(t *testing.T) {
	t.Helper()
	r.g.sidebar = true
	require.True(t, r.g.scene.Select("aaaa-1"))
	r.g.openReference()
}

func leaks(drawn []string) []string {
	var out []string
	for _, s := range drawn {
		for _, secret := range helpers.ShareSecrets {
			if strings.Contains(s, secret) || strings.Contains(s, secret[:8]) {
				out = append(out, s)
			}
		}
	}
	return out
}

func figures(drawn []string) []string {
	var out []string
	for _, s := range drawn {
		if shareFigure.MatchString(s) {
			out = append(out, s)
		}
	}
	return out
}

func TestSharing(t *testing.T) {
	t.Run("when the city is drawn normally", func(t *testing.T) {
		r := newShareRig(t)
		r.openEverything(t)
		drawn := r.draw()

		t.Run("it should draw the fixture's names, so the sharing tests can see a leak", func(t *testing.T) {
			assert.NotEmpty(t, leaks(drawn))
		})

		t.Run("it should draw a figure, so the sharing tests can see one", func(t *testing.T) {
			assert.NotEmpty(t, figures(drawn))
		})
	})

	t.Run("when sharing, with the sidebar, a card and the reference open", func(t *testing.T) {
		r := newShareRig(t)
		r.openEverything(t)
		r.press(t, ebiten.KeyS, true)
		drawn := r.draw()

		t.Run("it should be sharing", func(t *testing.T) {
			assert.True(t, r.g.sharing())
		})

		t.Run("it should draw no name from the fixture", func(t *testing.T) {
			assert.Empty(t, leaks(drawn))
		})

		t.Run("it should draw no spend, token or usage figure", func(t *testing.T) {
			assert.Empty(t, figures(drawn))
		})

		t.Run("it should need the guard for nothing", func(t *testing.T) {
			assert.Equal(t, 0, r.g.share.guard.refused, "refused: %v", r.g.share.guard.refusals)
		})

		t.Run("it should draw the sharing bar", func(t *testing.T) {
			assert.True(t, slices.ContainsFunc(drawn, func(s string) bool { return strings.Contains(s, "Sharing") }), "drawn: %q", drawn)
		})

		t.Run("it should wear the veil", func(t *testing.T) {
			assert.True(t, r.g.scene.Veiled())
		})
	})

	t.Run("when the guard is given", func(t *testing.T) {
		guard := newShareGuard(city.Words(helpers.ShareSnapshot(time.Now())))
		for _, c := range []struct {
			text  string
			allow bool
		}{
			{"Qzalphatitle", false},
			{"fix Qzalphatitle now", false},
			{"Qzalphati…", false},
			{"Qzhomeowner", false},
			{"Qzalphaproject", false},
			{"Qzlastprompt please", false},
			{"$12.34", false},
			{"12.34 USD", false},
			{"1.2M tok", false},
			{"340k tokens", false},
			{"60%", false},
			{"District A", true},
			{"Session 3", true},
			{"Tower 2", true},
			{"City Hall", true},
			{"Sharing", true},
		} {
			verb := "refuse"
			if c.allow {
				verb = "allow"
			}
			t.Run(c.text+", it should "+verb+" it", func(t *testing.T) {
				assert.Equal(t, c.allow, guard.allows(c.text))
			})
		}
	})

	for _, c := range []struct {
		name string
		draw func(g *Game, screen *ebiten.Image, s string)
	}{
		{"as text", func(g *Game, screen *ebiten.Image, s string) {
			g.text(screen, city.Point{}, s, ui.Small, colorText)
		}},
		{"as a sign", func(g *Game, screen *ebiten.Image, s string) {
			g.sign(screen, ui.Sign{Text: s, Scale: 1}, colorText)
		}},
	} {
		t.Run("when sharing and a name the veil missed is drawn "+c.name, func(t *testing.T) {
			r := newShareRig(t)
			r.press(t, ebiten.KeyS, true)
			r.drawn = nil
			c.draw(r.g, r.screen, "Qzalphatitle")

			t.Run("it should not be drawn", func(t *testing.T) {
				assert.Empty(t, r.drawn)
			})

			t.Run("it should be counted as refused", func(t *testing.T) {
				assert.Equal(t, 1, r.g.share.guard.refused)
			})
		})
	}

	t.Run("when sharing ends", func(t *testing.T) {
		r := newShareRig(t)
		r.openEverything(t)
		before := r.draw()
		reference := r.g.referenceOpen()
		r.press(t, ebiten.KeyS, true)
		r.draw()
		r.press(t, ebiten.KeyEscape, false)
		after := r.draw()

		t.Run("it should not be sharing", func(t *testing.T) {
			assert.False(t, r.g.sharing())
		})

		t.Run("it should draw what it drew before", func(t *testing.T) {
			assert.Equal(t, before, after)
		})

		t.Run("it should take the veil off", func(t *testing.T) {
			assert.False(t, r.g.scene.Veiled())
		})

		t.Run("it should keep the sidebar open", func(t *testing.T) {
			assert.True(t, r.g.sidebar)
		})

		t.Run("it should keep the selection", func(t *testing.T) {
			require.NotNil(t, r.g.scene.Selected())
			assert.Equal(t, "aaaa-1", r.g.scene.Selected().Session.ID)
		})

		t.Run("it should keep the reference open", func(t *testing.T) {
			assert.Same(t, reference, r.g.referenceOpen())
		})
	})

	t.Run("when shift+s is pressed again while sharing", func(t *testing.T) {
		r := newShareRig(t)
		r.press(t, ebiten.KeyS, true)
		r.press(t, ebiten.KeyS, true)

		t.Run("it should stop sharing", func(t *testing.T) {
			assert.False(t, r.g.sharing())
		})
	})

	for _, c := range []struct {
		name  string
		key   ebiten.Key
		shift bool
	}{
		{"d", ebiten.KeyD, false},
		{"enter", ebiten.KeyEnter, false},
		{"c", ebiten.KeyC, false},
		{"shift+p", ebiten.KeyP, true},
		{"s", ebiten.KeyS, false},
		{"t", ebiten.KeyT, false},
		{"x", ebiten.KeyX, false},
		{"q", ebiten.KeyQ, false},
		{"/", ebiten.KeySlash, false},
	} {
		t.Run("when sharing and "+c.name+" is pressed", func(t *testing.T) {
			r := newShareRig(t)
			require.True(t, r.g.scene.Select("aaaa-1"))
			r.press(t, ebiten.KeyS, true)
			overlays := len(r.g.overlays)
			r.press(t, c.key, c.shift)
			r.press(t, c.key, c.shift)

			t.Run("it should ask nothing of the sessions", func(t *testing.T) {
				assert.Empty(t, r.actor.done)
			})

			t.Run("it should open nothing", func(t *testing.T) {
				assert.Equal(t, []any{overlays, false, false, false}, []any{len(r.g.overlays), r.g.settingsOpen, r.g.timeline, r.g.breakdown})
			})

			t.Run("it should still be sharing", func(t *testing.T) {
				assert.True(t, r.g.sharing())
			})
		})
	}
}

func TestSharePicture(t *testing.T) {
	t.Run("when sharing and p is pressed", func(t *testing.T) {
		r := newShareRig(t)
		r.press(t, ebiten.KeyS, true)
		r.press(t, ebiten.KeyP, false)
		drawn := r.draw()
		files, err := filepath.Glob(filepath.Join(r.dir, "botropolis-share-*.png"))
		require.NoError(t, err)

		t.Run("it should save one picture in the pictures folder", func(t *testing.T) {
			assert.Len(t, files, 1)
		})

		t.Run("it should save a PNG of the window's size", func(t *testing.T) {
			require.Len(t, files, 1)
			f, err := os.Open(files[0])
			require.NoError(t, err)
			defer func() { _ = f.Close() }()
			cfg, err := png.DecodeConfig(f)
			require.NoError(t, err)
			assert.Equal(t, [2]int{shareWidth, shareHeight}, [2]int{cfg.Width, cfg.Height})
		})

		t.Run("it should read the frame before drawing the sharing bar", func(t *testing.T) {
			at := slices.Index(drawn, grabbed)
			bar := slices.IndexFunc(drawn, func(s string) bool { return strings.Contains(s, "Sharing") })
			require.GreaterOrEqual(t, at, 0, "the frame was never read")
			assert.Less(t, at, bar)
		})

		t.Run("it should say where it saved", func(t *testing.T) {
			require.Len(t, files, 1)
			assert.Contains(t, r.g.notice.Text(timeNow()), files[0])
		})

		t.Run("and o is pressed", func(t *testing.T) {
			r.press(t, ebiten.KeyO, false)

			t.Run("it should open the folder it saved in", func(t *testing.T) {
				assert.Equal(t, []string{r.dir}, r.opened)
			})
		})
	})

	t.Run("when not sharing and p is pressed", func(t *testing.T) {
		r := newShareRig(t)
		r.press(t, ebiten.KeyP, false)

		t.Run("it should leave p as it was, a screenshot of the whole window", func(t *testing.T) {
			assert.NotEmpty(t, r.g.snap)
		})
	})
}

func TestUserDir(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(home, ".config")
	require.NoError(t, os.MkdirAll(config, 0o755))
	mkdir := func(t *testing.T, parts ...string) string {
		t.Helper()
		dir := filepath.Join(append([]string{home}, parts...)...)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		return dir
	}
	dirs := func(t *testing.T, body string) {
		t.Helper()
		path := filepath.Join(config, "user-dirs.dirs")
		require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
		t.Cleanup(func() { _ = os.Remove(path) })
	}

	t.Run("when Linux names a pictures folder in user-dirs.dirs", func(t *testing.T) {
		want := mkdir(t, "Bilder")
		dirs(t, "# written by xdg-user-dirs-update\nXDG_PICTURES_DIR=\"$HOME/Bilder\"\nXDG_VIDEOS_DIR=\"$HOME/Videos\"\n")

		t.Run("it should save pictures there", func(t *testing.T) {
			assert.Equal(t, want, userDir(home, "linux", config, picturesDir))
		})
	})

	t.Run("when Linux names a folder that does not exist", func(t *testing.T) {
		dirs(t, "XDG_VIDEOS_DIR=\"$HOME/Filme\"\n")

		t.Run("it should save in home", func(t *testing.T) {
			assert.Equal(t, home, userDir(home, "linux", config, videosDir))
		})
	})

	t.Run("when Linux has no user-dirs.dirs and a Pictures folder", func(t *testing.T) {
		want := mkdir(t, "Pictures")

		t.Run("it should save pictures there", func(t *testing.T) {
			assert.Equal(t, want, userDir(home, "linux", config, picturesDir))
		})
	})

	t.Run("when Linux has neither", func(t *testing.T) {
		t.Run("it should save in home", func(t *testing.T) {
			assert.Equal(t, home, userDir(home, "linux", config, videosDir))
		})
	})

	for kind, folder := range map[userDirKind]string{picturesDir: "Pictures", videosDir: "Movies"} {
		t.Run("when macOS has a "+folder+" folder", func(t *testing.T) {
			want := mkdir(t, folder)

			t.Run("it should save there", func(t *testing.T) {
				assert.Equal(t, want, userDir(home, "darwin", config, kind))
			})
		})
	}
}

// Sharing must leave nothing behind built from the veiled names: a panel
// that opens itself while sharing would show them afterwards, and the
// first run's review saves the group names it shows.
func TestSharingLeavesNothingBehind(t *testing.T) {
	t.Run("when the first run is owed while sharing", func(t *testing.T) {
		r := newShareRig(t)
		r.g.driven = false
		owed := helpers.ShareSnapshot(time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC))
		owed.GroupsReviewed = false
		r.g.scene.SetSnapshot(owed)
		r.press(t, ebiten.KeyS, true)
		r.g.maybeFirstRun()
		_, opened := r.g.overlays.top().(*firstRun)

		t.Run("it should not open while sharing", func(t *testing.T) {
			assert.False(t, opened)
		})

		t.Run("and sharing ends", func(t *testing.T) {
			r.press(t, ebiten.KeyEscape, false)
			r.g.maybeFirstRun()
			review, ok := r.g.overlays.top().(*firstRun)
			require.True(t, ok, "the first run should open once sharing ends")

			t.Run("it should review the groups by their own names", func(t *testing.T) {
				require.NotEmpty(t, review.review.Rows)
				assert.Equal(t, "Qzgroupname", review.review.Rows[0].Name)
			})
		})
	})

	t.Run("when the journal is owed while sharing", func(t *testing.T) {
		r := newShareRig(t)
		r.g.awaySince, r.g.journalHours = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), 1
		r.press(t, ebiten.KeyS, true)
		r.g.maybeJournal()
		_, opened := r.g.overlays.top().(*journal)

		t.Run("it should not open while sharing", func(t *testing.T) {
			assert.False(t, opened)
		})

		t.Run("and sharing ends, it should open then", func(t *testing.T) {
			r.press(t, ebiten.KeyEscape, false)
			r.g.maybeJournal()
			_, opened := r.g.overlays.top().(*journal)
			assert.True(t, opened)
		})
	})

	t.Run("when sharing begins and ends", func(t *testing.T) {
		r := newShareRig(t)
		r.draw()
		before, err := json.Marshal(r.g.scene.Layout())
		require.NoError(t, err)
		r.press(t, ebiten.KeyS, true)
		r.draw()
		r.press(t, ebiten.KeyEscape, false)
		after, err := json.Marshal(r.g.scene.Layout())
		require.NoError(t, err)

		t.Run("it should leave the layout the map saves as it was", func(t *testing.T) {
			assert.JSONEq(t, string(before), string(after))
		})

		t.Run("it should ask nothing of the sessions", func(t *testing.T) {
			assert.Empty(t, r.actor.done)
		})
	})
}

func TestSharingPointer(t *testing.T) {
	building := func(r *shareRig) city.Point {
		b := r.g.scene.City().Buildings()[0]
		return r.g.scene.Camera().WorldToScreen(b.Rect.Center())
	}

	t.Run("when sharing and a building is clicked", func(t *testing.T) {
		r := newShareRig(t)
		r.g.openReference()
		r.press(t, ebiten.KeyS, true)
		at := building(r)
		r.g.sharePointer(at, 0, true, true, false)
		r.g.sharePointer(at, 0, false, false, true)

		t.Run("it should select nothing", func(t *testing.T) {
			assert.Nil(t, r.g.scene.Selected())
		})

		t.Run("it should leave the reference behind it as it was", func(t *testing.T) {
			assert.NotNil(t, r.g.referenceOpen())
		})

		t.Run("it should ask nothing of the sessions", func(t *testing.T) {
			assert.Empty(t, r.actor.done)
		})
	})

	t.Run("when sharing and the map is dragged", func(t *testing.T) {
		r := newShareRig(t)
		r.press(t, ebiten.KeyS, true)
		from := city.Point{X: 400, Y: 400}
		before := r.g.scene.Camera().WorldToScreen(city.Point{})
		r.g.sharePointer(from, 0, true, true, false)
		r.g.sharePointer(from.Add(city.Point{X: 50, Y: 20}), 0, false, true, false)
		r.g.sharePointer(from.Add(city.Point{X: 50, Y: 20}), 0, false, false, true)

		t.Run("it should pan the map", func(t *testing.T) {
			assert.NotEqual(t, before, r.g.scene.Camera().WorldToScreen(city.Point{}))
		})
	})

	for _, button := range []struct {
		name   string
		button ebiten.MouseButton
	}{{"right", ebiten.MouseButtonRight}, {"middle", ebiten.MouseButtonMiddle}} {
		t.Run("when sharing over the reference and the map is dragged with the "+button.name+" button", func(t *testing.T) {
			r := newShareRig(t)
			r.g.openReference()
			r.press(t, ebiten.KeyS, true)
			from := city.Point{X: 400, Y: 400}
			to := from.Add(city.Point{X: 50, Y: 20})
			before := r.g.scene.Camera().WorldToScreen(city.Point{})
			r.g.buttonPan(from, pressOf(button.button))
			r.g.buttonPan(to, holdOf(button.button))
			r.g.buttonPan(to, releaseOf(button.button))

			t.Run("it should pan the map", func(t *testing.T) {
				assert.NotEqual(t, before, r.g.scene.Camera().WorldToScreen(city.Point{}))
			})
		})
	}
}
