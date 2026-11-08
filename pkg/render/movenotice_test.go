package render

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

type failingActor struct{ err error }

func (a failingActor) Do(city.Action) error { return a.err }

// Item 365: a move says "moved" only once a snapshot shows it, since
// the write is to a file the daemon may not read, and says so when it
// does not take.
func TestMoveNotice(t *testing.T) {
	at := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)
	groups := []state.Group{{ID: "g1", Name: "PROJ-123 · headers"}, {ID: "g2", Name: "LOG-88 · buffer"}}
	sessions := func(group string) []state.Session {
		return []state.Session{
			{ID: "a", CWD: "/w/lib", State: state.Working, Group: group},
			{ID: "b", CWD: "/w/api", State: state.Working, Group: "g2"},
		}
	}
	newGame := func(t *testing.T, actor Actor) *Game {
		t.Helper()
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(state.Snapshot{At: at, Groups: groups, Sessions: sessions("g1")})
		return &Game{scene: s, actor: actor, scripted: -1, clicked: -1}
	}
	shows := func(g *Game, gs []state.Group, group string, after time.Duration) {
		g.scene.SetSnapshot(state.Snapshot{At: at, Groups: gs, Sessions: sessions(group)})
		g.checkMove(at.Add(after))
	}
	notice := func(g *Game) string { return g.notice.Text(timeNow()) }

	t.Run("when a session is moved into a group", func(t *testing.T) {
		t.Run("and the write fails", func(t *testing.T) {
			g := newGame(t, failingActor{errors.New("groups.json: read-only file system")})
			g.moveAt("a", "g2", at)

			t.Run("it should show the failure", func(t *testing.T) {
				assert.Equal(t, "groups.json: read-only file system", notice(g))
			})
		})

		t.Run("and the write succeeds", func(t *testing.T) {
			g := newGame(t, &recordingActor{})
			g.moveAt("a", "g2", at)

			t.Run("it should say it is moving there, not that it moved", func(t *testing.T) {
				assert.Equal(t, "moving to LOG-88 · buffer…", notice(g))
			})

			t.Run("and a snapshot shows the session in that group", func(t *testing.T) {
				g := newGame(t, &recordingActor{})
				g.moveAt("a", "g2", at)
				shows(g, groups, "g2", time.Second)

				t.Run("it should say it moved there", func(t *testing.T) {
					assert.Equal(t, "moved to LOG-88 · buffer", notice(g))
				})

				t.Run("and the wait then runs out", func(t *testing.T) {
					g.checkMove(at.Add(time.Minute))

					t.Run("it should never say the move has not taken", func(t *testing.T) {
						assert.Equal(t, "moved to LOG-88 · buffer", notice(g))
					})
				})
			})

			t.Run("and no snapshot shows it within the wait", func(t *testing.T) {
				g := newGame(t, &recordingActor{})
				g.moveAt("a", "g2", at)
				shows(g, groups, "g1", moveWait)

				t.Run("it should say the move has not taken", func(t *testing.T) {
					assert.Equal(t, "the move to LOG-88 · buffer has not taken", notice(g))
				})
			})
		})
	})

	t.Run("when a session is moved into a new group", func(t *testing.T) {
		t.Run("and a snapshot shows it in a group by that name", func(t *testing.T) {
			g := newGame(t, &recordingActor{})
			g.moveIntoNewAt("a", "Gateway", at)
			shows(g, append(groups, state.Group{ID: "g-new", Name: "Gateway"}), "g-new", time.Second)

			t.Run("it should say it moved to a new group", func(t *testing.T) {
				assert.Equal(t, "moved to Gateway, a new group", notice(g))
			})
		})
	})

	t.Run("when a session is moved to no group", func(t *testing.T) {
		t.Run("and a snapshot shows it in none", func(t *testing.T) {
			g := newGame(t, &recordingActor{})
			g.moveAt("a", "", at)
			shows(g, groups, "", time.Second)

			t.Run("it should say it moved to no group", func(t *testing.T) {
				assert.Equal(t, "moved to "+noGroup, notice(g))
			})
		})
	})

	t.Run("when a second move follows before the first takes", func(t *testing.T) {
		t.Run("and the first's group shows", func(t *testing.T) {
			g := newGame(t, &recordingActor{})
			g.moveAt("a", "g2", at)
			g.moveAt("a", "", at)
			shows(g, groups, "g2", time.Second)

			t.Run("it should not say it moved there", func(t *testing.T) {
				assert.Equal(t, "moving to "+noGroup+"…", notice(g))
			})
		})
	})
}
