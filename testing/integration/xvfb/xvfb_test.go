package xvfb_test

import (
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/jezek/xgb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/xvfb"
)

func need(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("Xvfb"); err == nil {
		return
	}
	if os.Getenv("CI") != "" {
		t.Fatal("Xvfb is not installed, and CI must test the virtual display")
	}
	t.Skip("Xvfb is not installed, so the virtual display went untested")
}

func start(t *testing.T) *xvfb.Server {
	t.Helper()
	s, err := xvfb.Start(t.TempDir(), 320, 240)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func connect(s *xvfb.Server, cookie string) error {
	sock, err := net.Dial("unix", "/tmp/.X11-unix/X"+strings.TrimPrefix(s.Display, ":"))
	if err != nil {
		return err
	}
	c, err := xgb.NewConnNetWithCookieHex(sock, cookie)
	if err != nil {
		_ = sock.Close()
		return err
	}
	c.Close()
	return nil
}

// Item 126: xvfb-run -a looks for a free display and then takes it, so
// runs started together took the same one and lost their X server.
func TestStart(t *testing.T) {
	need(t)

	t.Run("when sixteen servers are started at once", func(t *testing.T) {
		servers := make([]*xvfb.Server, 16)
		errs := make([]error, 16)
		var wg sync.WaitGroup
		for i := range servers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				servers[i], errs[i] = xvfb.Start(t.TempDir(), 320, 240)
			}()
		}
		wg.Wait()
		for i, err := range errs {
			require.NoError(t, err, "server %d", i)
		}
		t.Cleanup(func() {
			for _, s := range servers {
				_ = s.Close()
			}
		})

		t.Run("it should give each a display of its own", func(t *testing.T) {
			displays := map[string]bool{}
			for _, s := range servers {
				displays[s.Display] = true
			}
			assert.Len(t, displays, len(servers))
		})

		t.Run("it should leave every one answering", func(t *testing.T) {
			var failed []string
			for _, s := range servers {
				if err := connect(s, s.Cookie); err != nil {
					failed = append(failed, s.Display+": "+err.Error())
				}
			}
			assert.Empty(t, failed)
		})
	})

	t.Run("when a client connects without the cookie", func(t *testing.T) {
		s := start(t)

		t.Run("it should be refused", func(t *testing.T) {
			assert.Error(t, connect(s, strings.Repeat("0", 32)))
		})
	})

	t.Run("when the environment it gives a client is read", func(t *testing.T) {
		s := start(t)
		env := s.Env()

		t.Run("it should name the display", func(t *testing.T) {
			assert.Contains(t, env, "DISPLAY="+s.Display)
		})

		t.Run("it should name the cookie's file", func(t *testing.T) {
			assert.Contains(t, env, "XAUTHORITY="+s.Authority)
		})
	})

	t.Run("when a server is closed", func(t *testing.T) {
		s := start(t)
		require.NoError(t, s.Close())

		t.Run("it should stop answering", func(t *testing.T) {
			assert.Error(t, connect(s, s.Cookie))
		})
	})
}
