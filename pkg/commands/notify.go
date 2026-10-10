package commands

import (
	"context"
	"os/exec"
	"path/filepath"

	"botropolis.city/botropolis/pkg/state"
)

type Notifier interface {
	Notify(title, body string) error
}

type NotifySend struct{}

func (NotifySend) Notify(title, body string) error {
	return exec.Command("notify-send", "--app-name=botropolis", "--urgency=normal", title, body).Run()
}

type Notify struct {
	feed     func(ctx context.Context, offer func(state.Snapshot))
	notifier Notifier
	seen     map[string]state.State
	primed   bool
	OnError  func(error)
}

func NewNotify(feed func(ctx context.Context, offer func(state.Snapshot)), notifier Notifier) *Notify {
	return &Notify{feed: feed, notifier: notifier, seen: map[string]state.State{}}
}

func (n *Notify) Run(ctx context.Context) {
	n.feed(ctx, n.Offer)
}

func (n *Notify) Offer(snapshot state.Snapshot) {
	current := map[string]state.State{}
	for _, s := range snapshot.Sessions {
		current[s.ID] = s.State
		if !n.primed {
			continue
		}
		previous, known := n.seen[s.ID]
		if s.State == state.NeedsYou && (!known || previous != state.NeedsYou) {
			if err := n.notifier.Notify(Headline(s), Body(s)); err != nil && n.OnError != nil {
				n.OnError(err)
			}
		}
	}
	n.seen = current
	n.primed = true
}

func Headline(s state.Session) string {
	title := s.Title
	if title == "" {
		title = s.ID
	}
	return title + " needs you"
}

func Body(s state.Session) string {
	body := filepath.Base(s.CWD)
	if s.Branch != "" {
		body += " · " + s.Branch
	}
	if s.Note != "" {
		body += "\n" + s.Note
	}
	return body
}
