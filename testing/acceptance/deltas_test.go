//go:build linux

package acceptance_test

import (
	"bytes"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/proto"
)

// Item 201: about 220 KB of snapshot a second reached a city watching
// item 197's, and decoding it was most of what the city cost. The daemon
// now sends what changed, and this counts the bytes on the wire against
// what the whole snapshots would have been.
func TestTheStreamOnABusyCity(t *testing.T) {
	const (
		every  = 200 * time.Millisecond
		stream = 10 * time.Second
		most   = 0.05
	)
	b := newBusyCity(t)

	t.Run("when a session of 200 writes a line every 200 ms for 10 s", func(t *testing.T) {
		conn, err := net.Dial("unix", b.sock)
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		require.NoError(t, proto.Write(conn, proto.Request{Op: proto.OpSubscribe, Peer: proto.Self()}))
		wire := &counted{r: conn}
		decoder := proto.NewDecoder(wire)
		var (
			mirror  proto.Mirror
			mu      sync.Mutex
			whole   int64
			updates int
			bad     error
			first   = make(chan struct{})
			done    = make(chan struct{})
		)
		go func() {
			defer close(done)
			for n := 0; ; n++ {
				var r proto.Response
				if err := decoder.Decode(&r); err != nil {
					return
				}
				held, err := mirror.Take(r)
				var full bytes.Buffer
				_ = proto.Write(&full, proto.Response{Peer: r.Peer, Seq: r.Seq, Snapshot: &held, Events: r.Events})
				mu.Lock()
				if err != nil && bad == nil {
					bad = err
				}
				if n > 0 {
					whole += int64(full.Len())
					updates++
				}
				mu.Unlock()
				if n == 0 {
					close(first)
				}
			}
		}()
		select {
		case <-first:
		case <-time.After(60 * time.Second):
			t.Fatal("no first snapshot")
		}
		wire.n.Store(0)
		elapsed := b.write(every, stream)
		time.Sleep(2 * time.Second)
		sent := wire.n.Load()
		_ = conn.Close()
		<-done
		mu.Lock()
		defer mu.Unlock()
		t.Logf("%d updates in %.0f s: %d KiB sent, %.1f KiB/s, against %d KiB, %.1f KiB/s, as whole snapshots",
			updates, elapsed.Seconds(), sent>>10, float64(sent)/1024/elapsed.Seconds(), whole>>10, float64(whole)/1024/elapsed.Seconds())

		t.Run("its updates should all have applied", func(t *testing.T) {
			require.Greater(t, updates, 5)
			assert.NoError(t, bad)
		})

		t.Run("it should send at most 5% of the bytes the whole snapshots would have been", func(t *testing.T) {
			assert.LessOrEqual(t, float64(sent), most*float64(whole))
		})
	})
}

type counted struct {
	r io.Reader
	n atomic.Int64
}

func (c *counted) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}
