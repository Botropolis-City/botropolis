package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

// Every image under pkg/assets must have a traceable source (item 123,
// design/art-sources.md). tools/check-asset-sources is run by make lint; this
// is its second caller, against the tree and against mutated fixtures.

var (
	pngMagic  = []byte("\x89PNG\r\n\x1a\nfixture")
	jpegMagic = []byte("\xff\xd8\xff\xe0fixture")
)

func runAssetSources(t *testing.T, root string) (string, error) {
	t.Helper()
	out, err := exec.Command(helpers.Tool(t, "check-asset-sources"), "--root", root).CombinedOutput()
	return string(out), err
}

func writeFixture(t *testing.T, root string, files map[string][]byte) {
	t.Helper()
	for name, data := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, data, 0o644))
	}
}

// assetsFixture is a tree that passes: one licensed pack and one atlas page
// its manifest names.
func assetsFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, map[string][]byte{
		"pkg/assets/pack/License.txt":   []byte("CC0 1.0 Universal\n"),
		"pkg/assets/pack/sheet.png":     pngMagic,
		"pkg/assets/kits/kits-z1.json":  []byte(`{"pages": ["kits-z1-0.png"]}`),
		"pkg/assets/kits/kits-z1-0.png": pngMagic,
	})
	return root
}

func readAssets(t *testing.T) {
	t.Helper()
	require.NoError(t, filepath.WalkDir(filepath.Join(repoRoot(t), "pkg", "assets"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		_, err = os.ReadFile(p)
		return err
	}))
}

func TestAssetSources(t *testing.T) {
	t.Run("when the repository's pkg/assets is checked", func(t *testing.T) {
		readAssets(t)
		out, err := runAssetSources(t, repoRoot(t))

		t.Run("it should pass", func(t *testing.T) {
			assert.NoError(t, err, out)
		})
	})

	t.Run("when a fixture with a licensed pack and a manifested atlas page is checked", func(t *testing.T) {
		out, err := runAssetSources(t, assetsFixture(t))

		t.Run("it should pass", func(t *testing.T) {
			assert.NoError(t, err, out)
		})
	})

	for _, m := range []struct {
		scenario string
		named    string
		files    map[string][]byte
	}{
		{"an unlicensed PNG sits in a new directory", "pkg/assets/stray/new.png",
			map[string][]byte{"pkg/assets/stray/new.png": pngMagic}},
		{"an unlicensed JPEG sits in a new directory", "pkg/assets/stray/photo.jpg",
			map[string][]byte{"pkg/assets/stray/photo.jpg": jpegMagic}},
		{"an unlicensed SVG sits in a new directory", "pkg/assets/stray/logo.svg",
			map[string][]byte{"pkg/assets/stray/logo.svg": []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)}},
		{"a PNG renamed .bin sits in an unlicensed directory", "pkg/assets/stray/blob.bin",
			map[string][]byte{"pkg/assets/stray/blob.bin": pngMagic}},
		{"a PNG beside the atlas pages is named by no manifest", "pkg/assets/kits/kits-z1-9.png",
			map[string][]byte{"pkg/assets/kits/kits-z1-9.png": pngMagic}},
		{"a page is named by a manifest outside kits/", "pkg/assets/other/kits-z1-0.png",
			map[string][]byte{
				"pkg/assets/other/kits-z1.json":  []byte(`{"pages": ["kits-z1-0.png"]}`),
				"pkg/assets/other/kits-z1-0.png": pngMagic,
			}},
		{"a licensed directory's licence file is empty", "pkg/assets/pack/sheet.png",
			map[string][]byte{"pkg/assets/pack/License.txt": {}}},
	} {
		t.Run("when "+m.scenario, func(t *testing.T) {
			root := assetsFixture(t)
			writeFixture(t, root, m.files)
			out, err := runAssetSources(t, root)

			t.Run("it should fail", func(t *testing.T) {
				assert.Error(t, err, out)
			})

			t.Run("it should name the file", func(t *testing.T) {
				assert.Regexp(t, regexp.QuoteMeta(m.named), out)
			})
		})
	}

	t.Run("when pkg/assets holds no images", func(t *testing.T) {
		root := t.TempDir()
		writeFixture(t, root, map[string][]byte{"pkg/assets/pack/License.txt": []byte("CC0\n")})
		out, err := runAssetSources(t, root)

		t.Run("it should fail", func(t *testing.T) {
			assert.Error(t, err, out)
		})
	})

	t.Run("when pkg/assets is missing", func(t *testing.T) {
		out, err := runAssetSources(t, t.TempDir())

		t.Run("it should fail", func(t *testing.T) {
			assert.Error(t, err, out)
		})
	})
}
