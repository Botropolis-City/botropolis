package acceptance_test

import (
	"bytes"
	"context"
	"image"
	_ "image/png"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// oldDaemon answers every request on a socket as a daemon of another
// release would, with an empty snapshot.
func oldDaemon(t *testing.T, peer proto.Peer) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "bt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sock := filepath.Join(dir, "d.sock")
	listener, err := net.Listen("unix", sock)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				decoder := proto.NewDecoder(conn)
				for {
					var request proto.Request
					if decoder.Decode(&request) != nil {
						return
					}
					if proto.Write(conn, proto.Response{Peer: peer, Snapshot: &state.Snapshot{}}) != nil {
						return
					}
				}
			}()
		}
	}()
	return sock
}

// Item 152: a daemon of another release stops every command, which says
// so on one line, rather than being worked around.
func TestDaemonOfAnotherRelease(t *testing.T) {
	sock := oldDaemon(t, proto.Peer{Release: "0.0.1", Protocol: proto.Protocol})

	for _, args := range [][]string{
		{"status", "--json"},
		{"events"},
		{"find", "ci"},
		{"bar", "--watch"},
		{"notify"},
	} {
		t.Run("when "+strings.Join(args, " ")+" reaches a daemon of another release", func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, botropolis, append(args, "--socket", sock)...)
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()

			t.Run("it should refuse rather than run on", func(t *testing.T) {
				assert.NoError(t, ctx.Err())
			})

			t.Run("it should fail", func(t *testing.T) {
				assert.Error(t, err)
			})

			t.Run("it should say so on one line, with what to do", func(t *testing.T) {
				assert.Regexp(t, `^botropolis: the Botropolis city and its daemon must be running the same version \(daemon 0\.0\.1, city dev\): \S.*\n$`, stderr.String())
			})

			t.Run("it should print nothing else", func(t *testing.T) {
				assert.Empty(t, stdout.String())
			})
		})
	}
}

// Item 152: the map against a daemon of another release draws the
// dialog and no city.
func TestCityAgainstAnotherRelease(t *testing.T) {
	helpers.NeedXvfb(t)

	t.Run("when the city opens against a daemon of another release", func(t *testing.T) {
		sock := oldDaemon(t, proto.Peer{Release: "0.0.1", Protocol: proto.Protocol})
		r := newStateRun(t)
		withSession(t, r.home)
		shot := filepath.Join(r.tmp, "shot.png")
		out, err := r.command(t, true, "--headless", "--screenshot", shot, "--socket", sock).CombinedOutput()

		t.Run("it should exit cleanly", func(t *testing.T) {
			assert.NoError(t, err, string(out))
		})

		t.Run("it should draw nothing but the dialog's backdrop around it", func(t *testing.T) {
			f, err := os.Open(shot)
			require.NoError(t, err)
			defer func() { _ = f.Close() }()
			img, _, err := image.Decode(f)
			require.NoError(t, err)
			b := img.Bounds()
			backdrop := img.At(b.Min.X+2, b.Min.Y+2)
			var differing []image.Point
			for i := 0; i <= 10; i++ {
				for _, p := range []image.Point{
					{b.Min.X + 2 + i*(b.Dx()-5)/10, b.Min.Y + 2},
					{b.Min.X + 2 + i*(b.Dx()-5)/10, b.Max.Y - 3},
					{b.Min.X + 2, b.Min.Y + 2 + i*(b.Dy()-5)/10},
					{b.Max.X - 3, b.Min.Y + 2 + i*(b.Dy()-5)/10},
				} {
					if img.At(p.X, p.Y) != backdrop {
						differing = append(differing, p)
					}
				}
			}
			assert.Empty(t, differing)
		})
	})
}
