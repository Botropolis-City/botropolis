package cli_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/state"
)

var completionCity = state.Snapshot{Sessions: []state.Session{
	{ID: "0898d7e4-f2cb-45fe-833e-1921c1f4bcaa", Title: "fix the CI queue", Kind: claude.KindBackground, Alive: true, State: state.Working},
	{ID: "16daefa1-0000-0000-0000-000000000000", Title: "the parked one", Kind: claude.KindBackground, State: state.Resting},
	{ID: "aaaaaaaa-0000-0000-0000-000000000000", Title: "in a terminal", Kind: claude.KindInteractive, Alive: true, State: state.Working},
}}

// complete runs what a shell's completion script runs, and returns the
// candidates and the directive line.
func complete(t *testing.T, snapshot state.Snapshot, args ...string) ([]string, string) {
	t.Helper()
	h := newHarness(&config.Config{})
	h.status.snapshot = snapshot
	require.NoError(t, h.run(append([]string{"__complete"}, args...)...))
	lines := strings.Split(strings.TrimSpace(h.out.String()), "\n")
	return lines[:len(lines)-1], lines[len(lines)-1]
}

func TestCompletion(t *testing.T) {
	cases := []struct {
		scenario string
		args     []string
		want     []string
	}{
		{"when attach is completed", []string{"attach", ""}, []string{"0898d7e4\tfix the CI queue"}},
		{"when stop is completed", []string{"stop", ""}, []string{"0898d7e4\tfix the CI queue"}},
		{"when rm is completed", []string{"rm", ""}, []string{"0898d7e4\tfix the CI queue", "16daefa1\tthe parked one"}},
		{"when resume is completed", []string{"resume", ""}, []string{"16daefa1-0000-0000-0000-000000000000\tthe parked one"}},
		{"when attach already has its id", []string{"attach", "0898d7e4", ""}, nil},
	}
	for _, c := range cases {
		t.Run(c.scenario, func(t *testing.T) {
			got, directive := complete(t, completionCity, c.args...)

			t.Run("it should offer the sessions it can act on, titled", func(t *testing.T) {
				assert.Equal(t, c.want, nilIfEmpty(got))
			})

			t.Run("it should not fall back to file names", func(t *testing.T) {
				assert.Equal(t, ":4", directive)
			})
		})
	}

	t.Run("when new is completed", func(t *testing.T) {
		_, directive := complete(t, completionCity, "new", "")

		t.Run("it should offer directories only", func(t *testing.T) {
			assert.Equal(t, ":16", directive)
		})
	})

	t.Run("when new already has its directory", func(t *testing.T) {
		_, directive := complete(t, completionCity, "new", "/tmp", "")

		t.Run("it should not offer anything for the prompt", func(t *testing.T) {
			assert.Equal(t, ":4", directive)
		})
	})
}

func nilIfEmpty(lines []string) []string {
	var out []string
	for _, l := range lines {
		if l != "" && !strings.HasPrefix(l, "Completion ended") {
			out = append(out, l)
		}
	}
	return out
}
