package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

func TestBusy(t *testing.T) {
	resume := city.Action{Kind: city.ActionResume, SessionID: "a"}

	t.Run("when a claude action is running", func(t *testing.T) {
		for _, c := range []struct {
			name   string
			second city.Action
			want   bool
		}{
			{"and the same action is asked of the same session", resume, false},
			{"and the same action is asked of another session", city.Action{Kind: city.ActionResume, SessionID: "b"}, true},
			{"and another action is asked of the same session", city.Action{Kind: city.ActionStop, SessionID: "a"}, true},
			{"and a session is started in a directory", city.Action{Kind: city.ActionNew, Dir: "/src/kiln"}, true},
		} {
			t.Run(c.name, func(t *testing.T) {
				var b busy
				b.begin(resume)

				t.Run("it should say whether it may begin", func(t *testing.T) {
					assert.Equal(t, c.want, b.begin(c.second))
				})
			})
		}

		t.Run("and it ends", func(t *testing.T) {
			var b busy
			b.begin(resume)
			b.end(resume)

			t.Run("it should let the same action begin again", func(t *testing.T) {
				assert.True(t, b.begin(resume))
			})
		})
	})

	t.Run("when a session is being started in a directory", func(t *testing.T) {
		start := city.Action{Kind: city.ActionNew, Dir: "/src/kiln"}
		var b busy
		b.begin(start)

		t.Run("and another is asked in the same one", func(t *testing.T) {
			t.Run("it should refuse it", func(t *testing.T) {
				assert.False(t, b.begin(start))
			})
		})
	})

	t.Run("when the city says what a claude action is doing", func(t *testing.T) {
		for _, c := range []struct {
			kind    city.ActionKind
			running string
			already string
		}{
			{city.ActionResume, "resuming Kiln…", "already resuming Kiln"},
			{city.ActionNew, "starting a session in Kiln…", "already starting a session in Kiln"},
			{city.ActionNewSession, "starting a session in Kiln…", "already starting a session in Kiln"},
			{city.ActionFork, "forking Kiln…", "already forking Kiln"},
			{city.ActionStop, "stopping Kiln…", "already stopping Kiln"},
			{city.ActionDemolish, "removing Kiln…", "already removing Kiln"},
			{city.ActionRestore, "restoring the desk…", "already restoring the desk"},
			{city.ActionCloseUp, "closing up…", "already closing up"},
		} {
			t.Run("and it is "+string(c.kind), func(t *testing.T) {
				t.Run("it should say it is running", func(t *testing.T) {
					assert.Equal(t, c.running, running(c.kind, "Kiln"))
				})

				t.Run("it should say it is already running", func(t *testing.T) {
					assert.Equal(t, c.already, already(c.kind, "Kiln"))
				})
			})
		}
	})
}
