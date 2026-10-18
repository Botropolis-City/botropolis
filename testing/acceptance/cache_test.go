package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const module = "botropolis.city/botropolis/"

func goList(t *testing.T, args ...string) map[string]bool {
	t.Helper()
	out, err := exec.Command("go", append([]string{"list", "-deps", "-f", "{{.ImportPath}}"}, args...)...).CombinedOutput()
	require.NoError(t, err, string(out))
	pkgs := map[string]bool{}
	for _, line := range strings.Fields(string(out)) {
		if strings.HasPrefix(line, module) {
			pkgs[line] = true
		}
	}
	return pkgs
}

func goTestIn(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("go", "test", "./testing/acceptance", "-run", "^TestStatus$")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return string(out)
}

func scratchModule(t *testing.T) string {
	t.Helper()
	root := filepath.Join("..", "..")
	ls := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "go.mod", "go.sum", "cmd", "pkg", "testing")
	ls.Dir = root
	out, err := ls.Output()
	require.NoError(t, err)
	dir := t.TempDir()
	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		data, err := os.ReadFile(filepath.Join(root, name))
		if os.IsNotExist(err) {
			continue
		}
		require.NoError(t, err)
		info, err := os.Stat(filepath.Join(root, name))
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, info.Mode().Perm()))
	}
	return dir
}

type probe struct {
	where string
	file  string
	sig   string
}

var probes = []probe{
	{
		where: "in a package the acceptance tests do not import",
		file:  "pkg/app/module.go",
		sig:   "func Run(ctx context.Context, args []string) int {\n",
	},
	{
		where: "under pkg/render that the tests never call",
		file:  "pkg/render/run.go",
		sig:   "func Run(ctx context.Context, opts Options) error {\n",
	},
}

func (p probe) change(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, p.file)
	src, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(src), p.sig, "the probe no longer finds where to change %s", p.file)
	changed := strings.Replace(string(src), p.sig, strings.TrimSuffix(p.sig, "\n")+" item117Probe++\n", 1) + "\nvar item117Probe int\n"
	require.NoError(t, os.WriteFile(path, []byte(changed), 0o644))
}

func TestTheCacheSeesTheBinary(t *testing.T) {
	t.Run("when the acceptance package's dependencies are compared with the binary's", func(t *testing.T) {
		binary := goList(t, "../../cmd/botropolis")
		delete(binary, module+"cmd/botropolis")
		tests := goList(t, "-test", ".")
		var missing []string
		for pkg := range binary {
			if !tests[pkg] {
				missing = append(missing, pkg)
			}
		}
		sort.Strings(missing)

		t.Run("it should depend on every in-module package cmd/botropolis links", func(t *testing.T) {
			assert.Empty(t, missing, "binary_test.go must reach the binary entry point, so the test cache keys on its code")
		})
	})

	t.Run("when the code under test is copied to a scratch module and TestStatus is run there", func(t *testing.T) {
		if testing.Short() {
			t.Skip("runs go test three times")
		}
		dir := scratchModule(t)
		goTestIn(t, dir)
		second := goTestIn(t, dir)

		t.Run("and it is run a second time with nothing changed", func(t *testing.T) {
			t.Run("it should report (cached)", func(t *testing.T) {
				assert.Contains(t, second, "(cached)")
			})
		})

		for _, p := range probes {
			p.change(t, dir)
			out := goTestIn(t, dir)

			t.Run("and code "+p.where+" is then changed", func(t *testing.T) {
				t.Run("it should not report (cached)", func(t *testing.T) {
					assert.NotContains(t, out, "(cached)")
				})
			})
		}
	})
}
