//go:build devtools

package memlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLog(t *testing.T) {
	images := func() int64 { return 16 << 20 }

	t.Run("when no log is asked for", func(t *testing.T) {
		t.Run("it should make no log", func(t *testing.T) {
			assert.Nil(t, New("", images))
		})
	})

	t.Run("when a log is asked for", func(t *testing.T) {
		start := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
		open := func(t *testing.T) (*Log, string, *time.Time) {
			path := filepath.Join(t.TempDir(), "memory.log")
			now := start
			l := New(path, images)
			l.now = func() time.Time { return now }
			l.heap = func() uint64 { return 200 << 20 }
			return l, path, &now
		}
		lines := func(path string) []string {
			data, _ := os.ReadFile(path)
			return strings.Split(strings.TrimSpace(string(data)), "\n")
		}

		t.Run("and a sample is due", func(t *testing.T) {
			l, path, _ := open(t)
			l.Sample()

			t.Run("it should write the images and the live heap on one line", func(t *testing.T) {
				assert.Equal(t, []string{"images=16777216 heap=209715200"}, lines(path))
			})
		})

		t.Run("and the next frame comes before 5 s", func(t *testing.T) {
			l, path, now := open(t)
			l.Sample()
			*now = now.Add(4 * time.Second)
			l.Sample()

			t.Run("it should write nothing more", func(t *testing.T) {
				assert.Len(t, lines(path), 1)
			})
		})

		t.Run("and the next frame comes 5 s later", func(t *testing.T) {
			l, path, now := open(t)
			l.Sample()
			*now = now.Add(5 * time.Second)
			l.Sample()

			t.Run("it should write a second line", func(t *testing.T) {
				assert.Len(t, lines(path), 2)
			})
		})
	})
}
