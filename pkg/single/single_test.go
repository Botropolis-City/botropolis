package single_test

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/single"
)

// socketPath is a socket in a short temporary directory: a unix socket's
// path has to fit in about a hundred bytes.
func socketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "bc")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "city.sock")
}

// listening starts the first city on path and closes it after the test.
func listening(t *testing.T, path string) *single.City {
	t.Helper()
	city, deferred, err := single.Claim(path, single.Request{Op: single.Raise}, false)
	require.NoError(t, err)
	require.False(t, deferred)
	require.NotNil(t, city)
	t.Cleanup(func() { _ = city.Close() })
	return city
}

// next is the next request the city hears, or the zero request after a
// second.
func next(city *single.City) single.Request {
	select {
	case r := <-city.Requests():
		return r
	case <-time.After(time.Second):
		return single.Request{}
	}
}

func TestClaim(t *testing.T) {
	t.Run("when no city is listening", func(t *testing.T) {
		t.Run("and a city starts", func(t *testing.T) {
			path := socketPath(t)
			listening(t, path)

			t.Run("it should listen", func(t *testing.T) {
				conn, err := net.Dial("unix", path)
				require.NoError(t, err)
				assert.NoError(t, conn.Close())
			})
		})
	})

	t.Run("when a city is listening", func(t *testing.T) {
		t.Run("and a second city starts", func(t *testing.T) {
			path := socketPath(t)
			first := listening(t, path)
			second, deferred, err := single.Claim(path, single.Request{Op: single.Raise}, false)
			require.NoError(t, err)

			t.Run("it should ask the first to come forward and not open a window", func(t *testing.T) {
				assert.Equal(t, [3]any{true, true, single.Request{Op: single.Raise}}, [3]any{deferred, second == nil, next(first)})
			})
		})

		t.Run("and botropolis find is run", func(t *testing.T) {
			path := socketPath(t)
			first := listening(t, path)
			answered, err := single.Send(path, single.Request{Op: single.Find, Words: "kiln"})
			require.NoError(t, err)

			t.Run("it should ask the first to open the finder", func(t *testing.T) {
				assert.Equal(t, [2]any{true, single.Request{Op: single.Find, Words: "kiln"}}, [2]any{answered, next(first)})
			})
		})
	})

	t.Run("when the socket was left by a city that died", func(t *testing.T) {
		path := socketPath(t)
		dead, err := net.Listen("unix", path)
		require.NoError(t, err)
		dead.(*net.UnixListener).SetUnlinkOnClose(false)
		require.NoError(t, dead.Close())
		require.FileExists(t, path)

		t.Run("and a city starts", func(t *testing.T) {
			city, deferred, err := single.Claim(path, single.Request{Op: single.Raise}, false)
			require.NoError(t, err)
			if city != nil {
				t.Cleanup(func() { _ = city.Close() })
			}

			t.Run("it should take it over", func(t *testing.T) {
				assert.Equal(t, [2]bool{false, true}, [2]bool{deferred, city != nil})
			})
		})
	})

	t.Run("when a city is scripted", func(t *testing.T) {
		path := socketPath(t)
		first := listening(t, path)
		scripted, deferred, err := single.Claim(path, single.Request{Op: single.Raise}, true)
		require.NoError(t, err)

		t.Run("it should neither listen nor defer", func(t *testing.T) {
			assert.Equal(t, [3]any{false, true, single.Request{}}, [3]any{deferred, scripted == nil, next(first)})
		})
	})
}
