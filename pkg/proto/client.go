package proto

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/measure"
	"botropolis.city/botropolis/pkg/state"
)

const dialTimeout = 500 * time.Millisecond

type Client struct {
	conn   net.Conn
	reader *bufio.Reader
	// sock is where this client reached its daemon, which a mismatch
	// names (item 152).
	sock string
	// mirror is the daemon's view as a subscription's deltas build it
	// (item 201).
	mirror Mirror
}

func (c *Client) send(r Request) error {
	r.Peer = Self()
	return Write(c.conn, r)
}

func Dial(sock string) (*Client, error) {
	conn, err := net.DialTimeout("unix", sock, dialTimeout)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, reader: bufio.NewReader(conn), sock: sock}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Snapshot() (state.Snapshot, error) {
	if err := c.send(Request{Op: OpSnapshot}); err != nil {
		return state.Snapshot{}, err
	}
	update, err := c.readUpdate()
	return update.Snapshot, err
}

// Subscribe streams every change; the first update carries the current
// snapshot and the log since the moment given (all of it when zero).
// The first is read before it returns, so a daemon out of sync is
// refused here rather than ending the stream unexplained (item 152).
func (c *Client) Subscribe(since time.Time) (<-chan Update, error) {
	if err := c.send(Request{Op: OpSubscribe, Since: since}); err != nil {
		return nil, err
	}
	first, err := c.readUpdate()
	if err != nil {
		return nil, err
	}
	updates := make(chan Update, 1)
	updates <- first
	go func() {
		defer close(updates)
		for {
			update, err := c.readUpdate()
			if err != nil {
				return
			}
			updates <- update
		}
	}()
	return updates, nil
}

// Events is the daemon's log after a moment (all of it when zero),
// newest first.
func (c *Client) Events(since time.Time) ([]events.Event, error) {
	if err := c.send(Request{Op: OpEvents, Since: since}); err != nil {
		return nil, err
	}
	response, err := c.readResponse()
	if err != nil {
		return nil, err
	}
	return response.Events, nil
}

// Find asks the daemon's index for the sessions a query finds.
func (c *Client) Find(q find.Query) ([]find.Hit, error) {
	if err := c.send(Request{Op: OpFind, Find: &q}); err != nil {
		return nil, err
	}
	response, err := c.readResponse()
	if err != nil {
		return nil, err
	}
	return response.Found, nil
}

// CloseUp records a close-up with the daemon.
func (c *Client) CloseUp(open, pinned []string) error {
	if err := c.send(Request{Op: OpCloseUp, CloseUp: &CloseUp{Open: open, Pinned: pinned}}); err != nil {
		return err
	}
	_, err := c.readResponse()
	return err
}

// Pack packs a session up, or unpacks it.
func (c *Client) Pack(id string, packed bool) error {
	if err := c.send(Request{Op: OpPack, Pack: &Pack{SessionID: id, Packed: packed}}); err != nil {
		return err
	}
	_, err := c.readResponse()
	return err
}

// Forget has the daemon forget a demolished session (item 144).
func (c *Client) Forget(id string) error {
	if err := c.send(Request{Op: OpForget, Forget: id}); err != nil {
		return err
	}
	_, err := c.readResponse()
	return err
}

func (c *Client) readUpdate() (Update, error) {
	response, err := c.readResponse()
	if err != nil {
		return Update{}, err
	}
	var snapshot state.Snapshot
	reading.Add(measure.Thread(func() { snapshot, err = c.mirror.Take(response) }))
	if err != nil {
		return Update{}, err
	}
	return Update{Snapshot: snapshot, Events: response.Events}, nil
}

// reading is what every client in this process has spent decoding what
// the daemon sent, waiting for it aside (item 201), counted on the
// thread that decoded it (item 209).
var reading measure.Total

// Reading is what this process has spent decoding the daemon's
// responses and applying its deltas, which a city that draws nothing
// is gated on.
func Reading() measure.Cost {
	return reading.Cost()
}

// received is the bytes every client in this process has read from the
// daemon, which a hidden city is gated on apart from everything else it
// reads (item 209).
var received atomic.Int64

// Received is the bytes this process has read from the daemon.
func Received() int64 {
	return received.Load()
}

// decode reads one response, a line, and decodes it, timing only the
// decoding.
func (c *Client) decode(v any) error {
	line, err := c.reader.ReadBytes('\n')
	received.Add(int64(len(line)))
	if err != nil {
		return err
	}
	reading.Add(measure.Thread(func() { err = json.Unmarshal(line, v) }))
	return err
}

func (c *Client) readResponse() (Response, error) {
	var response Response
	if err := c.decode(&response); err != nil {
		return Response{}, err
	}
	if err := Compare(response.Peer, Self()); err != nil {
		mismatch := err.(*Mismatch)
		mismatch.Socket = c.sock
		return Response{}, mismatch
	}
	if response.Error != "" {
		return Response{}, errors.New(response.Error)
	}
	return response, nil
}

// Ask asks the daemon on sock what it is, without comparing it with
// this build. A daemon that does not know the question still says what
// it is alongside its refusal, and one from before the exchange says
// nothing (item 152).
func Ask(sock string) (Peer, error) {
	peer, _, err := AskStore(sock)
	return peer, err
}

// AskStore asks the daemon on sock what it is and what became of its
// statistics store, which is nil from a daemon that does not say
// (item 190).
func AskStore(sock string) (Peer, *state.StatsStore, error) {
	client, err := Dial(sock)
	if err != nil {
		return Peer{}, nil, err
	}
	defer func() { _ = client.Close() }()
	if err := client.send(Request{Op: OpPeer}); err != nil {
		return Peer{}, nil, err
	}
	var response Response
	if err := client.decode(&response); err != nil {
		return Peer{}, nil, err
	}
	return response.Peer, response.Store, nil
}

// Loading asks the daemon on sock how far its load has got, which it
// answers even while its first load holds every other request. A daemon
// that does not know the question answers with an error.
func Loading(sock string) (state.LoadProgress, error) {
	client, err := Dial(sock)
	if err != nil {
		return state.LoadProgress{}, err
	}
	defer func() { _ = client.Close() }()
	if err := client.send(Request{Op: OpLoading}); err != nil {
		return state.LoadProgress{}, err
	}
	var response Response
	if err := client.decode(&response); err != nil {
		return state.LoadProgress{}, err
	}
	if response.Error != "" || response.Loading == nil {
		return state.LoadProgress{}, fmt.Errorf("the daemon did not say how far it has loaded: %s", response.Error)
	}
	return *response.Loading, nil
}

func SendEvent(sock string, event json.RawMessage) error {
	client, err := Dial(sock)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	// A hook never checks the daemon's version: a tool call must not
	// fail over one (item 149).
	if err := client.send(Request{Op: OpEvent, Event: event}); err != nil {
		return err
	}
	var response Response
	if err := client.decode(&response); err != nil {
		return err
	}
	if response.Error != "" {
		return errors.New(response.Error)
	}
	return nil
}
