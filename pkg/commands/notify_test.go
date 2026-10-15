package commands_test

import (
	"context"
	"errors"
	"testing"

	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
)

type fakeNotifier struct {
	sent []string
	err  error
}

func (f *fakeNotifier) Notify(title, body string) error {
	f.sent = append(f.sent, title+" | "+body)
	return f.err
}

func snap(states ...state.State) state.Snapshot {
	var sessions []state.Session
	for i, st := range states {
		sessions = append(sessions, state.Session{ID: string(rune('a' + i)), Title: "Job " + string(rune('A'+i)), CWD: "/home/avesta/workspaces/github/mCedar/cinders", Branch: "main", State: st})
	}
	return state.Snapshot{Sessions: sessions}
}

func TestNotify(t *testing.T) {
	t.Run("when the first snapshot already has a session needing you", func(t *testing.T) {
		notifier := &fakeNotifier{}
		n := commands.NewNotify(nil, notifier)
		n.Offer(snap(state.NeedsYou))

		t.Run("it should not notify about what was already there", func(t *testing.T) {
			assert.Empty(t, notifier.sent)
		})
	})

	t.Run("when a working session hands the turn back", func(t *testing.T) {
		notifier := &fakeNotifier{}
		n := commands.NewNotify(nil, notifier)
		n.Offer(snap(state.Working))
		n.Offer(snap(state.NeedsYou))

		t.Run("it should notify once with the title, project and branch", func(t *testing.T) {
			assert.Equal(t, []string{"Job A needs you | cinders · main"}, notifier.sent)
		})

		t.Run("and the next snapshot still needs you", func(t *testing.T) {
			n.Offer(snap(state.NeedsYou))

			t.Run("it should not notify again", func(t *testing.T) {
				assert.Len(t, notifier.sent, 1)
			})
		})

		t.Run("and it goes back to work and hands the turn back again", func(t *testing.T) {
			n.Offer(snap(state.Working))
			n.Offer(snap(state.NeedsYou))

			t.Run("it should notify a second time", func(t *testing.T) {
				assert.Len(t, notifier.sent, 2)
			})
		})
	})

	t.Run("when a new session appears already needing you", func(t *testing.T) {
		notifier := &fakeNotifier{}
		n := commands.NewNotify(nil, notifier)
		n.Offer(snap(state.Working))
		n.Offer(snap(state.Working, state.NeedsYou))

		t.Run("it should notify about the new one", func(t *testing.T) {
			assert.Equal(t, []string{"Job B needs you | cinders · main"}, notifier.sent)
		})
	})

	t.Run("when the session carries a note from a hook", func(t *testing.T) {
		notifier := &fakeNotifier{}
		n := commands.NewNotify(nil, notifier)
		n.Offer(snap(state.Working))
		s := snap(state.NeedsYou)
		s.Sessions[0].Note = "Claude needs your permission to use Bash"

		n.Offer(s)

		t.Run("it should put the note in the body", func(t *testing.T) {
			assert.Equal(t, "Job A needs you | cinders · main\nClaude needs your permission to use Bash", notifier.sent[0])
		})
	})

	t.Run("when the notifier fails", func(t *testing.T) {
		notifier := &fakeNotifier{err: errors.New("no dbus")}
		var got error
		n := commands.NewNotify(nil, notifier)
		n.OnError = func(err error) { got = err }
		n.Offer(snap(state.Working))
		n.Offer(snap(state.NeedsYou))

		t.Run("it should report the failure and carry on", func(t *testing.T) {
			assert.ErrorContains(t, got, "no dbus")
		})
	})

	t.Run("when run on a feed", func(t *testing.T) {
		notifier := &fakeNotifier{}
		feed := func(_ context.Context, offer func(state.Snapshot)) {
			offer(snap(state.Working))
			offer(snap(state.NeedsYou))
		}
		commands.NewNotify(feed, notifier).Run(context.Background())

		t.Run("it should watch every snapshot the feed delivers", func(t *testing.T) {
			assert.Len(t, notifier.sent, 1)
		})
	})
}

func TestNotifierFor(t *testing.T) {
	cases := []struct {
		goos string
		want []string
	}{
		{"linux", []string{"notify-send", "--app-name=botropolis", "--urgency=normal", "Job A", `says "hi"`}},
		{"darwin", []string{"osascript", "-e", `display notification "says \"hi\"" with title "Job A"`}},
	}
	for _, c := range cases {
		t.Run("when the desktop is "+c.goos, func(t *testing.T) {
			t.Run("it should build that desktop's notification command", func(t *testing.T) {
				assert.Equal(t, c.want, commands.NotifierFor(c.goos).Argv("Job A", `says "hi"`))
			})
		})
	}
}
