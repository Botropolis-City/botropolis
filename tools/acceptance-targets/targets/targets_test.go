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
	resolve := Resolver{
		PackageOf: func(file string) (string, bool) { return PackageOf(root, mod, file) },
		Acceptance: func(file string) ([]string, string) {
			switch file {
			case "testing/acceptance/drive_test.go":
				return []string{"TestDrive", "TestFind"}, ""
			case "testing/acceptance/status_test.go":
				return nil, "TestMain changed"
			}
			return nil, file + " is not Go"
		},
	}
	m := &Map{Rev: "abc", Tests: map[string][]string{
		"TestStatus": {mod + "/pkg/state", mod + "/pkg/cli"},
		"TestDrive":  {mod + "/pkg/render"},
		"TestDocs":   {},
		"TestFind":   {mod + "/pkg/find"},
	}}
	tests := []string{"TestDocs", "TestDrive", "TestFind", "TestStatus"}
	at := func(changed ...string) Situation {
		return Situation{CI: "green", Map: m, Behind: 3, Tests: tests, Changed: changed}
	}

	for _, c := range []struct {
		when     string
		s        Situation
		fallback string
	}{
		{"when there is no map", Situation{CI: "green", Tests: tests, Changed: []string{"pkg/state/state.go"}}, "no map"},
		{"when main's CI is red", Situation{CI: "red", Map: m, Behind: 3, Tests: tests, Changed: []string{"pkg/state/state.go"}}, "main's CI is red"},
		{"when main's CI is unknown", Situation{CI: "unknown", Map: m, Behind: 3, Tests: tests, Changed: []string{"pkg/state/state.go"}}, "main's CI is unknown"},
		{"when the map is more than 30 commits behind HEAD", Situation{CI: "green", Map: m, Behind: 31, Tests: tests, Changed: []string{"pkg/state/state.go"}}, "31 commits behind"},
		{"when the map's revision is not an ancestor of HEAD", Situation{CI: "green", Map: m, Behind: -1, Tests: tests, Changed: []string{"pkg/state/state.go"}}, "not an ancestor"},
		{"when a test was added since the map", Situation{CI: "green", Map: m, Behind: 3, Tests: append(tests, "TestNew"), Changed: []string{"pkg/state/state.go"}}, "TestNew"},
		{"when a changed file is unmapped", at("pkg/state/state.go", "Makefile"), "Makefile"},
		{"when TestMain changed", at("testing/acceptance/status_test.go"), "TestMain changed"},
	} {
		t.Run(c.when, func(t *testing.T) {
			got := Choose(c.s, resolve)

			t.Run("it should run the full suite, saying why", func(t *testing.T) {
				assert.Contains(t, got.Fallback, c.fallback)
			})
		})
	}

	t.Run("when the changed packages are mapped", func(t *testing.T) {
		c := Choose(at("pkg/state/state.go"), resolve)

		t.Run("it should not fall back", func(t *testing.T) {
			assert.Empty(t, c.Fallback)
		})

		t.Run("it should select every test that touched one, and every test whose package list is empty, and nothing else", func(t *testing.T) {
			assert.Equal(t, []string{"TestDocs", "TestStatus"}, c.Tests)
		})
	})

	t.Run("when only an acceptance test file changed", func(t *testing.T) {
		c := Choose(at("testing/acceptance/drive_test.go"), resolve)

		t.Run("it should select the tests that reach what changed, and every test whose package list is empty", func(t *testing.T) {
			assert.Equal(t, []string{"TestDocs", "TestDrive", "TestFind"}, c.Tests)
		})
	})

	t.Run("when nothing changed", func(t *testing.T) {
		c := Choose(at(), resolve)

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
