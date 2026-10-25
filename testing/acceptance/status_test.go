package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var botropolis string

func TestMain(m *testing.M) {
	os.Exit(helpers.RunIsolated(&built{m}))
}

// built builds the binary once inside the isolated home, so the build
// sees the same environment the tests do.
type built struct{ *testing.M }

func (b *built) Run() int {
	m := b.M
	// Item 249: a capture here fails only on a read that has stopped
	// moving, so a loaded machine cannot fail one that is slow.
	_ = os.Setenv("BOTROPOLIS_FRAME_OPEN_CEILING", "stall")
	// make test-acceptance runs each test in a process of its own (item
	// 226), so it builds the binary once and names it here. A map run
	// builds its own, under coverage.
	if prebuilt := os.Getenv("BOTROPOLIS_ACCEPTANCE_BINARY"); prebuilt != "" && os.Getenv("BOTROPOLIS_ACCEPTANCE_COVER") == "" {
		botropolis = prebuilt
		return m.Run()
	}
	dir, err := os.MkdirTemp("", "botropolis-acceptance")
	if err != nil {
		panic(err)
	}
	botropolis = filepath.Join(dir, "botropolis")
	// The devtools tag keeps the frame tooling the city tests drive (item
	// 167); release_build_test.go builds without it.
	build := exec.Command("go", "build", "-tags", "devtools", "-o", botropolis, "../../cmd/botropolis")
	// tools/acceptance-targets maps each test to the packages it runs (item
	// 117). go test -cover points GOCOVERDIR at a directory of its own, so the
	// binary's coverage is sent to the one the tool names instead.
	if cover := os.Getenv("BOTROPOLIS_ACCEPTANCE_COVER"); cover != "" {
		build.Args = slices.Insert(build.Args, 2, "-cover", "-coverpkg=botropolis.city/botropolis/pkg/...,botropolis.city/botropolis/cmd/...")
		_ = os.Setenv("GOCOVERDIR", cover)
	}
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	return code
}

func TestStatus(t *testing.T) {
	t.Run("when a user runs status against the synthetic fixture", func(t *testing.T) {
		home := helpers.LiveFixtureHome(t, "synthetic")
		out, err := exec.Command(botropolis, "status", "--home", home).CombinedOutput()
		require.NoError(t, err, string(out))
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")

		t.Run("it should print a header", func(t *testing.T) {
			assert.True(t, strings.HasPrefix(lines[0], "STATE"), lines[0])
		})

		t.Run("it should print one row per live session", func(t *testing.T) {
			assert.Len(t, lines[1:], helpers.Manifest(t, "synthetic").LiveSessions)
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
