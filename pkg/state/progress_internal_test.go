package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 159: with statistics off, the whole read is the first launch's
// wait.
func TestWholeReadProgress(t *testing.T) {
	t.Run("when the whole read is reading the second of a 30 MiB and a 10 MiB stopped transcript", func(t *testing.T) {
		todo := []stoppedFile{{path: "a.jsonl", key: fileKey{size: 30 << 20}}, {path: "b.jsonl", key: fileKey{size: 10 << 20}}}
		reading, release := make(chan struct{}), make(chan struct{})
		w := &wholeRead{hefts: map[string]cachedHeft{}, pace: (&fakeClock{}).pace(1), running: true}
		w.readFile = func(path string) (int64, bool) {
			if path == "b.jsonl" {
				close(reading)
				<-release
			}
			return 0, true
		}
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			w.pass(todo)
		}()
		<-reading
		got := w.progress()
		close(release)
		<-finished

		t.Run("it should report the first as read, of both", func(t *testing.T) {
			assert.Equal(t, &ScanProgress{Done: 30 << 20, Known: 40 << 20}, got)
		})

		t.Run("and it has finished", func(t *testing.T) {
			t.Run("it should report nothing", func(t *testing.T) {
				assert.Nil(t, w.progress())
			})
		})
	})
}
