package stats_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/stats"
)

// Item 138: ingest gives way to a rescan before each transcript and
// inside a long one, so a session's state is never late behind it.
func TestIngestYields(t *testing.T) {
	ingestCounting := func(f *fixture) int {
		f.t.Helper()
		yields := 0
		if err := f.store.Ingest(f.projects, f.now, func() { yields++ }); err != nil {
			f.t.Fatal(err)
		}
		return yields
	}

	t.Run("when there are three transcripts", func(t *testing.T) {
		f := newFixture(t)
		for _, session := range []string{"s1", "s2", "s3"} {
			f.transcript(session, prompt(session, "/w/lib", "2026-10-18T10:00:00Z"))
		}

		t.Run("it should yield before each one", func(t *testing.T) {
			assert.Equal(t, 3, ingestCounting(f))
		})
	})

	t.Run("when a transcript is longer than a batch", func(t *testing.T) {
		f := newFixture(t)
		line := prompt("s1", "/w/lib", "2026-10-18T10:00:00Z")
		lines := make([]string, stats.YieldEvery*5/2/(len(line)+1)+1)
		for i := range lines {
			lines[i] = line
		}
		f.transcript("s1", lines...)

		t.Run("it should yield inside it too", func(t *testing.T) {
			assert.Equal(t, 3, ingestCounting(f))
		})
	})

	t.Run("when a line is longer than the reader's buffer", func(t *testing.T) {
		f := newFixture(t)
		long := reply{session: "s1", id: "msg_long", at: "2026-10-18T10:00:01Z", cwd: "/w/lib", usage: usage{Output: 42}}.line()
		long = strings.Replace(long, `"hello"`, `"`+strings.Repeat("x", 3*stats.YieldEvery)+`"`, 1)
		f.transcript("s1", prompt("s1", "/w/lib", "2026-10-18T10:00:00Z"), long)
		f.ingest()

		t.Run("it should still read the message on it", func(t *testing.T) {
			assert.Equal(t, int64(42), f.number("SELECT output FROM messages WHERE id = 'msg_long'"))
		})
	})
}
