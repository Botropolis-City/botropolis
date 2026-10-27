package commands_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/testing/helpers"
)

func TestActorRestore(t *testing.T) {
	t.Run("when Restore resumes several sessions", func(t *testing.T) {
		t.Run("and one of them cannot be resumed", func(t *testing.T) {
			const lost = "deadbeef-0000-0000-0000-000000000000"
			home := helpers.NewHome(t)
			home.Transcript(sid, cwd, helpers.UserPrompt(sid, cwd, "2026-09-16T19:50:00.000Z"))
			ctl := &fakeController{id: "08b1d655"}
			actor := commands.Actor{Sessions: commands.NewSessions(home.Path, ctl)}
			err := actor.Do(city.Action{Kind: city.ActionRestore, Open: []string{lost, sid}})

			t.Run("it should still resume the others", func(t *testing.T) {
				assert.Equal(t, []string{"resume " + cwd + " " + sid}, ctl.calls)
			})

			t.Run("it should name the one that failed", func(t *testing.T) {
				assert.ErrorContains(t, err, lost)
			})
		})
	})
}
