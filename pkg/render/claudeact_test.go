package render

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/state"
)

// Item 286: the actions that run claude wait beside the city's loop, as
// item 284's desktop calls do, and say what they are doing meanwhile.
func TestActClaudeBesideTheLoop(t *testing.T) {
	const fixer = "0898d7e4-f2cb-45fe-833e-1921c1f4bcaa"
	newGame := func(t *testing.T, failure error) (*Game, *blockingActor) {
		t.Helper()
		actor := &blockingActor{release: make(chan struct{}), reached: make(chan city.Action, 2), failure: failure}
		scene := city.NewScene(city.NewLayout())
		scene.Resize(800, 600)
		scene.SetSnapshot(state.Snapshot{At: time.Now(), Sessions: []state.Session{{ID: fixer, Title: "Fix the queue", CWD: "/src/kiln", State: state.Resting}}})
		g := &Game{scene: scene, actor: actor, scripted: -1, clicked: -1}
		t.Cleanup(func() {
			select {
			case <-actor.release:
			default:
				close(actor.release)
			}
			g.settle()
		})
		return g, actor
	}
	returns := func(g *Game, action city.Action) bool {
		done := make(chan struct{})
		go func() {
			g.act(action)
			close(done)
		}()
		select {
		case <-done:
			return true
		case <-time.After(time.Second):
			return false
		}
	}

	for _, c := range []struct {
		action  city.Action
		running string
	}{
		{city.Action{Kind: city.ActionResume, SessionID: fixer}, "resuming Fix the queue…"},
		{city.Action{Kind: city.ActionNew, Dir: "/src/kiln"}, "starting a session in kiln…"},
		{city.Action{Kind: city.ActionNewSession, Dir: "/src/kiln", Prompt: "go"}, "starting a session in kiln…"},
		{city.Action{Kind: city.ActionFork, SessionID: fixer, Dir: "/src/kiln"}, "forking Fix the queue…"},
		{city.Action{Kind: city.ActionStop, SessionID: fixer}, "stopping Fix the queue…"},
		{city.Action{Kind: city.ActionDemolish, SessionID: fixer}, "removing Fix the queue…"},
		{city.Action{Kind: city.ActionRestore, Open: []string{fixer}}, "restoring the desk…"},
	} {
		t.Run("when "+string(c.action.Kind)+" is chosen", func(t *testing.T) {
			t.Run("and claude behind it does not return", func(t *testing.T) {
				g, _ := newGame(t, nil)
				back := returns(g, c.action)

				t.Run("it should return to the loop anyway", func(t *testing.T) {
					assert.True(t, back)
				})

				t.Run("it should say what it is doing", func(t *testing.T) {
					assert.Equal(t, c.running, g.notice.Text(timeNow()))
				})

				t.Run("and it is chosen again", func(t *testing.T) {
					g.act(c.action)

					t.Run("it should say it already is", func(t *testing.T) {
						assert.Equal(t, "already "+c.running[:len(c.running)-len("…")], g.notice.Text(timeNow()))
					})
				})
			})

			t.Run("and it is chosen twice while claude runs", func(t *testing.T) {
				g, actor := newGame(t, nil)
				g.act(c.action)
				<-actor.reached
				g.act(c.action)
				close(actor.release)
				g.settle()

				t.Run("it should run claude once", func(t *testing.T) {
					assert.Empty(t, actor.reached)
				})
			})

			t.Run("and claude fails", func(t *testing.T) {
				g, actor := newGame(t, errors.New("claude did not answer within 2m"))
				close(actor.release)
				g.act(c.action)
				g.settle()

				t.Run("it should show the failure as a notice once it returns", func(t *testing.T) {
					assert.Equal(t, "claude did not answer within 2m", g.notice.Text(timeNow()))
				})
			})

			t.Run("and claude succeeds", func(t *testing.T) {
				g, actor := newGame(t, nil)
				close(actor.release)
				g.act(c.action)
				g.settle()

				t.Run("it should clear what it was doing", func(t *testing.T) {
					assert.Empty(t, g.notice.Text(timeNow()))
				})
			})
		})
	}

	for _, action := range []city.Action{
		{Kind: city.ActionMove, SessionID: fixer, Group: "g1"},
		{Kind: city.ActionMute, Text: "queue"},
		{Kind: city.ActionReviewGroups, Review: &group.Review{}},
	} {
		t.Run("when "+string(action.Kind)+", which writes the groups file in order, is chosen", func(t *testing.T) {
			g, actor := newGame(t, errors.New("groups file is read-only"))
			close(actor.release)
			g.act(action)

			t.Run("it should have its answer by the time act returns", func(t *testing.T) {
				assert.Equal(t, "groups file is read-only", g.notice.Text(timeNow()))
			})
		})
	}

}
