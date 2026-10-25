package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
)

func TestSynthHome(t *testing.T) {
	end := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	t.Setenv("HOME", filepath.Join(dir, "user"))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	synth := func(home string, args ...string) (int, string) {
		var out, errOut bytes.Buffer
		code := run(append([]string{"synth-home", "--home", home, "--end", end.Format(time.RFC3339)}, args...), &out, &errOut)
		return code, out.String() + errOut.String()
	}
	tree := func(t *testing.T, root string) map[string]string {
		files := map[string]string{}
		require.NoError(t, filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err == nil && info.Mode().IsRegular() {
				content, err := os.ReadFile(path)
				require.NoError(t, err)
				rel, _ := filepath.Rel(root, path)
				files[rel] = string(content)
			}
			return err
		}))
		return files
	}

	t.Run("when synth-home writes 200 sessions in 150 projects", func(t *testing.T) {
		home := filepath.Join(dir, "soak")
		code, out := synth(home, "--sessions", "200", "--projects", "150", "--seed", "248")

		t.Run("it should exit zero", func(t *testing.T) {
			assert.Equal(t, 0, code, out)
		})

		t.Run("it should print the counts it wrote", func(t *testing.T) {
			assert.Contains(t, out, "200 sessions in 150 projects")
		})

		t.Run("and the daemon's loader loads it", func(t *testing.T) {
			snapshot, err := state.Load(home, state.Probes{Alive: func(int) bool { return false }, Attached: func(string) bool { return false }}, end)
			require.NoError(t, err)
			projects := map[string]bool{}
			for _, s := range snapshot.Sessions {
				projects[s.CWD] = true
			}

			t.Run("it should find 200 sessions", func(t *testing.T) {
				assert.Len(t, snapshot.Sessions, 200)
			})

			t.Run("it should find them in 150 projects", func(t *testing.T) {
				assert.Len(t, projects, 150)
			})
		})

		t.Run("and it writes again from the same seed into a second home", func(t *testing.T) {
			again := filepath.Join(dir, "again")
			_, out := synth(again, "--sessions", "200", "--projects", "150", "--seed", "248")

			t.Run("it should write the same files", func(t *testing.T) {
				assert.Equal(t, tree(t, home), tree(t, again), out)
			})
		})

		t.Run("and it is asked to write there again", func(t *testing.T) {
			before := tree(t, home)
			code, _ := synth(home, "--sessions", "200", "--projects", "150", "--seed", "248")

			t.Run("it should refuse", func(t *testing.T) {
				assert.Equal(t, 1, code)
			})

			t.Run("it should leave the home as it was", func(t *testing.T) {
				assert.Equal(t, before, tree(t, home))
			})
		})
	})

	t.Run("when --home is the user's home, whose .claude is the real one", func(t *testing.T) {
		code, _ := synth(os.Getenv("HOME"), "--sessions", "20", "--projects", "5", "--seed", "248")

		t.Run("it should refuse", func(t *testing.T) {
			assert.Equal(t, 1, code)
		})

		t.Run("it should write nothing", func(t *testing.T) {
			assert.NoDirExists(t, filepath.Join(os.Getenv("HOME"), ".claude"))
		})
	})
}
