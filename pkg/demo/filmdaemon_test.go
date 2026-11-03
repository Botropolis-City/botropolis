package demo

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 333: a shot that packs sessions up films against a daemon of its
// own, started in the shot's scratch directory and nowhere else.
func TestFilmDaemon(t *testing.T) {
	t.Run("when a shot runs with its own daemon", func(t *testing.T) {
		work := t.TempDir()
		sock := daemonSocket(work)
		args := cityArgs(Shot{Name: "convoy", Daemon: true}, filepath.Join(work, "home"), "out.png", "", withDaemon(sock))

		t.Run("the city should not be pointed at a home, so the scratch home is its own", func(t *testing.T) {
			assert.NotContains(t, args, "--home")
		})

		t.Run("the city should talk to the shot's daemon", func(t *testing.T) {
			at := slices.Index(args, "--socket")
			require.GreaterOrEqual(t, at, 0)
			assert.Equal(t, sock, args[at+1])
		})

		t.Run("the daemon's socket should be under the shot's scratch directory", func(t *testing.T) {
			assert.NoError(t, contained(work, nil, sock))
		})

		t.Run("the daemon should be started on that socket", func(t *testing.T) {
			cmd := Filmer{Daemon: "/bin/botropolisd"}.daemonCommand(work, "UTC", sock)
			assert.Equal(t, []string{"/bin/botropolisd", "--socket", sock}, cmd.Args)
		})

		t.Run("the daemon should die with the filmer", func(t *testing.T) {
			if runtime.GOOS != "linux" {
				t.Skip("only Linux kills a child with its parent")
			}
			cmd := Filmer{Daemon: "/bin/botropolisd"}.daemonCommand(work, "UTC", sock)
			assert.True(t, tiedToParent(cmd))
		})
	})

	t.Run("when every path the shot's processes get is in its scratch directory", func(t *testing.T) {
		work := t.TempDir()

		t.Run("it should allow them", func(t *testing.T) {
			assert.NoError(t, contained(work, filmEnv(work, "UTC"), daemonSocket(work)))
		})
	})

	for _, c := range []struct {
		name  string
		env   func(work string) []string
		paths func(work string) []string
	}{
		{"when HOME is outside the scratch directory",
			func(string) []string { return []string{"HOME=" + os.TempDir() + "/elsewhere"} }, nil},
		{"when an XDG directory is outside it",
			func(work string) []string { return append(filmEnv(work, "UTC"), "XDG_STATE_HOME=/var/tmp/state") }, nil},
		{"when the socket is outside it", nil,
			func(string) []string { return []string{"/run/user/1000/botropolis/botropolis.sock"} }},
		{"when a path inside it is a link that leads out", nil, func(work string) []string {
			out := t.TempDir()
			link := filepath.Join(work, "run")
			require.NoError(t, os.Symlink(out, link))
			return []string{filepath.Join(link, "botropolis.sock")}
		}},
		{"when a path climbs out with ..", nil,
			func(work string) []string { return []string{filepath.Join(work, "..", "botropolis.sock")} }},
	} {
		t.Run(c.name, func(t *testing.T) {
			work := t.TempDir()
			var env, paths []string
			if c.env != nil {
				env = c.env(work)
			}
			if c.paths != nil {
				paths = c.paths(work)
			}

			t.Run("it should refuse to start the daemon", func(t *testing.T) {
				assert.ErrorContains(t, contained(work, env, paths...), "outside the shot's scratch directory")
			})
		})
	}
}
