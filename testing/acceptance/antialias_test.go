package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

// No world shape is drawn with an anti-aliased vector call (item 191).
// tools/check-antialias is run by make lint; this is its second caller,
// against the tree and against a copy that gains such a call.

func runAntialias(t *testing.T, root string) (string, error) {
	t.Helper()
	cmd := exec.Command(helpers.Tool(t, "check-antialias"), "--root", root)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestCheckAntialias(t *testing.T) {
	t.Run("when it runs against the tree", func(t *testing.T) {
		root := repoRoot(t)
		walkRepo(t, root, func(rel, path string) {
			if filepath.Ext(path) == ".go" || rel == filepath.Join("tools", "antialias", "allow.txt") {
				_, _ = os.ReadFile(path)
			}
		})
		out, err := runAntialias(t, root)

		t.Run("it should pass", func(t *testing.T) {
			assert.NoError(t, err, out)
		})
	})

	t.Run("when a copy of the tree gains an anti-aliased call", func(t *testing.T) {
		root := t.TempDir()
		src := filepath.Join(repoRoot(t), "pkg", "shapes")
		entries, err := os.ReadDir(src)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Join(root, "pkg", "shapes"), 0o755))
		for _, e := range entries {
			data, err := os.ReadFile(filepath.Join(src, e.Name()))
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(root, "pkg", "shapes", e.Name()), data, 0o644))
		}
		allow, err := os.ReadFile(filepath.Join(repoRoot(t), "tools", "antialias", "allow.txt"))
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Join(root, "tools", "antialias"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, "tools", "antialias", "allow.txt"), allow, 0o644))
		require.NoError(t, os.MkdirAll(filepath.Join(root, "pkg", "render"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, "pkg", "render", "spark.go"), []byte(`package render

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func spark(dst *ebiten.Image) {
	vector.FillCircle(dst, 1, 2, 3, color.White, true)
}
`), 0o644))
		out, err := runAntialias(t, root)

		t.Run("it should fail", func(t *testing.T) {
			assert.Error(t, err, out)
		})

		t.Run("it should name the call", func(t *testing.T) {
			assert.Contains(t, out, "pkg/render/spark.go:11: vector.FillCircle with anti-aliasing on")
		})
	})
}
