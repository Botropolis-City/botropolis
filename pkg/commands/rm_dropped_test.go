package commands_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/control"
)

type stubClaude struct{ rm error }

func (s stubClaude) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	if len(args) > 0 && args[0] == "rm" {
		return "", s.rm
	}
	return "", nil
}
func (stubClaude) Start(string, string, ...string) error                   { return nil }
func (stubClaude) Interactive(string, ...string) error                     { return nil }
func (stubClaude) Feed(context.Context, string, string, ...string) error   { return nil }
func (stubClaude) Read(context.Context, string, ...string) (string, error) { return "", nil }

// Item 341: Claude Code 2.1.310's claude stop drops the job outright, so
// claude rm then has no job for it, and rm still forgets the session.
func TestRemoveWhenTheCLIHasNoJob(t *testing.T) {
	session := "abcdef12-0000-0000-0000-000000000000"
	removing := func(rm error) ([]string, string, error) {
		var told []string
		var said strings.Builder
		ctl := control.New(stubClaude{rm: rm}, control.Native(), func(string) string { return "" }, func(string) bool { return false })
		sessions := commands.NewSessions(t.TempDir(), ctl).WithNotices(&said).WithDemolished(func(id string) error {
			told = append(told, id)
			return nil
		})
		err := sessions.Remove(context.Background(), session)
		return told, said.String(), err
	}

	t.Run("when claude rm says no job matches", func(t *testing.T) {
		told, said, err := removing(errors.New("claude rm abcdef12: exit status 1: No job matching 'abcdef12'"))

		t.Run("it should succeed", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should forget the session", func(t *testing.T) {
			assert.Equal(t, []string{session}, told)
		})

		t.Run("it should say Claude Code had already dropped it", func(t *testing.T) {
			assert.Contains(t, said, "Claude Code had already dropped")
		})
	})

	t.Run("when claude rm fails any other way", func(t *testing.T) {
		told, _, err := removing(errors.New("claude rm abcdef12: exit status 1: permission denied"))

		t.Run("it should fail", func(t *testing.T) {
			assert.Error(t, err)
		})

		t.Run("it should forget nothing", func(t *testing.T) {
			assert.Empty(t, told)
		})
	})
}
