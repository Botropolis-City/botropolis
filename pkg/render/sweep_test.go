package render

import (
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 333: City Hall packs up everything quiet in one move, saying how
// many before and after.
func TestQuietSweep(t *testing.T) {
	now := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	resting := func(id, cwd string, quiet time.Duration) state.Session {
		return state.Session{ID: id, CWD: cwd, Title: id, State: state.Resting, LastActivity: now.Add(-quiet)}
	}
	everyKind := func() []state.Session {
		grouped := resting("grouped", "/w/api", 10*day)
		grouped.Group = "g1"
		open := resting("open", "/w/cli", 10*day)
		open.Alive = true
		working := resting("working", "/w/lib", 10*day)
		working.State = state.Working
		gone := resting("gone", "/w/old", 10*day)
		gone.ConversationGone = true
		return []state.Session{
			resting("old", "/w/site", 10*day), resting("older", "/w/docs", 40*day),
			resting("fresh", "/w/tool", day), grouped, open, working, gone,
		}
	}
	type call struct {
		ids      []string
		quietFor time.Duration
	}
	newGame := func(sessions []state.Session, sweep func([]string, time.Duration) (int, error)) (*Game, *[]call) {
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(state.Snapshot{At: now, Sessions: sessions})
		calls := &[]call{}
		g := &Game{scene: s, scripted: -1, clicked: -1, quietFor: 7 * day}
		g.sweep = func(ids []string, quietFor time.Duration) (int, error) {
			*calls = append(*calls, call{ids, quietFor})
			return sweep(ids, quietFor)
		}
		return g, calls
	}
	packsAll := func(ids []string, _ time.Duration) (int, error) { return len(ids), nil }
	press := func(t *testing.T, g *Game, key ebiten.Key) {
		t.Helper()
		g.scripted = key
		require.NoError(t, g.handleKeys())
		g.scripted = -1
		g.acting.Wait()
	}
	board := func(t *testing.T, g *Game) {
		t.Helper()
		press(t, g, ebiten.KeyG)
		require.NotNil(t, g.board(), "the board should be open")
	}

	t.Run("when sessions of every kind are on the map", func(t *testing.T) {
		g, _ := newGame(everyKind(), packsAll)

		t.Run("it should count only the loose sessions quiet for quiet_days that the map shows", func(t *testing.T) {
			assert.ElementsMatch(t, []string{"old", "older"}, g.quietIDs())
		})

		t.Run("it should say the count on City Hall before anything is packed", func(t *testing.T) {
			assert.Equal(t, "Pack up 2 quiet sessions", quietLabel(len(g.quietIDs())))
		})
	})

	t.Run("when the board is open and p is pressed", func(t *testing.T) {
		g, calls := newGame(everyKind(), packsAll)
		board(t, g)
		press(t, g, ebiten.KeyP)

		t.Run("it should ask once for the counted sessions at quiet_days", func(t *testing.T) {
			require.Len(t, *calls, 1)
			got := (*calls)[0]
			assert.Equal(t, call{[]string{"old", "older"}, 7 * day}, call{sorted(got.ids), got.quietFor})
		})

		t.Run("it should say how many it packed and where they went", func(t *testing.T) {
			assert.Equal(t, "packed up 2 quiet sessions; unpack any from the depot", g.notice.Text(timeNow()))
		})
	})

	t.Run("when the daemon packs fewer than were counted", func(t *testing.T) {
		g, _ := newGame(everyKind(), func([]string, time.Duration) (int, error) { return 1, nil })
		board(t, g)
		press(t, g, ebiten.KeyP)

		t.Run("it should say the number the daemon packed", func(t *testing.T) {
			assert.Equal(t, "packed up 1 quiet session; unpack any from the depot", g.notice.Text(timeNow()))
		})
	})

	t.Run("when nothing is quiet and p is pressed", func(t *testing.T) {
		g, calls := newGame([]state.Session{resting("fresh", "/w/tool", day)}, packsAll)
		board(t, g)
		press(t, g, ebiten.KeyP)

		t.Run("it should not ask the daemon", func(t *testing.T) {
			assert.Empty(t, *calls)
		})

		t.Run("it should say nothing is quiet, and for how long it looked", func(t *testing.T) {
			assert.Equal(t, "nothing in no group has been quiet for 7 days", g.notice.Text(timeNow()))
		})
	})

	t.Run("when the sweep is refused", func(t *testing.T) {
		g, _ := newGame(everyKind(), func([]string, time.Duration) (int, error) {
			return 0, errors.New("the quiet sweep packs up only your own home's sessions")
		})
		board(t, g)
		press(t, g, ebiten.KeyP)

		t.Run("it should say why", func(t *testing.T) {
			assert.Equal(t, "the quiet sweep packs up only your own home's sessions", g.notice.Text(timeNow()))
		})
	})

	t.Run("when the board's quiet button is clicked", func(t *testing.T) {
		g, calls := newGame(everyKind(), packsAll)
		board(t, g)
		g.board().quietButton = city.RectAt(10, 10, 100, 20)
		g.board().click(g, city.Point{X: 20, Y: 15})
		g.acting.Wait()

		t.Run("it should sweep, as p does", func(t *testing.T) {
			assert.Len(t, *calls, 1)
		})
	})

	t.Run("when the board is closed and p is pressed", func(t *testing.T) {
		g, calls := newGame(everyKind(), packsAll)
		press(t, g, ebiten.KeyP)

		t.Run("it should not sweep", func(t *testing.T) {
			assert.Empty(t, *calls)
		})
	})

	t.Run("when the quiet days setting changes", func(t *testing.T) {
		g, _ := newGame(everyKind(), packsAll)
		g.applySetting(quietDaysSetting("30"))

		t.Run("it should count against the new threshold", func(t *testing.T) {
			assert.Equal(t, []string{"older"}, g.quietIDs())
		})
	})

	for _, c := range []struct {
		n    int
		want string
	}{{0, "Nothing quiet to pack up"}, {1, "Pack up 1 quiet session"}, {212, "Pack up 212 quiet sessions"}} {
		t.Run("when the count is "+itoa(c.n), func(t *testing.T) {
			t.Run("it should read "+c.want, func(t *testing.T) {
				assert.Equal(t, c.want, quietLabel(c.n))
			})
		})
	}
}

func sorted(ids []string) []string {
	out := slices.Clone(ids)
	slices.Sort(out)
	return out
}

func quietDaysSetting(days string) ui.Setting {
	return ui.Setting{Key: config.KeyQuietDays, Label: "quiet days", Value: days}
}

func itoa(n int) string { return strconv.Itoa(n) }
