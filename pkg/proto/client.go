package proto

import (
	"encoding/json"
	"errors"
	"net"
	"time"

	"botropolis.city/botropolis/pkg/events"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/state"
)

const dialTimeout = 500 * time.Millisecond

type Client struct {
	conn    net.Conn
	decoder *json.Decoder
}

func Dial(sock string) (*Client, error) {
	conn, err := net.DialTimeout("unix", sock, dialTimeout)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, decoder: json.NewDecoder(conn)}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Snapshot() (state.Snapshot, error) {
	if err := Write(c.conn, Request{Op: OpSnapshot}); err != nil {
		return state.Snapshot{}, err
	}
	update, err := c.readUpdate()
	return update.Snapshot, err
}

// Subscribe streams every change; the first update carries the current
// snapshot and the log since the moment given (all of it when zero).
func (c *Client) Subscribe(since time.Time) (<-chan Update, error) {
	if err := Write(c.conn, Request{Op: OpSubscribe, Since: since}); err != nil {
		return nil, err
	}
	updates := make(chan Update, 1)
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
	if err := Write(c.conn, Request{Op: OpEvents, Since: since}); err != nil {
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
	if err := Write(c.conn, Request{Op: OpFind, Find: &q}); err != nil {
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
	if err := Write(c.conn, Request{Op: OpCloseUp, CloseUp: &CloseUp{Open: open, Pinned: pinned}}); err != nil {
		return err
	}
	_, err := c.readResponse()
	return err
}

// Pack packs a session up, or unpacks it.
func (c *Client) Pack(id string, packed bool) error {
	if err := Write(c.conn, Request{Op: OpPack, Pack: &Pack{SessionID: id, Packed: packed}}); err != nil {
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
	if response.Snapshot == nil {
		return Update{}, errors.New("response carried no snapshot")
	}
	return Update{Snapshot: *response.Snapshot, Events: response.Events}, nil
}

func (c *Client) readResponse() (Response, error) {
	var response Response
	if err := c.decoder.Decode(&response); err != nil {
		return Response{}, err
	}
	if response.Error != "" {
		return Response{}, errors.New(response.Error)
	}
	return response, nil
}

func SendEvent(sock string, event json.RawMessage) error {
	client, err := Dial(sock)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	if err := Write(client.conn, Request{Op: OpEvent, Event: event}); err != nil {
		return err
	}
	var response Response
	if err := client.decoder.Decode(&response); err != nil {
		return err
	}
	if response.Error != "" {
		return errors.New(response.Error)
	}
	return nil
}
