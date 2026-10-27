//go:build linux || darwin

package control_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"botropolis.city/botropolis/pkg/control"
)

func TestExecRunner(t *testing.T) {
	runner := control.ExecRunner()

	killOnCleanup := func(t *testing.T, pidFile string) {
		t.Cleanup(func() {
			raw, err := os.ReadFile(pidFile)
			if err != nil {
				return
			}
			if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		})
	}

	t.Run("when a clipboard tool forks a child that keeps stdout and stderr open, as xclip does", func(t *testing.T) {
		pidFile := filepath.Join(t.TempDir(), "child")
		killOnCleanup(t, pidFile)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		start := time.Now()
		_ = runner.Feed(ctx, "/src/kiln", "sh", "-c", `cat >/dev/null; sleep 30 & echo $! >"$0"; exit 0`, pidFile)

		t.Run("it should return once the tool itself exits", func(t *testing.T) {
			assert.Less(t, time.Since(start), time.Second)
		})
	})

	t.Run("when a tool is read from", func(t *testing.T) {
		t.Run("and it prints to stdout", func(t *testing.T) {
			got, err := runner.Read(context.Background(), "sh", "-c", `printf '/w/api'`)
			require.NoError(t, err)

			t.Run("it should return what it printed", func(t *testing.T) {
				assert.Equal(t, "/w/api", got)
			})
		})

		t.Run("and it forks a child that keeps stdout and stderr open", func(t *testing.T) {
			pidFile := filepath.Join(t.TempDir(), "child")
			killOnCleanup(t, pidFile)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			start := time.Now()
			_, _ = runner.Read(ctx, "sh", "-c", `printf x; sleep 30 & echo $! >"$0"; exit 0`, pidFile)

			t.Run("it should return once the tool itself exits", func(t *testing.T) {
				assert.Less(t, time.Since(start), time.Second)
			})
		})

		t.Run("and it never exits before the context's deadline", func(t *testing.T) {
			pidFile := filepath.Join(t.TempDir(), "child")
			killOnCleanup(t, pidFile)
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			start := time.Now()
			_, _ = runner.Read(ctx, "sh", "-c", `sleep 30 & echo $! >"$0"; wait`, pidFile)

			t.Run("it should return within the deadline plus the wait delay", func(t *testing.T) {
				assert.Less(t, time.Since(start), 2*time.Second)
			})
		})

		t.Run("and it fails and says why on stderr", func(t *testing.T) {
			_, err := runner.Read(context.Background(), "sh", "-c", `echo "Nothing is copied" >&2; exit 1`)

			t.Run("it should return an error carrying what it said", func(t *testing.T) {
				assert.ErrorContains(t, err, "Nothing is copied")
			})
		})
	})

	t.Run("when the tool reads its input", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out")
		require.NoError(t, runner.Feed(context.Background(), "/src/kiln", "sh", "-c", `cat >"$0"`, out))
		got, _ := os.ReadFile(out)

		t.Run("it should receive the text on stdin", func(t *testing.T) {
			assert.Equal(t, "/src/kiln", string(got))
		})
	})

	t.Run("when the tool fails and says why on stderr", func(t *testing.T) {
		err := runner.Feed(context.Background(), "x", "sh", "-c", `cat >/dev/null; echo "cannot open display" >&2; exit 1`)

		t.Run("it should return an error carrying what it said", func(t *testing.T) {
			assert.ErrorContains(t, err, "cannot open display")
		})
	})

	t.Run("when the tool never exits", func(t *testing.T) {
		t.Run("and the context has a deadline", func(t *testing.T) {
			pidFile := filepath.Join(t.TempDir(), "child")
			killOnCleanup(t, pidFile)
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			start := time.Now()
			_ = runner.Feed(ctx, "x", "sh", "-c", `sleep 30 & echo $! >"$0"; wait`, pidFile)

			t.Run("it should return within the deadline plus the wait delay", func(t *testing.T) {
				assert.Less(t, time.Since(start), 2*time.Second)
			})
		})
	})

	t.Run("when claude prints its job id and exits while a child it started keeps stdout open", func(t *testing.T) {
		pidFile := filepath.Join(t.TempDir(), "child")
		killOnCleanup(t, pidFile)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		start := time.Now()
		out, err := runner.Run(ctx, "", "sh", "-c", `echo "backgrounded · 16daefa1"; sleep 30 & echo $! >"$0"; exit 0`, pidFile)
		took := time.Since(start)

		t.Run("it should return soon after claude exits", func(t *testing.T) {
			assert.Less(t, took, 3*time.Second)
		})

		t.Run("it should return what claude printed", func(t *testing.T) {
			assert.Contains(t, out, "backgrounded · 16daefa1")
		})

		t.Run("it should not call it a failure", func(t *testing.T) {
			assert.NoError(t, err)
		})
	})

	t.Run("when claude never exits", func(t *testing.T) {
		pidFile := filepath.Join(t.TempDir(), "child")
		killOnCleanup(t, pidFile)
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		start := time.Now()
		_, err := runner.Run(ctx, "", "sh", "-c", `sleep 30 & echo $! >"$0"; wait`, pidFile)
		took := time.Since(start)

		t.Run("it should return within the deadline plus the wait delay", func(t *testing.T) {
			assert.Less(t, took, 3*time.Second)
		})

		t.Run("it should return an error", func(t *testing.T) {
			assert.Error(t, err)
		})
	})

	t.Run("when the city holds a descriptor opened without close-on-exec", func(t *testing.T) {
		t.Run("and the system has no close_range, so the descriptors are swept one by one", func(t *testing.T) {
			fd, err := syscall.Open(os.DevNull, syscall.O_RDONLY, 0)
			require.NoError(t, err)
			t.Cleanup(func() { _ = syscall.Close(fd) })
			control.SweepOnExec()
			flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
			require.NoError(t, err)

			t.Run("it should mark it close-on-exec", func(t *testing.T) {
				assert.NotZero(t, flags&unix.FD_CLOEXEC)
			})
		})

		for _, c := range []struct {
			name string
			run  func(probe, out string) error
		}{
			{"Feed", func(probe, out string) error {
				return runner.Feed(context.Background(), "", "sh", "-c", probe, out)
			}},
			{"Run", func(probe, out string) error {
				_, err := runner.Run(context.Background(), "", "sh", "-c", probe, out)
				return err
			}},
			{"Start", func(probe, out string) error { return runner.Start("", "sh", "-c", probe, out) }},
			{"Read", func(probe, out string) error {
				_, err := runner.Read(context.Background(), "sh", "-c", probe, out)
				return err
			}},
		} {
			t.Run("and the tool is run through "+c.name, func(t *testing.T) {
				fd, err := syscall.Open(os.DevNull, syscall.O_RDONLY, 0)
				require.NoError(t, err)
				t.Cleanup(func() { _ = syscall.Close(fd) })
				probe := `if [ -e /dev/fd/` + strconv.Itoa(fd) + ` ]; then echo leaked; else echo closed; fi >"$0"`
				out := filepath.Join(t.TempDir(), "out")
				require.NoError(t, c.run(probe, out))
				require.Eventually(t, func() bool {
					got, _ := os.ReadFile(out)
					return strings.HasSuffix(string(got), "\n")
				}, 5*time.Second, 10*time.Millisecond)
				got, _ := os.ReadFile(out)

				t.Run("it should not reach the tool", func(t *testing.T) {
					assert.Equal(t, "closed\n", string(got))
				})
			})
		}
	})
}
