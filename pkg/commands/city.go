package commands

import (
	"context"
	"errors"
	"fmt"
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
	// Mismatch hears of a daemon of another release or protocol. The
	// feed then offers nothing, and never scans around it, until a
	// daemon of this build's answers (item 152).
	Mismatch func(*proto.Mismatch)
}

// MismatchRetry is how often a feed asks again of a daemon out of sync,
// so a restarted one is seen soon after it answers.
const MismatchRetry = time.Second

func (f Feed) Run(ctx context.Context, offer func(state.Snapshot)) {
	// since is the newest event handed on, so a feed that attaches
	// again is given what it missed and nothing twice.
	since := f.since()
	for ctx.Err() == nil {
		delivered, mismatch := f.subscribe(ctx, offer, &since)
		switch {
		case mismatch != nil:
			if f.Mismatch != nil {
				f.Mismatch(mismatch)
			}
			select {
			case <-ctx.Done():
			case <-time.After(min(f.Retry, MismatchRetry)):
			}
		case delivered:
		default:
			f.poll(ctx, offer)
		}
	}
}

// RunRefusing runs the feed until its context ends, or until it meets a
// daemon out of sync, which it returns: a command refuses rather than
// waits (item 152).
func (f Feed) RunRefusing(ctx context.Context, offer func(state.Snapshot)) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	f.Mismatch = func(m *proto.Mismatch) { cancel(m) }
	f.Run(ctx, offer)
	var mismatch *proto.Mismatch
	if cause := context.Cause(ctx); errors.As(cause, &mismatch) {
		return mismatch
	}
	return nil
}

func (f Feed) subscribe(ctx context.Context, offer func(state.Snapshot), since *time.Time) (bool, *proto.Mismatch) {
	client, err := proto.Dial(f.Socket)
	if err != nil {
		return false, nil
	}
	defer func() { _ = client.Close() }()
	updates, err := client.Subscribe(*since)
	var mismatch *proto.Mismatch
	if errors.As(err, &mismatch) {
		return false, mismatch
	}
	if err != nil {
		return false, nil
	}
	delivered := false
	for {
		select {
		case <-ctx.Done():
			return true, nil
		case update, ok := <-updates:
			if !ok {
				return delivered, nil
			}
			delivered = true
			f.take(since, update.Events)
			offer(update.Snapshot)
		}
	}
}

// take hands fresh events on, and moves since to the newest of them.
func (f Feed) take(since *time.Time, fresh []events.Event) {
	for _, e := range fresh {
		if e.At.After(*since) {
			*since = e.At
		}
	}
	f.deliver(fresh)
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
	// Pack asks the daemon to pack a session up or unpack it; nil when
	// there is no daemon to ask (item 86).
	Pack func(id string, packed bool) error
	// CloseUp asks the daemon to record a close-up; nil when there is
	// no daemon to ask (item 86).
	CloseUp func(open, pinned []string) error
}

var errNoDaemonCloseUp = errors.New("closing up needs the daemon, which keeps the record of what was open; start botropolisd")

var errNoDaemon = errors.New("packing up needs the daemon, which keeps the ledger of what is packed; start botropolisd")

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
	case city.ActionFork:
		_, err := a.Sessions.Fork(context.Background(), action.Dir, action.SessionID)
		return err
	case city.ActionOpenPR:
		return a.Sessions.Reveal(action.Text)
	case city.ActionCopyPath:
		return a.Sessions.Copy(context.Background(), action.Dir)
	case city.ActionCopy:
		return a.Sessions.Copy(context.Background(), action.Text)
	case city.ActionMove:
		return a.move(action.SessionID, action.Group, action.GroupName)
	case city.ActionMute:
		return a.mute(action.Text)
	case city.ActionCloseUp:
		return a.closeUp(action)
	case city.ActionRestore:
		var errs []error
		for _, id := range action.Open {
			if _, err := a.Sessions.ResumeBackground(context.Background(), id); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", id, err))
			}
		}
		return errors.Join(errs...)
	case city.ActionPack, city.ActionUnpack:
		if a.Pack == nil {
			return errNoDaemon
		}
		return a.Pack(action.SessionID, action.Kind == city.ActionPack)
	case city.ActionReviewGroups:
		return a.review(action.Review)
	case city.ActionNewSession:
		return a.newSession(action)
	}
	return nil
}

// move writes a session's move into the groups' memory; the daemon sees
// the file change and groups again with the move honoured. A name, from
// the picker's new group…, makes the group it moves into (item 288).
func (a Actor) move(session, to, name string) error {
	if a.GroupStore == "" {
		return errNoGroups
	}
	memory, err := group.Load(a.GroupStore)
	if err != nil {
		return err
	}
	if name != "" {
		memory, to = memory.WithNewGroup(name, session)
	}
	return group.Save(a.GroupStore, memory.WithMove(session, to))
}

// mute writes a muted word into the groups' memory; the daemon sees the
// file change and groups again without it (item 90).
func (a Actor) mute(word string) error {
	if a.GroupStore == "" {
		return errNoGroups
	}
	memory, err := group.Load(a.GroupStore)
	if err != nil {
		return err
	}
	return group.Save(a.GroupStore, memory.WithMuted(word))
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

// newSession starts a session in the background and records the group it
// was started in by the job it runs as, which is the surest sign there
// is that it belongs there. A new group is stored under the name typed.
func (a Actor) newSession(action city.Action) error {
	job, err := a.Sessions.New(context.Background(), action.Dir, action.Prompt)
	if err != nil {
		return err
	}
	if a.GroupStore == "" || job == "" || (action.Group == "" && action.GroupName == "") {
		return nil
	}
	memory, err := group.Load(a.GroupStore)
	if err != nil {
		return err
	}
	id := action.Group
	if id == "" {
		memory, id = memory.WithNewGroup(action.GroupName, job)
	}
	if memory.StartedFrom == nil {
		memory.StartedFrom = map[string]string{}
	}
	memory.StartedFrom[job] = id
	return group.Save(a.GroupStore, memory)
}

// closeUp records what is open with the daemon, then pauses only the
// sessions you marked pause, with claude stop, which keeps the
// conversation; everything else is left running. Recording comes first:
// a close-up that paused sessions and then failed to say which were open
// would lose the desk it was meant to keep.
func (a Actor) closeUp(action city.Action) error {
	if a.CloseUp == nil {
		return errNoDaemonCloseUp
	}
	if err := a.CloseUp(action.Open, action.Pinned); err != nil {
		return err
	}
	var errs []error
	for _, id := range action.Pause {
		if err := a.Sessions.Stop(context.Background(), id); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}
