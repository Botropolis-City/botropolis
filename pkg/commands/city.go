package commands

import (
	"context"
	"errors"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
)

// Feed delivers snapshots: streamed from the daemon when it is there,
// polled from the source directly when it is not. Events, when set,
// receives what happened before each snapshot is offered — the daemon's
// log since Since with the first, then each change as it is logged;
// without a daemon the feed diffs the polled snapshots itself.
type Feed struct {
	Socket  string
	Source  SnapshotSource
	Poll    time.Duration
	Retry   time.Duration
	OnError func(error)
	Since   func() time.Time
	Events  func([]events.Event)
}

func (f Feed) Run(ctx context.Context, offer func(state.Snapshot)) {
	for ctx.Err() == nil {
		if f.subscribe(ctx, offer) {
			continue
		}
		f.poll(ctx, offer)
	}
}

func (f Feed) subscribe(ctx context.Context, offer func(state.Snapshot)) bool {
	client, err := proto.Dial(f.Socket)
	if err != nil {
		return false
	}
	defer func() { _ = client.Close() }()
	updates, err := client.Subscribe(f.since())
	if err != nil {
		return false
	}
	delivered := false
	for {
		select {
		case <-ctx.Done():
			return true
		case update, ok := <-updates:
			if !ok {
				return delivered
			}
			delivered = true
			f.deliver(update.Events)
			offer(update.Snapshot)
		}
	}
}

func (f Feed) since() time.Time {
	if f.Since == nil {
		return time.Time{}
	}
	return f.Since()
}

func (f Feed) deliver(fresh []events.Event) {
	if f.Events != nil {
		f.Events(fresh)
	}
}

func (f Feed) poll(ctx context.Context, offer func(state.Snapshot)) {
	deadline := time.After(f.Retry)
	log := events.NewLog()
	for {
		snapshot, err := f.Source.Snapshot(true)
		if err != nil && f.OnError != nil {
			f.OnError(err)
		}
		if err == nil {
			f.deliver(log.Observe(snapshot, time.Now()))
			offer(snapshot)
		}
		select {
		case <-ctx.Done():
			return
		case <-deadline:
			return
		case <-time.After(f.Poll):
		}
	}
}

type Actor struct {
	Sessions *Sessions
	// GroupStore is the groups' memory a move is written into (item 87).
	GroupStore string
	// Now is the clock a review is stamped with; nil is the wall clock.
	Now func() time.Time
}

func (a Actor) Do(action city.Action) error {
	switch action.Kind {
	case city.ActionAttach:
		return a.Sessions.Attach(action.SessionID)
	case city.ActionResume:
		_, err := a.Sessions.Resume(context.Background(), "", action.SessionID)
		return err
	case city.ActionDemolish:
		return a.Sessions.Remove(context.Background(), action.SessionID)
	case city.ActionStop:
		return a.Sessions.Stop(context.Background(), action.SessionID)
	case city.ActionNew:
		_, err := a.Sessions.New(context.Background(), action.Dir, "")
		return err
	case city.ActionReveal:
		return a.Sessions.Reveal(action.Dir)
	case city.ActionCopyPath:
		return a.Sessions.Copy(context.Background(), action.Dir)
	case city.ActionCopy:
		return a.Sessions.Copy(context.Background(), action.Text)
	case city.ActionMove:
		return a.move(action.SessionID, action.Group)
	case city.ActionReviewGroups:
		return a.review(action.Review)
	}
	return nil
}

// move writes a session's move into the groups' memory; the daemon sees
// the file change and groups again with the move honoured.
func (a Actor) move(session, to string) error {
	if a.GroupStore == "" {
		return errNoGroups
	}
	memory, err := group.Load(a.GroupStore)
	if err != nil {
		return err
	}
	return group.Save(a.GroupStore, memory.WithMove(session, to))
}

// review writes the first run's confirmed groups into the groups'
// memory, alongside what it already holds.
func (a Actor) review(r *group.Review) error {
	if r == nil {
		return nil
	}
	if a.GroupStore == "" {
		return errNoGroups
	}
	now := time.Now
	if a.Now != nil {
		now = a.Now
	}
	memory, err := group.Load(a.GroupStore)
	if err != nil {
		return err
	}
	return group.Save(a.GroupStore, memory.Confirm(*r, now()))
}

var errNoGroups = errors.New("groups are kept only for your own home, not for one --home points at")
