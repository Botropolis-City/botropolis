package interrupt_test

import (
	"bytes"
	"context"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/interrupt"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// Item 114: the first signal asks the city to stop and clean up after
// itself; a second, while it is still stopping, is the person insisting.
func TestContext(t *testing.T) {
	type run struct {
		ctx    context.Context
		exited chan int
		stderr *syncBuffer
		sigs   chan os.Signal
	}
	start := func(t *testing.T) run {
		r := run{exited: make(chan int, 1), stderr: &syncBuffer{}, sigs: make(chan os.Signal, 2)}
		parent, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		r.ctx = interrupt.Context(parent, r.sigs, func(code int) { r.exited <- code }, r.stderr)
		return r
	}
	exitCode := func(r run) int {
		select {
		case code := <-r.exited:
			return code
		case <-time.After(time.Second):
			return -1
		}
	}

	t.Run("when no signal has arrived", func(t *testing.T) {
		r := start(t)

		t.Run("it should leave the context running", func(t *testing.T) {
			assert.NoError(t, r.ctx.Err())
		})
	})

	t.Run("when one signal arrives", func(t *testing.T) {
		r := start(t)
		r.sigs <- syscall.SIGINT
		<-r.ctx.Done()

		t.Run("it should cancel the context", func(t *testing.T) {
			assert.ErrorIs(t, r.ctx.Err(), context.Canceled)
		})

		t.Run("it should not exit", func(t *testing.T) {
			select {
			case code := <-r.exited:
				t.Fatalf("exited with %d", code)
			case <-time.After(50 * time.Millisecond):
			}
		})
	})

	for _, s := range []struct {
		sig  syscall.Signal
		code int
	}{{syscall.SIGINT, 130}, {syscall.SIGTERM, 143}} {
		t.Run("when a second "+s.sig.String()+" arrives before the loop ends", func(t *testing.T) {
			r := start(t)
			r.sigs <- s.sig
			<-r.ctx.Done()
			r.sigs <- s.sig
			code := exitCode(r)

			t.Run("it should exit at once with the signal's status", func(t *testing.T) {
				assert.Equal(t, s.code, code)
			})

			t.Run("it should say why", func(t *testing.T) {
				assert.Contains(t, r.stderr.String(), "second interrupt: exiting without cleanup")
			})
		})
	}
}
