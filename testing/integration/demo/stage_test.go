package demo_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/demo"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

var probes = state.Probes{Alive: state.ProcessAlive, Attached: state.UnixSocketConnected}

// recorded writes a two-line conversation into the corpus as if the
// recorder had captured it in /home/demo/src/<project>.
func recorded(t *testing.T, corpus, project, id string) {
	t.Helper()
	cwd := demo.Root + "/" + project
	lines := []string{
		helpers.UserPrompt(id, cwd, "2026-09-01T10:00:00.000Z"),
		helpers.AssistantReply(id, "msg_"+id[:8], "2026-09-01T10:00:20.000Z"),
	}
	path := filepath.Join(corpus, project, id+".jsonl")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
}

func TestStagedHomeLoadsAsTheScenarioSays(t *testing.T) {
	now := time.Now()
	corpus := t.TempDir()
	ids := map[state.State]string{
		state.Working:    "a1000000-0000-4000-8000-000000000001",
		state.NeedsYou:   "b2000000-0000-4000-8000-000000000002",
		state.Unattended: "c3000000-0000-4000-8000-000000000003",
		state.Resting:    "d4000000-0000-4000-8000-000000000004",
	}
	projects := map[state.State]string{
		state.Working: "tidepool", state.NeedsYou: "lanternfish", state.Unattended: "kiln", state.Resting: "orchard",
	}
	scenario := demo.Scenario{Name: "test", MCP: []string{"tracker"}}
	for s, id := range ids {
		recorded(t, corpus, projects[s], id)
		scenario.Sessions = append(scenario.Sessions, demo.Placement{Ref: projects[s] + "/" + id[:4], State: s, Ago: demo.Duration(time.Hour)})
	}
	scenario.Sessions = append(scenario.Sessions, demo.Placement{Project: "beacon", State: state.Empty})

	home := t.TempDir()
	require.NoError(t, demo.Stage(demo.Corpus{Dir: corpus}, scenario, home, now, demo.Sleepers(home, time.Minute)))
	t.Cleanup(func() { _ = demo.Unstage(home) })
	snapshot, err := state.Load(home, probes, now)
	require.NoError(t, err)
	got := map[string]state.State{}
	for _, s := range snapshot.Sessions {
		got[s.ID] = s.State
	}

	t.Run("when a scenario is staged and loaded", func(t *testing.T) {
		for want, id := range ids {
			t.Run("it should show the "+projects[want]+" session as "+string(want), func(t *testing.T) {
				assert.Equal(t, want, got[id])
			})
		}

		t.Run("it should show the empty plot as empty", func(t *testing.T) {
			var empties int
			for _, s := range got {
				if s == state.Empty {
					empties++
				}
			}
			assert.Equal(t, 1, empties)
		})

		t.Run("it should raise a tower for the configured MCP server", func(t *testing.T) {
			var names []string
			for _, s := range snapshot.Servers {
				names = append(names, s.Name)
			}
			assert.Equal(t, []string{"tracker"}, names)
		})
	})

	t.Run("when the home is unstaged", func(t *testing.T) {
		require.NoError(t, demo.Unstage(home))

		t.Run("it should leave no session alive", func(t *testing.T) {
			assert.Eventually(t, func() bool {
				after, err := state.Load(home, probes, now)
				if err != nil {
					return false
				}
				for _, s := range after.Sessions {
					if s.State != state.Resting {
						return false
					}
				}
				return true
			}, 5*time.Second, 50*time.Millisecond)
		})
	})
}
