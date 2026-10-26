package acceptance_test

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// sameSizeProbe changes a file without changing its size, so only its
// modification time can tell the test cache it changed (item 209).
func sameSizeProbe(where, file string) probe {
	return probe{where, file, func(src string) (string, bool) {
		if !strings.HasSuffix(src, "\n") {
			return src, false
		}
		return strings.TrimSuffix(src, "\n") + " ", true
	}}
}

func (p probe) change(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, p.file)
	src, err := os.ReadFile(path)
	require.NoError(t, err)
	changed, ok := p.edit(string(src))
	require.True(t, ok, "the probe no longer finds where to change %s", p.file)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(changed), 0o644))
	at := info.ModTime().Add(time.Minute)
	require.NoError(t, os.Chtimes(path, at, at))
}

// copiedAt is when a scratch copy says its files were written: long
// enough ago that go test will cache a run that opens them, which it
// refuses for a file modified under 2 s before. Its cache key holds an
// opened file's size, mode and modification time, not its contents, so
// each probe moves the time on by a minute, which a change of the same
// size needs to be seen (cmd/go/internal/test: hashOpen, hashWriteStat).
func copiedAt() time.Time {
	return time.Now().Add(-time.Hour)
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
	at := copiedAt()
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
		require.NoError(t, os.Chtimes(filepath.Join(dir, name), at, at))
	}
	require.NoError(t, filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == dir {
			return err
		}
		return os.Chtimes(path, at, at)
	}))
	return dir
}

// scratchGoTest runs go test in a scratch copy with gocachetest on, so a
// run that misses the cache prints go's reason into the output a failing
// assertion shows (item 256).
func scratchGoTest(dir, pkg, run string) *exec.Cmd {
	cmd := exec.Command("go", "test", pkg, "-run", run)
	cmd.Dir = dir
	godebug := "gocachetest=1"
	if prior := os.Getenv("GODEBUG"); prior != "" {
		godebug = prior + "," + godebug
	}
	cmd.Env = append(os.Environ(), "GOFLAGS=", "GODEBUG="+godebug)
	return cmd
}

func goTestIn(t *testing.T, dir, pkg, run string) string {
	t.Helper()
	out, err := scratchGoTest(dir, pkg, run).CombinedOutput()
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

// Item 256: a second run that misses the cache now and then, at random,
// says why, in go's own words, in the failure it causes.
func TestScratchGoTest(t *testing.T) {
	t.Run("when a scratch copy's go test command is built", func(t *testing.T) {
		env := map[string]string{}
		for _, kv := range scratchGoTest(t.TempDir(), "./testing/acceptance", "^TestStatus$").Env {
			k, v, _ := strings.Cut(kv, "=")
			env[k] = v
		}

		t.Run("it should clear GOFLAGS", func(t *testing.T) {
			assert.Equal(t, "", env["GOFLAGS"])
		})

		t.Run("it should ask go to say why it misses the test cache", func(t *testing.T) {
			assert.Contains(t, strings.Split(env["GODEBUG"], ","), "gocachetest=1")
		})
	})
}

// Item 209: go test caches no run that opened a file modified under 2 s
// before, so a scratch copy made just before a run whose build is
// already cached is never cached, at random.
func TestScratchModule(t *testing.T) {
	const cutoff = 2 * time.Second
	age := func(t *testing.T, path string) time.Duration {
		t.Helper()
		info, err := os.Stat(path)
		require.NoError(t, err)
		return time.Since(info.ModTime())
	}

	t.Run("when a scratch copy of the module is made", func(t *testing.T) {
		dir := scratchModule(t, "tools")
		var young, youngDirs []string
		require.NoError(t, filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err == nil && d.Type().IsRegular() && age(t, path) < cutoff {
				young = append(young, path)
			}
			if err == nil && d.IsDir() && path != dir && age(t, path) < cutoff {
				youngDirs = append(youngDirs, path)
			}
			return err
		}))

		t.Run("it should leave every file older than go test's 2 s freshness cutoff", func(t *testing.T) {
			assert.Zero(t, len(young), "%d files younger than 2 s, the first %v", len(young), append(young, "")[0])
		})

		// Item 256: go hashes a directory's time and its entries' times
		// too, so the copy's directories are dated with its files.
		t.Run("it should leave every directory older than the cutoff too", func(t *testing.T) {
			assert.Zero(t, len(youngDirs), "%d directories younger than 2 s, the first %v", len(youngDirs), append(youngDirs, "")[0])
		})

		t.Run("and a probe then changes a file in it", func(t *testing.T) {
			p := sameSizeProbe("in tools/check-asset-sources", "tools/check-asset-sources")
			path := filepath.Join(dir, p.file)
			copied := age(t, path)
			p.change(t, dir)
			changed := age(t, path)

			t.Run("it should leave that file older than the cutoff too", func(t *testing.T) {
				assert.GreaterOrEqual(t, changed, cutoff)
			})

			t.Run("it should give it a modification time apart from the copy's", func(t *testing.T) {
				assert.Less(t, changed, copied-time.Second)
			})
		})
	})
}
