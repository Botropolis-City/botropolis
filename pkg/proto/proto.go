package proto

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/state"
)

const (
	OpSnapshot  = "snapshot"
	OpSubscribe = "subscribe"
	OpEvent     = "event"
	// OpEvents asks for the daemon's event log after Since (all of it
	// when Since is zero), newest first.
	OpEvents = "events"
	// OpFind asks the daemon's index for the sessions a query finds.
	// Item 82.
	OpFind = "find"
	// OpPack packs a session up, or unpacks it (item 86).
	OpPack = "pack"
	// OpCloseUp records that you closed up (item 86).
	OpCloseUp = "closeup"

	socketDir  = "botropolis"
	socketName = "botropolis.sock"
)

type Request struct {
	Op    string          `json:"op"`
	Event json.RawMessage `json:"event,omitempty"`
	// Since is the moment an events or subscribe request wants the log
	// from: a subscriber gets the backlog with its first snapshot.
	Since time.Time `json:"since,omitzero"`
	// Find is a find request's query.
	Find *find.Query `json:"find,omitempty"`
	// Pack is a pack request's session and whether to pack it or unpack.
	Pack *Pack `json:"pack,omitempty"`
	// CloseUp is a close-up's open and pinned sessions.
	CloseUp *CloseUp `json:"closeUp,omitempty"`
}

// CloseUp is a request to record a close-up: the sessions open, and the
// ones pinned to start with next time.
type CloseUp struct {
	Open   []string `json:"open"`
	Pinned []string `json:"pinned,omitempty"`
}

// Pack is a request to pack a session up, or with Packed false to unpack
// it.
type Pack struct {
	SessionID string `json:"sessionId"`
	Packed    bool   `json:"packed"`
}

type Response struct {
	Snapshot *state.Snapshot `json:"snapshot,omitempty"`
	Events   []events.Event  `json:"events,omitempty"`
	Found    []find.Hit      `json:"found,omitempty"`
	Error    string          `json:"error,omitempty"`
}

// Update is one delivery to a subscriber: the snapshot, and the events
// the daemon logged since the last delivery (the backlog on the first).
type Update struct {
	Snapshot state.Snapshot
	Events   []events.Event
}

func SocketPath() string {
	home, _ := os.UserHomeDir()
	return SocketPathFor(runtime.GOOS, os.Getenv, home)
}

// SocketPathFor is where the daemon listens and every client dials, so it
// must come out the same in both. On macOS the daemon runs under launchd,
// which hands it neither the shell's TMPDIR nor anything XDG, so the path
// there depends on HOME alone, which launchd does set.
func SocketPathFor(goos string, getenv func(string) string, home string) string {
	if goos == "darwin" {
		return filepath.Join(home, "Library", "Caches", socketDir, socketName)
	}
	if dir := getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, socketDir, socketName)
	}
	tmp := getenv("TMPDIR")
	if tmp == "" {
		tmp = "/tmp"
	}
	return filepath.Join(tmp, socketDir+"-"+strconv.Itoa(os.Getuid()), socketName)
}

func Write(w io.Writer, v any) error {
	return json.NewEncoder(w).Encode(v)
}

func NewDecoder(r io.Reader) *json.Decoder {
	return json.NewDecoder(r)
}
