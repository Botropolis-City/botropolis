package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
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
func TestLintChecksFormatting(t *testing.T) {
	t.Run("when make lint runs in a scratch copy of the tree", func(t *testing.T) {
		if testing.Short() {
			t.Skip("copies the tree and runs make lint")
		}
		dir := scratchModule(t, ".")

		t.Run("and a Go file is unformatted", func(t *testing.T) {
			const probe = "pkg/render/zz_unformatted.go"
			require.NoError(t, os.WriteFile(filepath.Join(dir, probe), []byte("package render\n\nconst  Unformatted = 1\n"), 0o644))
			cmd := exec.Command("make", "-s", "-C", dir, "lint")
			cmd.Env = append(os.Environ(), "GOFLAGS=")
			out, err := cmd.CombinedOutput()

			t.Run("it should fail", func(t *testing.T) {
				assert.Error(t, err, string(out))
			})

			t.Run("it should name the file", func(t *testing.T) {
				assert.Contains(t, string(out), probe)
			})
		})
	})
}
