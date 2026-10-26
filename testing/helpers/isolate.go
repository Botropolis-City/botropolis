package helpers

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
)

// HomeVariables are every variable the code reads a home or a per-user
// directory from. Item 92.
var HomeVariables = []string{
	"XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME",
	"XDG_RUNTIME_DIR", "CLAUDE_CONFIG_DIR",
}

// RunIsolated runs a package's tests with HOME and every variable in
// HomeVariables pointed at an empty temp dir, and every variable in
// ConfigVariables unset, so nothing reads this machine's Claude home or
// the builder's own config (item 263). Go's own caches are pinned first, because the
// tests build binaries and those caches default to the real home.
func RunIsolated(m interface{ Run() int }) int {
	pinned := []string{"GOCACHE", "GOMODCACHE", "GOPATH", "GOENV"}
	out, err := exec.Command("go", append([]string{"env"}, pinned...)...).Output()
	if err != nil {
		panic(fmt.Sprintf("go env: %v", err))
	}
	for i, value := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		_ = os.Setenv(pinned[i], value)
	}
	home, err := os.MkdirTemp("", "botropolis-home-")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(home) }()
	_ = os.Setenv("HOME", home)
	for _, name := range HomeVariables {
		dir := filepath.Join(home, strings.ToLower(name))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			panic(err)
		}
		_ = os.Setenv(name, dir)
	}
	for _, name := range ConfigVariables() {
		_ = os.Unsetenv(name)
	}
	return m.Run()
}

// AssertIsolated is the guard every integration and acceptance package
// runs, so a test that reaches for the real home fails on any machine.
func AssertIsolated(t *testing.T) {
	t.Run("when a test runs", func(t *testing.T) {
		home := os.Getenv("HOME")
		me, err := user.Current()
		require.NoError(t, err)

		t.Run("it should point HOME away from the real home", func(t *testing.T) {
			assert.NotEqual(t, filepath.Clean(me.HomeDir), filepath.Clean(home))
		})

		t.Run("it should point HOME into the temp dir", func(t *testing.T) {
			assert.True(t, strings.HasPrefix(home, filepath.Clean(os.TempDir())+string(filepath.Separator)), home)
		})

		t.Run("it should find no Claude home in HOME", func(t *testing.T) {
			assert.NoDirExists(t, filepath.Join(home, ".claude"))
		})

		for _, name := range HomeVariables {
			t.Run("it should point "+name+" inside HOME", func(t *testing.T) {
				assert.True(t, strings.HasPrefix(os.Getenv(name), home+string(filepath.Separator)), os.Getenv(name))
			})
		}

		for _, name := range ConfigVariables() {
			t.Run("it should leave "+name+" unset", func(t *testing.T) {
				_, set := os.LookupEnv(name)
				assert.False(t, set)
			})
		}
	})

	t.Run("when the config variables are listed", func(t *testing.T) {
		names := ConfigVariables()

		for _, name := range []string{"BOTROPOLIS_DISTRICT_TYPE", "BOTROPOLIS_CONFIG", "BOTROPOLIS_SET"} {
			t.Run("it should list "+name, func(t *testing.T) {
				assert.Contains(t, names, name)
			})
		}
	})
}

// ConfigVariables are every BOTROPOLIS_ variable pkg/config reads, taken
// from its own key list so a new key cannot escape isolation. Item 263.
func ConfigVariables() []string {
	var names []string
	for _, key := range []string{config.KeyConfig, config.KeySet} {
		names = append(names, "BOTROPOLIS_"+strings.ToUpper(key))
	}
	for _, doc := range config.Docs() {
		names = append(names, doc.Env)
	}
	return names
}
