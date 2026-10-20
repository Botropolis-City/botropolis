package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 186, from item 167: at a load of about 150, pkg/render hit go test's
// default ten-minute timeout, the fault item 131 fixed for acceptance alone.
func TestTestTimeouts(t *testing.T) {
	for _, target := range []string{"test-unit", "test-integration", "test-acceptance"} {
		t.Run("when "+target+" is dry-run with a timeout", func(t *testing.T) {
			cmd := exec.Command("make", "-n", "-C", repoRoot(t), target, "TEST_TIMEOUT=17m")
			cmd.Env = landEnviron()
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, string(out))
			var runs, timed []string
			for _, line := range strings.Split(string(out), "\n") {
				if !strings.Contains(line, "go test") {
					continue
				}
				runs = append(runs, line)
				if strings.Contains(line, "-timeout 17m") {
					timed = append(timed, line)
				}
			}
			require.NotEmpty(t, runs)

			t.Run("it should give every go test the timeout", func(t *testing.T) {
				assert.Equal(t, runs, timed)
			})
		})
	}
}

// Item 186: golangci-lint takes a lock in TMPDIR for the whole machine and,
// by default, gives up on it after five seconds, so a lint failed whenever
// another session was linting. The test holds that lock longer than that in
// a TMPDIR of its own and runs the Makefile's golangci-lint, with its default
// flags, over a two-file module.
func TestLintWaitsForLint(t *testing.T) {
	if testing.Short() {
		t.Skip("runs golangci-lint")
	}
	t.Parallel()
	const hold = 8 * time.Second

	t.Run("when another golangci-lint holds the lock for longer than golangci-lint waits by default", func(t *testing.T) {
		root := repoRoot(t)
		dir := t.TempDir()
		for _, name := range []string{"Makefile", "VERSION", ".golangci.yml", "tools/lint/go.mod", "tools/lint/go.sum"} {
			data, err := os.ReadFile(filepath.Join(root, name))
			require.NoError(t, err)
			require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o644))
		}
		mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
		require.NoError(t, err)
		var head []string
		for _, line := range strings.Split(string(mod), "\n") {
			if strings.HasPrefix(line, "go ") || strings.HasPrefix(line, "toolchain ") {
				head = append(head, line)
			}
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module scratch\n\n"+strings.Join(head, "\n")+"\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("// Package scratch is linted.\npackage scratch\n\n// A is one.\nconst A = 1\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("package scratch\n\n// B is two.\nconst B = A + 1\n"), 0o644))

		tmp := t.TempDir()
		lock, err := os.Create(filepath.Join(tmp, "golangci-lint.lock"))
		require.NoError(t, err)
		t.Cleanup(func() { _ = lock.Close() })
		require.NoError(t, syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB))
		start := time.Now()
		go func() {
			time.Sleep(hold)
			_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		}()

		cmd := exec.Command("make", "-s", "-C", dir, "lint-golangci")
		cmd.Env = append(landEnviron(), "TMPDIR="+tmp, "GOLANGCI_LINT_CACHE="+filepath.Join(tmp, "golangci-lint-cache"))
		out, err := cmd.CombinedOutput()
		took := time.Since(start)

		t.Run("it should pass", func(t *testing.T) {
			assert.NoError(t, err, string(out))
		})

		t.Run("it should have waited for the lock", func(t *testing.T) {
			assert.GreaterOrEqual(t, took, hold)
		})
	})
}
