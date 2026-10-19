package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type probe struct {
	where string
	file  string
	edit  func(src string) (string, bool)
}

func funcProbe(where, file, sig string) probe {
	return probe{where, file, func(src string) (string, bool) {
		if !strings.Contains(src, sig) {
			return src, false
		}
		return strings.Replace(src, sig, strings.TrimSuffix(sig, "\n")+" item117Probe++\n", 1) + "\nvar item117Probe int\n", true
	}}
}

func appendProbe(where, file string) probe {
	return probe{where, file, func(src string) (string, bool) {
		return src + "# item 117 probe\n", true
	}}
}

func (p probe) change(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, p.file)
	src, err := os.ReadFile(path)
	require.NoError(t, err)
	changed, ok := p.edit(string(src))
	require.True(t, ok, "the probe no longer finds where to change %s", p.file)
	require.NoError(t, os.WriteFile(path, []byte(changed), 0o644))
}

var binaryProbes = []probe{
	funcProbe("in a package the tests do not import", "pkg/app/module.go",
		"func Run(ctx context.Context, args []string) int {\n"),
	funcProbe("under pkg/render that the tests never call", "pkg/render/run.go",
		"func Run(ctx context.Context, opts Options) error {\n"),
}

// cacheRun is one go test invocation run in a scratch copy of the module,
// three ways: once, again unchanged, and again after each probe.
type cacheRun struct {
	name   string
	pkg    string
	run    string
	extra  []string
	probes []probe
}

func scratchModule(t *testing.T, extra ...string) string {
	t.Helper()
	root := filepath.Join("..", "..")
	args := append([]string{"ls-files", "--cached", "--others", "--exclude-standard", "go.mod", "go.sum", "cmd", "pkg", "testing", "tools/acceptance-targets"}, extra...)
	ls := exec.Command("git", args...)
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

func goTestIn(t *testing.T, dir, pkg, run string) string {
	t.Helper()
	cmd := exec.Command("go", "test", pkg, "-run", run)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return string(out)
}

func (c cacheRun) assert(t *testing.T) {
	t.Run("when "+c.name+" is run in a scratch copy of the module", func(t *testing.T) {
		if testing.Short() {
			t.Skip("runs go test several times")
		}
		dir := scratchModule(t, c.extra...)
		goTestIn(t, dir, c.pkg, c.run)
		second := goTestIn(t, dir, c.pkg, c.run)

		t.Run("and it is run a second time with nothing changed", func(t *testing.T) {
			t.Run("it should report (cached)", func(t *testing.T) {
				assert.Contains(t, second, "(cached)")
			})
		})

		for _, p := range c.probes {
			p.change(t, dir)
			out := goTestIn(t, dir, c.pkg, c.run)

			t.Run("and code "+p.where+" is then changed", func(t *testing.T) {
				t.Run("it should not report (cached)", func(t *testing.T) {
					assert.NotContains(t, out, "(cached)")
				})
			})
		}
	})
}

func TestTheCacheSeesTheBinary(t *testing.T) {
	for _, c := range []cacheRun{
		{name: "the acceptance tests' TestStatus", pkg: "./testing/acceptance", run: "^TestStatus$", probes: binaryProbes},
		{name: "the packaging tests' TestCompletionScripts", pkg: "./testing/integration/packaging", run: "^TestCompletionScripts$", probes: binaryProbes},
	} {
		c.assert(t)
	}
}
