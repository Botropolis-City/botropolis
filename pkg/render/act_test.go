package render

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

type blockingActor struct {
	release chan struct{}
	reached chan city.Action
	failure error
}

func (a *blockingActor) Do(action city.Action) error {
	a.reached <- action
	<-a.release
	return a.failure
}

func TestActOffTheLoop(t *testing.T) {
	newGame := func(t *testing.T, failure error) (*Game, *blockingActor) {
		actor := &blockingActor{release: make(chan struct{}), reached: make(chan city.Action, 1), failure: failure}
		g := &Game{scene: city.NewScene(city.NewLayout()), actor: actor, scripted: -1, clicked: -1}
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

	for _, action := range []city.Action{
		{Kind: city.ActionCopy, Text: "cd /src/kiln && claude --resume stopped"},
		{Kind: city.ActionCopyPath, Dir: "/src/kiln"},
		{Kind: city.ActionReveal, Dir: "/src/kiln"},
		{Kind: city.ActionOpenPR, Text: "https://example.com/pr/1"},
	} {
		t.Run("when "+string(action.Kind)+" is chosen", func(t *testing.T) {
			t.Run("and the tool behind it does not return", func(t *testing.T) {
				g, _ := newGame(t, nil)

				t.Run("it should return to the loop anyway", func(t *testing.T) {
					assert.True(t, returns(g, action))
				})
			})

			t.Run("and the tool behind it fails", func(t *testing.T) {
				g, actor := newGame(t, errors.New("xclip: signal: killed"))
				close(actor.release)
				g.act(action)
				g.settle()

				t.Run("it should show the failure as a notice once it returns", func(t *testing.T) {
					assert.Equal(t, "xclip: signal: killed", g.notice.Text(timeNow()))
				})
			})
		})
	}

	t.Run("when an action that is not a desktop call is chosen, such as stop", func(t *testing.T) {
		g, actor := newGame(t, errors.New("claude stop: no such job"))
		close(actor.release)
		g.act(city.Action{Kind: city.ActionStop, SessionID: "a"})

		t.Run("it should have its answer by the time act returns", func(t *testing.T) {
			assert.Equal(t, "claude stop: no such job", g.notice.Text(timeNow()))
		})
	})
}
