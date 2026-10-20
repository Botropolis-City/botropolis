package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"text/tabwriter"
	"time"

	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
)

type SnapshotSource interface {
	Snapshot(direct bool) (state.Snapshot, error)
}

type Status struct {
	source SnapshotSource
}

func NewStatus(source SnapshotSource) *Status {
	return &Status{source: source}
}

func (s *Status) Snapshot(direct bool) (state.Snapshot, error) {
	return s.source.Snapshot(direct)
}

func (s *Status) Run(out io.Writer, direct, all, asJSON bool) error {
	snapshot, err := s.source.Snapshot(direct)
	if err != nil {
		return err
	}
	if !all {
		snapshot = LiveOnly(snapshot)
	}
	if asJSON {
		return RenderJSON(out, snapshot)
	}
	Render(out, snapshot)
	return nil
}

// RenderJSON writes the sessions as they are modelled, which is how the
// daemon already sends them. Item 71: the table has no column that is a
// key, so reconciling it with `claude agents --json` could only be done
// on the title, and two mullet sessions share the title "What time is
// it". That filed a defect against the loader for two sessions it was
// in fact reporting correctly, under titles taken from their transcripts
// rather than the names the CLI gives them.
//
// Encoding state.Session rather than a hand-built row is the point: the
// two cannot drift, and every field the model gains is here without
// anyone remembering to add it. `id` and `pid` are the two the CLI also
// knows, and either one makes the reconciliation mechanical.
// PromisedStatusKeys are the keys of each `status --json` object that
// 1.0.0 promises (Aria, 2026-10-20, item 149). Every other key the
// model gains is emitted too, but may change in any release.
var PromisedStatusKeys = []string{"id", "pid", "harness", "title", "state", "cwd", "branch", "model", "contextPercent", "startedAt", "lastActivity", "prs"}

func RenderJSON(out io.Writer, snapshot state.Snapshot) error {
	sessions := snapshot.Sessions
	if sessions == nil {
		sessions = []state.Session{}
	}
	return json.NewEncoder(out).Encode(sessions)
}

func LiveOnly(snapshot state.Snapshot) state.Snapshot {
	live := snapshot
	live.Sessions = nil
	for _, s := range snapshot.Sessions {
		if s.State != state.Resting {
			live.Sessions = append(live.Sessions, s)
		}
	}
	return live
}

// DoingWidth caps the DOING column so a long command does not push the
// rest of the row off the terminal.
const DoingWidth = 32

func Render(out io.Writer, snapshot state.Snapshot) {
	if len(snapshot.Sessions) == 0 {
		fmt.Fprintln(out, "no sessions")
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STATE\tDOING\tPROJECT\tTITLE\tBRANCH\tMODEL\tCTX\tFRESH/H\tCACHED/H\tSUBS\tIDLE")
	for _, s := range snapshot.Sessions {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d/%d\t%s\n",
			s.State, format.Dash(format.Clip(s.Doing(), DoingWidth)), filepath.Base(s.CWD), s.Title, s.Branch, s.Model,
			format.Percent(s.ContextPercent), format.Tokens(s.FreshTokensPerHour), format.Tokens(s.CacheReadPerHour),
			s.SubagentsInFlight, s.Subagents, format.Age(snapshot.At.Sub(s.LastActivity)))
	}
	_ = w.Flush()
	for _, skipped := range snapshot.Skipped {
		fmt.Fprintf(out, "skipped %s\n", skipped)
	}
}

type DaemonOrDirect struct {
	Home   string
	Socket string
	Probes state.Probes
	Direct func(now time.Time) (state.Snapshot, error)
	Now    func() time.Time
	Notice io.Writer
}

func (d DaemonOrDirect) Snapshot(direct bool) (state.Snapshot, error) {
	if !direct {
		snapshot, err := fromDaemon(d.Socket)
		if err == nil {
			return snapshot, nil
		}
		// A daemon out of sync is refused, never scanned around (item 152).
		var mismatch *proto.Mismatch
		if errors.As(err, &mismatch) {
			return state.Snapshot{}, err
		}
		if d.Notice != nil {
			fmt.Fprintf(d.Notice, "botropolis: botropolisd not reachable at %s (%v); scanning %s directly\n", d.Socket, err, d.Home)
		}
	}
	if d.Direct != nil {
		return d.Direct(d.Now())
	}
	return state.Load(d.Home, d.Probes, d.Now())
}

func fromDaemon(sock string) (state.Snapshot, error) {
	client, err := proto.Dial(sock)
	if err != nil {
		return state.Snapshot{}, err
	}
	defer func() { _ = client.Close() }()
	return client.Snapshot()
}
