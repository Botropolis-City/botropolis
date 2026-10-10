package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var botropolis string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "botropolis-acceptance")
	if err != nil {
		panic(err)
	}
	botropolis = filepath.Join(dir, "botropolis")
	build := exec.Command("go", "build", "-o", botropolis, "../../cmd/botropolis")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestStatus(t *testing.T) {
	t.Run("when a user runs status against the sample fixture", func(t *testing.T) {
		home := helpers.LiveFixtureHome(t, "sample")
		out, err := exec.Command(botropolis, "status", "--home", home).CombinedOutput()
		require.NoError(t, err, string(out))
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")

		t.Run("it should print a header", func(t *testing.T) {
			assert.True(t, strings.HasPrefix(lines[0], "STATE"), lines[0])
		})

		t.Run("it should print one row per live session", func(t *testing.T) {
			assert.Len(t, lines[1:], helpers.Manifest(t, "sample").LiveSessions)
		})

		t.Run("it should give every row a state", func(t *testing.T) {
			names := make([]string, 0, len(state.Order)+1)
			for _, st := range append(state.Order, state.Empty) {
				names = append(names, string(st))
			}
			for _, line := range lines[1:] {
				assert.Regexp(t, `^(`+strings.Join(names, "|")+`)\s`, line)
			}
		})
	})

	t.Run("when a user runs status against an empty home", func(t *testing.T) {
		out, err := exec.Command(botropolis, "status", "--home", t.TempDir()).CombinedOutput()

		t.Run("it should succeed", func(t *testing.T) {
			assert.NoError(t, err, string(out))
		})

		t.Run("it should say there are no sessions", func(t *testing.T) {
			assert.Equal(t, "no sessions\n", string(out))
		})
	})
}
