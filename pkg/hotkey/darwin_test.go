//go:build darwin

package hotkey

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOpenOnMacOS(t *testing.T) {
	t.Run("when the desktop is macOS and nothing drains the main thread", func(t *testing.T) {
		type opened struct {
			d   Desktop
			err error
		}
		done := make(chan opened, 1)
		go func() {
			d, err := Open("darwin", os.Getenv)
			done <- opened{d, err}
		}()
		var got opened
		select {
		case got = <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("Open waited on the main thread, which nothing drains in a unit test or in Ebitengine's Update")
		}
		if got.d != nil {
			t.Cleanup(got.d.Close)
		}

		t.Run("it should return a desktop without waiting", func(t *testing.T) {
			assert.NoError(t, got.err)
		})
	})
}
