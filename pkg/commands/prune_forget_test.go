package commands_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/control"
	"botropolis.city/botropolis/pkg/state"
)

type removeFails struct{ *fakeController }

func (r removeFails) Remove(context.Context, string) error { return errors.New("rm failed") }

// Item 110: prune forgets the statistics of each session it removes.
func TestPruneForgets(t *testing.T) {
	now := time.Date(2026, time.October, 18, 8, 0, 0, 0, time.UTC)
	session := "11111111-0000-0000-0000-000000000000"
	agents := []control.Agent{{ID: "11111111", SessionID: session, Kind: "background", StartedAt: now.Add(-10 * 24 * time.Hour).UnixMilli()}}
	snapshot := state.Snapshot{At: now, Sessions: []state.Session{{ID: session, State: state.Resting, LastActivity: now.Add(-9 * 24 * time.Hour)}}}
	week := 7 * 24 * time.Hour

	prune := func(ctl commands.Controller, dryRun bool) []string {
		var forgotten []string
		sessions := commands.NewSessions(t.TempDir(), ctl).WithForget(func(id string) error {
			forgotten = append(forgotten, id)
			return nil
		})
		_, _ = sessions.Prune(context.Background(), snapshot, week, now, dryRun)
		return forgotten
	}

	t.Run("when prune removes a session, and stats are on", func(t *testing.T) {
		forgotten := prune(&fakeController{agents: agents}, false)

		t.Run("it should forget that session", func(t *testing.T) {
			assert.Equal(t, []string{session}, forgotten)
		})
	})

	t.Run("when it is a dry run", func(t *testing.T) {
		forgotten := prune(&fakeController{agents: agents}, true)

		t.Run("it should forget nothing", func(t *testing.T) {
			assert.Empty(t, forgotten)
		})
	})

	t.Run("when the remove fails", func(t *testing.T) {
		forgotten := prune(removeFails{&fakeController{agents: agents}}, false)

		t.Run("it should not forget it", func(t *testing.T) {
			assert.Empty(t, forgotten)
		})
	})

	t.Run("when stats are off", func(t *testing.T) {
		ctl := &fakeController{agents: agents}
		pruned, err := commands.NewSessions(t.TempDir(), ctl).Prune(context.Background(), snapshot, week, now, false)

		t.Run("it should prune without forgetting", func(t *testing.T) {
			assert.Equal(t, []any{1, nil}, []any{len(pruned), err})
		})
	})
}
