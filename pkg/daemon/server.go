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
		switch request.Op {
		case proto.OpSnapshot:
			snapshot := d.Snapshot()
			if err := proto.Write(conn, proto.Response{Snapshot: &snapshot}); err != nil {
				return
			}
		case proto.OpSubscribe:
			d.stream(ctx, conn, request.Since)
			return
		case proto.OpEvents:
			if err := proto.Write(conn, proto.Response{Events: d.Events(request.Since)}); err != nil {
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
			if err := proto.Write(conn, response); err != nil {
				return
			}
		default:
			if err := proto.Write(conn, proto.Response{Error: fmt.Sprintf("unknown op %q", request.Op)}); err != nil {
				return
			}
		}
	}
}

func (d *Daemon) stream(ctx context.Context, conn net.Conn, since time.Time) {
	first, updates, cancel := d.Attach(since, true)
	defer cancel()
	if err := proto.Write(conn, proto.Response{Snapshot: &first.Snapshot, Events: first.Events}); err != nil {
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
			if err := proto.Write(conn, proto.Response{Snapshot: &update.Snapshot, Events: update.Events}); err != nil {
				return
			}
		}
	}
}
