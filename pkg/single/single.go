// Package single keeps to one city per user: the city listens on a
// small socket of its own, a second botropolis city asks the first to
// come forward instead of opening another window, and botropolis find
// asks it to open the finder. Item 82.
package single

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"botropolis.city/botropolis/pkg/proto"
)

// What a request asks of the running city.
const (
	Raise = "raise"
	Find  = "find"
)

// answerWithin is how long a city has to say it heard a request; a
// socket that takes no answer is one nobody is listening on.
const answerWithin = 2 * time.Second

// Request is one thing asked of the running city: to come forward, or
// to open the finder, with words to start from.
type Request struct {
	Op    string `json:"op"`
	Words string `json:"words,omitempty"`
}

// SocketPath is where the city listens, beside the daemon's socket.
func SocketPath() string {
	return filepath.Join(filepath.Dir(proto.SocketPath()), "city.sock")
}

// City is the running city's end of the socket.
type City struct {
	listener net.Listener
	requests chan Request
	once     sync.Once
}

// Requests delivers what other commands ask of the city.
func (c *City) Requests() <-chan Request { return c.requests }

// Close stops listening and removes the socket.
func (c *City) Close() error {
	var err error
	c.once.Do(func() { err = c.listener.Close() })
	return err
}

// Claim makes this the one city, or finds there is one already. When a
// city answers on path it is sent first and Claim reports that it
// deferred; otherwise Claim listens, taking over a socket a city that
// died left behind. A scripted city — a screenshot, a recording, a
// headless run — does neither, so frames and tests can run beside the
// city someone is using.
func Claim(path string, first Request, scripted bool) (*City, bool, error) {
	if scripted {
		return nil, false, nil
	}
	if answered, err := Send(path, first); answered || err != nil {
		return nil, answered, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, false, err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, false, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, false, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		return nil, false, err
	}
	c := &City{listener: listener, requests: make(chan Request, 8)}
	go c.serve()
	return c, false, nil
}

func (c *City) serve() {
	for {
		conn, err := c.listener.Accept()
		if err != nil {
			return
		}
		go c.hear(conn)
	}
}

func (c *City) hear(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(answerWithin))
	var r Request
	if json.NewDecoder(conn).Decode(&r) != nil {
		return
	}
	select {
	case c.requests <- r:
	default:
	}
	_ = json.NewEncoder(conn).Encode(struct {
		Heard bool `json:"heard"`
	}{true})
}

// Send asks the running city for something, and reports whether a city
// answered; with none listening it reports false and no error.
func Send(path string, r Request) (bool, error) {
	conn, err := net.DialTimeout("unix", path, answerWithin)
	if err != nil {
		return false, nil
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(answerWithin))
	if err := json.NewEncoder(conn).Encode(r); err != nil {
		return false, nil
	}
	var answer struct {
		Heard bool `json:"heard"`
	}
	if json.NewDecoder(conn).Decode(&answer) != nil {
		return false, nil
	}
	return answer.Heard, nil
}
