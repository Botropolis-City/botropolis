package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/control"
	"botropolis.city/botropolis/pkg/state"
)

// Item 144: every way of demolishing a session says so once the remove
// has succeeded, by its session id, so the ledger can forget it.
func TestDemolished(t *testing.T) {
	session := "11111111-0000-0000-0000-000000000000"
	resumed := "22222222-0000-0000-0000-000000000000"
	agents := []control.Agent{{ID: "abcdef12", SessionID: resumed, Kind: "background"}}
	demolishing := func(ctl commands.Controller) (*commands.Sessions, *[]string) {
		var told []string
		sessions := commands.NewSessions(t.TempDir(), ctl).WithDemolished(func(id string) error {
			told = append(told, id)
			return nil
		})
		return sessions, &told
	}

	t.Run("when a session is removed by its session id", func(t *testing.T) {
		sessions, told := demolishing(&fakeController{})
		_ = sessions.Remove(context.Background(), session)

		t.Run("it should say that session was demolished", func(t *testing.T) {
			assert.Equal(t, []string{session}, *told)
		})
	})

	t.Run("when a session is removed by its job id", func(t *testing.T) {
		sessions, told := demolishing(&fakeController{agents: agents})
		_ = sessions.Remove(context.Background(), "abcdef12")

		t.Run("it should say the job's session was demolished", func(t *testing.T) {
			assert.Equal(t, []string{resumed}, *told)
		})
	})

	t.Run("when the remove fails", func(t *testing.T) {
		sessions, told := demolishing(removeFails{&fakeController{}})
		_ = sessions.Remove(context.Background(), session)

		t.Run("it should say nothing was demolished", func(t *testing.T) {
			assert.Empty(t, *told)
		})
	})

	t.Run("when the map demolishes a building", func(t *testing.T) {
		sessions, told := demolishing(&fakeController{})
		_ = commands.Actor{Sessions: sessions}.Do(city.Action{Kind: city.ActionDemolish, SessionID: session})

		t.Run("it should say that session was demolished", func(t *testing.T) {
			assert.Equal(t, []string{session}, *told)
		})
	})

	t.Run("when prune removes a session", func(t *testing.T) {
		now := time.Date(2026, time.October, 19, 8, 0, 0, 0, time.UTC)
		old := []control.Agent{{ID: "11111111", SessionID: session, Kind: "background", StartedAt: now.Add(-10 * 24 * time.Hour).UnixMilli()}}
		snapshot := state.Snapshot{At: now, Sessions: []state.Session{{ID: session, State: state.Resting, LastActivity: now.Add(-9 * 24 * time.Hour)}}}

		t.Run("it should say that session was demolished", func(t *testing.T) {
			sessions, told := demolishing(&fakeController{agents: old})
			_, _ = sessions.Prune(context.Background(), snapshot, 7*24*time.Hour, now, false)
			assert.Equal(t, []string{session}, *told)
		})

		t.Run("and it is a dry run", func(t *testing.T) {
			t.Run("it should say nothing was demolished", func(t *testing.T) {
				sessions, told := demolishing(&fakeController{agents: old})
				_, _ = sessions.Prune(context.Background(), snapshot, 7*24*time.Hour, now, true)
				assert.Empty(t, *told)
			})
		})
	})
}
