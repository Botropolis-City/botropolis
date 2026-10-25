package demo

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSynthHistory(t *testing.T) {
	end := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

	t.Run("when 200 sessions in 150 projects are made from a seed", func(t *testing.T) {
		history, err := SynthHistory(248, 200, 150, end)
		require.NoError(t, err)
		sessions, projects := map[string]bool{}, map[string]bool{}
		for _, p := range history {
			sessions[p.Session], projects[p.Dir] = true, true
		}

		t.Run("it should make 200 sessions", func(t *testing.T) {
			assert.Len(t, sessions, 200)
		})

		t.Run("it should use 150 projects", func(t *testing.T) {
			assert.Len(t, projects, 150)
		})

		t.Run("and made again from the same seed", func(t *testing.T) {
			again, err := SynthHistory(248, 200, 150, end)
			require.NoError(t, err)

			t.Run("it should be the same history", func(t *testing.T) {
				assert.Equal(t, history, again)
			})
		})
	})

	for _, c := range []struct {
		scenario           string
		sessions, projects int
	}{
		{scenario: "when fewer sessions than projects are asked for", sessions: 10, projects: 11},
		{scenario: "when no projects are asked for", sessions: 10, projects: 0},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			_, err := SynthHistory(248, c.sessions, c.projects, end)

			t.Run("it should refuse", func(t *testing.T) {
				assert.Error(t, err)
			})
		})
	}
}

func TestRefuseRealClaude(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "user")
	real := filepath.Join(user, ".claude")
	dotfiles := filepath.Join(dir, "dotfiles", "claude")
	linked := filepath.Join(dir, "linked")
	for _, d := range []string{real, filepath.Join(real, "projects"), dotfiles, filepath.Join(dir, "elsewhere")} {
		require.NoError(t, os.MkdirAll(d, 0o700))
	}
	require.NoError(t, os.Symlink(user, filepath.Join(dir, "to-user")))
	require.NoError(t, os.Symlink(filepath.Join(real, "projects"), filepath.Join(dir, "to-projects")))
	require.NoError(t, os.MkdirAll(linked, 0o700))
	require.NoError(t, os.Symlink(dotfiles, filepath.Join(linked, ".claude")))

	for _, c := range []struct {
		scenario string
		home     string
		real     []string
		refused  bool
	}{
		{scenario: "when --home is the user's home, whose .claude is the real one",
			home: user, real: []string{real}, refused: true},
		{scenario: "when --home is the real .claude itself",
			home: real, real: []string{real}, refused: true},
		{scenario: "when --home is a directory under the real .claude that does not exist yet",
			home: filepath.Join(real, "soak", "home"), real: []string{real}, refused: true},
		{scenario: "when --home is a link to the user's home",
			home: filepath.Join(dir, "to-user"), real: []string{real}, refused: true},
		{scenario: "when --home is under a link into the real .claude",
			home: filepath.Join(dir, "to-projects", "home"), real: []string{real}, refused: true},
		{scenario: "when the real .claude is a link, and --home is under where it points",
			home: filepath.Join(dir, "dotfiles", "claude", "home"), real: []string{filepath.Join(linked, ".claude")}, refused: true},
		{scenario: "when --home is inside the second of the real directories",
			home: filepath.Join(dotfiles, "home"), real: []string{real, dotfiles}, refused: true},
		{scenario: "when --home is elsewhere",
			home: filepath.Join(dir, "elsewhere", "home"), real: []string{real, dotfiles}, refused: false},
		{scenario: "when --home sits beside the real .claude's parent with a name it begins",
			home: filepath.Join(dir, "user-soak"), real: []string{real}, refused: false},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			err := RefuseRealClaude(c.home, c.real)

			t.Run("it should refuse it only if it is inside", func(t *testing.T) {
				assert.Equal(t, c.refused, err != nil, "%v", err)
			})
		})
	}
}

func TestRealClaudeDirs(t *testing.T) {
	home, config := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", config)
	dirs := RealClaudeDirs()

	t.Run("when HOME and CLAUDE_CONFIG_DIR are set", func(t *testing.T) {
		t.Run("it should hold $HOME/.claude", func(t *testing.T) {
			assert.Contains(t, dirs, filepath.Join(home, ".claude"))
		})

		t.Run("it should hold $CLAUDE_CONFIG_DIR", func(t *testing.T) {
			assert.Contains(t, dirs, config)
		})
	})
}
