package commands_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/state"
)

// daemonSource answers for the files the way the daemon's own loader
// would, so a run without the daemon sees the same states.
type daemonSource struct{ d *daemon.Daemon }

func (s daemonSource) Snapshot(bool) (state.Snapshot, error) { return s.d.Snapshot(), nil }

func TestFind(t *testing.T) {
	run := func(t *testing.T, f *commands.Find, q find.Query) string {
		t.Helper()
		var out bytes.Buffer
		require.NoError(t, f.Run(&out, q))
		return out.String()
	}

	t.Run("when the daemon is listening", func(t *testing.T) {
		d, sock, home := servedDaemonHome(t)
		f := commands.NewFind(sock, home, daemonSource{d}, func() time.Time { return daemonNow })
		out := run(t, f, find.Query{Words: "ci queue", Limit: 5})

		t.Run("it should print what the daemon found", func(t *testing.T) {
			assert.Equal(t, "needs-you  59m ago  "+cwd+" (main)  "+sid[:8]+"\n    title: Fix the CI queue\n", out)
		})

		t.Run("and the daemon is not running", func(t *testing.T) {
			alone := commands.NewFind(t.TempDir()+"/none.sock", home, daemonSource{d}, func() time.Time { return daemonNow })

			t.Run("it should build its own index and print the same", func(t *testing.T) {
				assert.Equal(t, out, run(t, alone, find.Query{Words: "ci queue", Limit: 5}))
			})
		})
	})

	t.Run("when nothing matches", func(t *testing.T) {
		d, sock, home := servedDaemonHome(t)
		f := commands.NewFind(sock, home, daemonSource{d}, func() time.Time { return daemonNow })

		t.Run("it should say so", func(t *testing.T) {
			assert.Equal(t, "nothing found for \"kiln\"\n", run(t, f, find.Query{Words: "kiln"}))
		})
	})
}
