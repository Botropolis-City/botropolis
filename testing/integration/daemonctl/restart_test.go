package daemonctl_test

import (
	"context"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/daemonctl"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/testing/helpers"
)

const fakeFlag = "--fake-daemon"

// TestMain lets this test binary stand in for a daemon started by hand:
// run with fakeFlag, a socket and a release file, it answers every
// request as that release, as an upgraded package's binary would.
func TestMain(m *testing.M) {
	if len(os.Args) == 4 && os.Args[1] == fakeFlag {
		os.Exit(fakeDaemon(os.Args[2], os.Args[3]))
	}
	os.Exit(helpers.RunIsolated(m))
}

func TestHomeIsolation(t *testing.T) {
	helpers.AssertIsolated(t)
}

func fakeDaemon(sock, releaseFile string) int {
	release, err := os.ReadFile(releaseFile)
	if err != nil {
		return 1
	}
	peer := proto.Peer{Release: strings.TrimSpace(string(release)), Protocol: proto.Protocol}
	_ = os.Remove(sock)
	listener, err := net.Listen("unix", sock)
	if err != nil {
		return 1
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM)
	go func() {
		<-stop
		_ = listener.Close()
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			return 0
		}
		go func() {
			defer func() { _ = conn.Close() }()
			var request proto.Request
			if proto.NewDecoder(conn).Decode(&request) == nil {
				_ = proto.Write(conn, proto.Response{Peer: peer})
			}
		}()
	}
}

func waitForAnswer(t *testing.T, sock string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := proto.Ask(sock); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the fake daemon never answered on %s", sock)
}

// Item 152: a daemon started by hand is stopped and this build's daemon
// started in its place, with the arguments the old one had.
func TestRestartByHand(t *testing.T) {
	t.Run("when a daemon of another release runs by hand", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "bt")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		sock := filepath.Join(dir, "d.sock")
		releaseFile := filepath.Join(dir, "release")
		require.NoError(t, os.WriteFile(releaseFile, []byte("0.0.1\n"), 0o600))
		self, err := os.Executable()
		require.NoError(t, err)

		old := exec.Command(self, fakeFlag, sock, releaseFile)
		require.NoError(t, old.Start())
		oldExited := make(chan struct{})
		go func() { _ = old.Wait(); close(oldExited) }()
		t.Cleanup(func() { _ = old.Process.Kill() })
		waitForAnswer(t, sock)

		require.NoError(t, os.WriteFile(releaseFile, []byte("0.2.3\n"), 0o600))
		want := proto.Peer{Release: "0.2.3", Protocol: proto.Protocol}
		sys := daemonctl.Native()
		sys.GOOS = "linux"
		sys.Cgroup = func(int) (string, error) { return "0::/user.slice/test.scope\n", nil }
		sys.Run = func(_ context.Context, name string, args ...string) (string, error) {
			t.Errorf("a by-hand restart ran %s %v", name, args)
			return "", os.ErrPermission
		}
		sys.Binary = func() (string, error) { return self, nil }

		t.Run("and it is restarted", func(t *testing.T) {
			restartErr := daemonctl.Restart(context.Background(), sys, sock, want)
			newPID, _ := daemonctl.PeerPID(sock)
			t.Cleanup(func() {
				if newPID > 0 && newPID != old.Process.Pid {
					_ = syscall.Kill(newPID, syscall.SIGTERM)
				}
			})

			t.Run("it should succeed", func(t *testing.T) {
				assert.NoError(t, restartErr)
			})

			t.Run("it should stop the old process", func(t *testing.T) {
				select {
				case <-oldExited:
				case <-time.After(time.Second):
					t.Fatal("the old daemon is still running")
				}
			})

			t.Run("it should answer at this build's release", func(t *testing.T) {
				peer, _ := proto.Ask(sock)
				assert.Equal(t, want, peer)
			})

			t.Run("it should answer from another process", func(t *testing.T) {
				assert.NotEqual(t, old.Process.Pid, newPID)
			})
		})
	})
}
