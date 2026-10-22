package stats_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type progressAt struct{ done, known int64 }

// Item 159: nothing reported how far a first launch had got, so the
// city could show no percentage.
func TestIngestProgress(t *testing.T) {
	ingestWatching := func(f *fixture) (first, last progressAt) {
		f.t.Helper()
		seen := false
		require.NoError(f.t, f.store.Ingest(f.projects, f.now, func() {
			if !seen {
				first.done, first.known = f.store.Progress()
				seen = true
			}
		}))
		last.done, last.known = f.store.Progress()
		return first, last
	}
	size := func(t *testing.T, path string) int64 {
		info, err := os.Stat(path)
		require.NoError(t, err)
		return info.Size()
	}

	t.Run("when an ingest has two transcripts to read, one of them read halfway before", func(t *testing.T) {
		f := newFixture(t)
		long := strings.Repeat("x", 4096)
		a := f.transcript(sessionA, prompt(sessionA, lib, "2026-10-18T08:00:00.000Z"))
		f.ingest()
		readBefore := size(t, a)
		f.appendRaw(a, strings.Replace(prompt(sessionA, lib, "2026-10-18T08:00:01.000Z"), `"hi"`, `"`+long+`"`, 1)+"\n")
		b := f.transcript(sessionB, prompt(sessionB, lib, "2026-10-18T08:00:02.000Z"))
		left := size(t, a) - readBefore + size(t, b)
		first, last := ingestWatching(f)

		t.Run("it should know the bytes left to read before it reads any", func(t *testing.T) {
			assert.Equal(t, progressAt{0, left}, first)
		})

		t.Run("it should count every byte it reads", func(t *testing.T) {
			assert.Equal(t, progressAt{left, left}, last)
		})
	})

	t.Run("when nothing is left to read", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, prompt(sessionA, lib, "2026-10-18T08:00:00.000Z"))
		f.ingest()
		_, last := ingestWatching(f)

		t.Run("it should know of nothing to read", func(t *testing.T) {
			assert.Equal(t, progressAt{0, 0}, last)
		})
	})
}
