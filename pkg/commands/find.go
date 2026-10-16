package commands

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
)

// Find finds sessions by what was said in them: from the daemon's index
// when it is listening, else from an index of its own, built from the
// same files and dropped when the command ends. Item 82.
type Find struct {
	socket string
	home   string
	source SnapshotSource
	now    func() time.Time

	// index is kept between searches, for a city with no daemon, so each
	// search reads only what changed since the last.
	mu    sync.Mutex
	index *find.Index
}

func NewFind(socket, home string, source SnapshotSource, now func() time.Time) *Find {
	return &Find{socket: socket, home: home, source: source, now: now}
}

// Search returns what a query finds, ranked.
func (f *Find) Search(q find.Query) ([]find.Hit, error) {
	if client, err := proto.Dial(f.socket); err == nil {
		defer func() { _ = client.Close() }()
		return client.Find(q)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.index == nil {
		f.index = find.New(f.home)
	}
	ix := f.index
	if err := ix.Refresh(); err != nil {
		return nil, err
	}
	if f.source != nil {
		if snapshot, err := f.source.Snapshot(true); err == nil {
			q.Live = map[string]state.State{}
			for _, s := range snapshot.Sessions {
				q.Live[s.ID] = s.State
			}
		}
	}
	return ix.Find(q), nil
}

// Run prints each session found: its state when it is running, how
// long since it was active, its directory and branch and the start of
// its id, then the passage that matched.
func (f *Find) Run(out io.Writer, q find.Query) error {
	hits, err := f.Search(q)
	if err != nil {
		return err
	}
	if len(hits) == 0 {
		_, err := fmt.Fprintf(out, "nothing found for %q\n", q.Words)
		return err
	}
	for _, h := range hits {
		var head []string
		if h.State != "" && h.State != state.Parked {
			head = append(head, string(h.State))
		}
		head = append(head, format.Age(f.now().Sub(h.At))+" ago")
		where := h.Dir
		if h.Branch != "" {
			where += " (" + h.Branch + ")"
		}
		head = append(head, where, shortID(h.SessionID))
		if h.Gone {
			head = append(head, "conversation deleted")
		}
		passage := strings.Join(strings.Fields(h.Passage), " ")
		if _, err := fmt.Fprintf(out, "%s\n    %s: %s\n", strings.Join(head, "  "), kindName(h.Kind), format.Clip(passage, 100)); err != nil {
			return err
		}
	}
	return nil
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func kindName(k claude.SaidKind) string {
	switch k {
	case claude.SaidPrompt:
		return "you"
	case claude.SaidRecap:
		return "recap"
	case claude.SaidReply:
		return "claude"
	case claude.SaidCommand:
		return "ran"
	case claude.SaidPath:
		return "file"
	case claude.SaidTitle:
		return "title"
	}
	return "output"
}
