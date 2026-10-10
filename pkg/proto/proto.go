package proto

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/state"
)

const (
	OpSnapshot  = "snapshot"
	OpSubscribe = "subscribe"
	OpEvent     = "event"
	// OpEvents asks for the daemon's event log after Since (all of it
	// when Since is zero), newest first.
	OpEvents = "events"

	socketDir  = "botropolis"
	socketName = "botropolis.sock"
)

type Request struct {
	Op    string          `json:"op"`
	Event json.RawMessage `json:"event,omitempty"`
	// Since is the moment an events or subscribe request wants the log
	// from: a subscriber gets the backlog with its first snapshot.
	Since time.Time `json:"since,omitzero"`
}

type Response struct {
	Snapshot *state.Snapshot `json:"snapshot,omitempty"`
	Events   []events.Event  `json:"events,omitempty"`
	Error    string          `json:"error,omitempty"`
}

// Update is one delivery to a subscriber: the snapshot, and the events
// the daemon logged since the last delivery (the backlog on the first).
type Update struct {
	Snapshot state.Snapshot
	Events   []events.Event
}

func SocketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, socketDir, socketName)
	}
	return filepath.Join(os.TempDir(), socketDir+"-"+strconv.Itoa(os.Getuid()), socketName)
}

func Write(w io.Writer, v any) error {
	return json.NewEncoder(w).Encode(v)
}

func NewDecoder(r io.Reader) *json.Decoder {
	return json.NewDecoder(r)
}
