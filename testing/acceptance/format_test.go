package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 129. Two unformatted files sat on main behind a green make lint, so
// lint runs the gofmt formatter. The probe is exported so that unused, which
// would also name the file, cannot pass this test for it.
//
// scratchModule reads every file it copies, which puts .golangci.yml and the
// Makefile in the test cache's key.
//
// Item 133. golangci-lint takes a lock in TMPDIR for the whole machine, so the
// probe failed whenever another session was linting. The test holds that lock
// in the probe's own TMPDIR, so it blocks nobody else's lint.
func TestLintChecksFormatting(t *testing.T) {
	t.Run("when make lint runs in a scratch copy of the tree", func(t *testing.T) {
		if testing.Short() {
			t.Skip("copies the tree and runs make lint")
		}
		dir := scratchModule(t, ".")

		t.Run("and another golangci-lint holds the machine's lock", func(t *testing.T) {
			tmp := t.TempDir()
			lock, err := os.Create(filepath.Join(tmp, "golangci-lint.lock"))
			require.NoError(t, err)
			t.Cleanup(func() { _ = lock.Close() })
			require.NoError(t, syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB))

			t.Run("and a Go file is unformatted", func(t *testing.T) {
				const probe = "pkg/render/zz_unformatted.go"
				require.NoError(t, os.WriteFile(filepath.Join(dir, probe), []byte("package render\n\nconst  Unformatted = 1\n"), 0o644))
				cmd := exec.Command("make", "-s", "-C", dir, "lint", "GOLANGCI_LINT_FLAGS=--allow-parallel-runners")
				cmd.Env = append(os.Environ(), "GOFLAGS=", "TMPDIR="+tmp, "GOLANGCI_LINT_CACHE="+filepath.Join(tmp, "golangci-lint-cache"))
				out, err := cmd.CombinedOutput()

				t.Run("it should fail", func(t *testing.T) {
					assert.Error(t, err, string(out))
				})

				t.Run("it should name the file", func(t *testing.T) {
					assert.Contains(t, string(out), probe)
				})

				t.Run("it should not report a parallel run", func(t *testing.T) {
					assert.NotContains(t, string(out), "parallel golangci-lint is running")
				})
			})
		})
	})
}
