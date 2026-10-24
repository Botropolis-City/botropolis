package acceptance_test

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a process's output, which the test reads while the
// process is still writing it.
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

// untilStill waits until what read counts has not moved for quiet, which
// is how a test knows a stream of deliveries has ended, however long a
// loaded machine takes to deliver them (item 226). It fails the test,
// saying what it waited for, if read is still moving after limit.
func untilStill(t *testing.T, what string, read func() int64, quiet, limit time.Duration) {
	t.Helper()
	deadline := time.Now().Add(limit)
	last, since := read(), time.Now()
	for time.Since(since) < quiet {
		if time.Now().After(deadline) {
			t.Fatalf("%s was still moving after %s", what, limit)
		}
		time.Sleep(50 * time.Millisecond)
		if n := read(); n != last {
			last, since = n, time.Now()
		}
	}
}
