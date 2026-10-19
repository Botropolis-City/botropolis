package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/proto"
)

func (d *Daemon) Serve(ctx context.Context, listener net.Listener) error {
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go d.handle(ctx, conn)
	}
}

func (d *Daemon) handle(ctx context.Context, conn net.Conn) {
	defer func() { _ = conn.Close() }()
	decoder := proto.NewDecoder(conn)
	for {
		var request proto.Request
		if err := decoder.Decode(&request); err != nil {
			return
		}
		// A hook holds up a session's tool call until it is answered.
		if request.Op != proto.OpEvent && !d.waitLoaded(ctx) {
			return
		}
		switch request.Op {
		case proto.OpSnapshot:
			snapshot := d.Snapshot()
			if err := d.respond(conn, proto.Response{Snapshot: &snapshot}); err != nil {
				return
			}
		case proto.OpSubscribe:
			d.stream(ctx, conn, request.Since)
			return
		case proto.OpEvents:
			if err := d.respond(conn, proto.Response{Events: d.Events(request.Since)}); err != nil {
				return
			}
		case proto.OpFind:
			response := proto.Response{}
			if request.Find == nil {
				response.Error = "find carries no query"
			} else if found, err := d.Find(*request.Find); err != nil {
				response.Error = err.Error()
			} else {
				response.Found = found
			}
			if err := d.respond(conn, response); err != nil {
				return
			}
		case proto.OpCloseUp:
			response := proto.Response{}
			if request.CloseUp == nil {
				response.Error = "close up names no sessions"
			} else if err := d.CloseUp(request.CloseUp.Open, request.CloseUp.Pinned); err != nil {
				response.Error = err.Error()
			}
			if err := d.respond(conn, response); err != nil {
				return
			}
		case proto.OpPack:
			response := proto.Response{}
			if request.Pack == nil {
				response.Error = "pack names no session"
			} else if err := d.Pack(request.Pack.SessionID, request.Pack.Packed); err != nil {
				response.Error = err.Error()
			}
			if err := d.respond(conn, response); err != nil {
				return
			}
		case proto.OpForget:
			response := proto.Response{}
			if request.Forget == "" {
				response.Error = "forget names no session"
			} else if err := d.Forget(request.Forget); err != nil {
				response.Error = err.Error()
			}
			if err := d.respond(conn, response); err != nil {
				return
			}
		case proto.OpEvent:
			response := proto.Response{}
			event, err := claude.ParseHookEvent(request.Event)
			if err != nil {
				response.Error = err.Error()
			} else {
				d.Apply(event, d.clock())
			}
			if err := d.respond(conn, response); err != nil {
				return
			}
		default:
			if err := d.respond(conn, proto.Response{Error: fmt.Sprintf("unknown op %q", request.Op)}); err != nil {
				return
			}
		}
	}
}

// respond writes a response saying which daemon sent it (item 149).
func (d *Daemon) respond(conn net.Conn, response proto.Response) error {
	response.Peer = d.peer
	return proto.Write(conn, response)
}

func (d *Daemon) stream(ctx context.Context, conn net.Conn, since time.Time) {
	first, updates, cancel := d.Attach(since, true)
	defer cancel()
	if err := d.respond(conn, proto.Response{Snapshot: &first.Snapshot, Events: first.Events}); err != nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			if err := d.respond(conn, proto.Response{Snapshot: &update.Snapshot, Events: update.Events}); err != nil {
				return
			}
		}
	}
}
