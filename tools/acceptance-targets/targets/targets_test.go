package targets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mod = "botropolis.city/botropolis"

func TestPackageOf(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"pkg/state/state.go", "pkg/assets/kits/kits.go", "pkg/assets/kits/roads/a.png", "tools/lint/go.mod", "cmd/botropolis/main.go"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.Dir(f)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, f), nil, 0o644))
	}

	for _, c := range []struct {
		when, file, pkg string
		ok              bool
	}{
		{"when a changed file is a Go file under pkg/", "pkg/state/state.go", mod + "/pkg/state", true},
		{"when a changed file is a Go file under cmd/", "cmd/botropolis/main.go", mod + "/cmd/botropolis", true},
		{"when a changed file is an embedded asset in a subdirectory", "pkg/assets/kits/roads/a.png", mod + "/pkg/assets/kits", true},
		{"when a changed file was deleted from a package", "pkg/state/gone.go", mod + "/pkg/state", true},
		{"when a changed file is outside pkg/ and cmd/", "tools/lint/go.mod", "", false},
		{"when a changed file is at the root", "go.mod", "", false},
	} {
		t.Run(c.when, func(t *testing.T) {
			pkg, ok := PackageOf(root, mod, c.file)

			t.Run("it should say whether it maps", func(t *testing.T) {
				assert.Equal(t, c.ok, ok)
			})

			t.Run("it should name the nearest package above it", func(t *testing.T) {
				assert.Equal(t, c.pkg, pkg)
			})
		})
	}
}

func TestChoose(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"pkg/state/state.go", "pkg/render/run.go", "pkg/find/find.go"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.Dir(f)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, f), nil, 0o644))
	}
	pkgOf := func(file string) (string, bool) { return PackageOf(root, mod, file) }
	m := &Map{Rev: "abc", Tests: map[string][]string{
		"TestStatus": {mod + "/pkg/state", mod + "/pkg/cli"},
		"TestDrive":  {mod + "/pkg/render"},
		"TestDocs":   {},
		"TestFind":   {mod + "/pkg/find"},
	}}
	tests := []string{"TestDocs", "TestDrive", "TestFind", "TestStatus"}

	t.Run("when there is no map", func(t *testing.T) {
		c := Choose(nil, tests, []string{"pkg/state/state.go"}, pkgOf)

		t.Run("it should fall back, saying the map is missing", func(t *testing.T) {
			assert.Contains(t, c.Fallback, "no map")
		})
	})

	t.Run("when the test list differs from the map's", func(t *testing.T) {
		c := Choose(m, []string{"TestDocs", "TestDrive", "TestFind", "TestNew"}, []string{"pkg/state/state.go"}, pkgOf)

		t.Run("it should name the added test", func(t *testing.T) {
			assert.Contains(t, c.Fallback, "TestNew")
		})

		t.Run("it should name the removed test", func(t *testing.T) {
			assert.Contains(t, c.Fallback, "TestStatus")
		})
	})

	t.Run("when a changed file is unmapped", func(t *testing.T) {
		c := Choose(m, tests, []string{"pkg/state/state.go", "Makefile"}, pkgOf)

		t.Run("it should fall back, naming the file", func(t *testing.T) {
			assert.Contains(t, c.Fallback, "Makefile")
		})
	})

	t.Run("when the changed packages are mapped", func(t *testing.T) {
		c := Choose(m, tests, []string{"pkg/state/state.go"}, pkgOf)

		t.Run("it should not fall back", func(t *testing.T) {
			assert.Empty(t, c.Fallback)
		})

		t.Run("it should select every test that touched one, and every test whose package list is empty, and nothing else", func(t *testing.T) {
			assert.Equal(t, []string{"TestDocs", "TestStatus"}, c.Tests)
		})
	})

	t.Run("when nothing changed", func(t *testing.T) {
		c := Choose(m, tests, nil, pkgOf)

		t.Run("it should select no tests", func(t *testing.T) {
			assert.Empty(t, c.Tests)
		})

		t.Run("it should say so", func(t *testing.T) {
			assert.Contains(t, c.Note, "nothing changed")
		})
	})
}

func TestTouchedPackages(t *testing.T) {
	t.Run("when covdata prints a profile", func(t *testing.T) {
		profile := "mode: set\n" +
			mod + "/pkg/state/state.go:10.2,12.3 2 1\n" +
			mod + "/pkg/state/live.go:4.1,5.2 1 0\n" +
			mod + "/pkg/render/run.go:1.1,2.2 1 0\n" +
			mod + "/pkg/cli/root.go:3.1,4.2 1 7\n" +
			"example.com/other/x.go:1.1,2.2 1 3\n"

		t.Run("it should keep the in-module packages with a counter above zero", func(t *testing.T) {
			assert.Equal(t, []string{mod + "/pkg/cli", mod + "/pkg/state"}, TouchedPackages(profile, mod))
		})
	})
}
