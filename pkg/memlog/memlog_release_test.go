//go:build !devtools

package memlog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLog(t *testing.T) {
	t.Run("when built for release", func(t *testing.T) {
		t.Run("and a log is asked for", func(t *testing.T) {
			l := New(t.TempDir()+"/memory.log", Sources{Images: func() int64 { return 1 }})

			t.Run("it should make no log, so nothing ever collects or writes", func(t *testing.T) {
				assert.Nil(t, l)
			})
		})
	})
}
