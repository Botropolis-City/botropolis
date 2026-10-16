package commands_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/single"
)

func TestFindOpen(t *testing.T) {
	citySocket := func(t *testing.T) string {
		t.Helper()
		dir, err := os.MkdirTemp("", "bc")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		return filepath.Join(dir, "city.sock")
	}

	t.Run("when run with no words and a city is running", func(t *testing.T) {
		path := citySocket(t)
		running, deferred, err := single.Claim(path, single.Request{Op: single.Raise}, false)
		require.NoError(t, err)
		require.False(t, deferred)
		t.Cleanup(func() { _ = running.Close() })
		started := 0
		f := commands.NewFind("", "", nil, time.Now).WithCity(path, func() error { started++; return nil })
		require.NoError(t, f.Open())

		t.Run("it should open the finder in that city", func(t *testing.T) {
			var heard single.Request
			select {
			case heard = <-running.Requests():
			case <-time.After(time.Second):
			}
			assert.Equal(t, [2]any{single.Request{Op: single.Find}, 0}, [2]any{heard, started})
		})
	})

	t.Run("when run with no words and no city is running", func(t *testing.T) {
		started := 0
		f := commands.NewFind("", "", nil, time.Now).WithCity(citySocket(t), func() error { started++; return nil })
		require.NoError(t, f.Open())

		t.Run("it should start the city with the finder open", func(t *testing.T) {
			assert.Equal(t, 1, started)
		})
	})
}
