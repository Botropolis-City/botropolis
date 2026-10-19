package demo_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
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

		// Unstage has signalled every stand-in; what is left is each one's
		// reaper collecting it, and a zombie still answers kill(pid, 0).
		// That is scheduler latency, so the deadline is one a working
		// suite cannot reach, however loaded (item 124).
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
			}, 30*time.Second, 50*time.Millisecond)
		})
	})
}

// Item 115. A replayed session's lines are restamped as they are
// revealed, so the session looks new for the whole replay; the city
// builds once, from the session first appearing, and finishes.
func TestReplayedSessionIsBuiltOnce(t *testing.T) {
	start := time.Now()
	corpus := t.TempDir()
	const replayed, resting = "a1000000-0000-4000-8000-000000000001", "d4000000-0000-4000-8000-000000000004"
	recorded(t, corpus, "tidepool", replayed)
	recorded(t, corpus, "orchard", resting)
	arrive := time.Second
	scenario := demo.Scenario{Name: "test", Sessions: []demo.Placement{
		{Ref: "orchard/" + resting[:4], State: state.Resting, Ago: demo.Duration(time.Hour)},
		{Ref: "tidepool/" + replayed[:4], State: state.Working, Arrive: demo.Duration(arrive), Replay: &demo.Replay{Over: demo.Duration(14 * time.Second)}},
	}}
	home := t.TempDir()
	director, err := demo.NewDirector(demo.Corpus{Dir: corpus}, scenario, home, start, demo.Sleepers(home, time.Minute))
	require.NoError(t, err)
	t.Cleanup(func() { _ = demo.Unstage(home) })

	clock := start
	scene := city.NewScene(city.NewLayout())
	scene.SetClock(func() time.Time { return clock }, time.UTC)
	events := director.Events()
	built := arrive + city.SailFor + city.BuildFor
	var id string
	seenBuilding, buildingAfter := false, true
	for at := time.Duration(0); at <= built+time.Second; at += 250 * time.Millisecond {
		clock = start.Add(at)
		for len(events) > 0 && events[0].At <= at {
			require.NoError(t, director.Apply(events[0], clock))
			events = events[1:]
		}
		snapshot, err := state.Load(home, probes, clock)
		require.NoError(t, err)
		scene.SetSnapshot(snapshot)
		for _, s := range snapshot.Sessions {
			if s.State == state.Working {
				id = s.ID
			}
		}
		if id == "" {
			continue
		}
		_, building := scene.Construction(id)
		seenBuilding = seenBuilding || building
		buildingAfter = building
	}
	require.NotEmpty(t, events, "the replay should still be revealing lines when the build is due to finish")

	t.Run("when a staged session arrives and replays for longer than a build", func(t *testing.T) {
		t.Run("it should be seen going up", func(t *testing.T) {
			assert.True(t, seenBuilding)
		})

		t.Run("and the city has watched it past SailFor and BuildFor", func(t *testing.T) {
			t.Run("it should show its building finished, not under construction", func(t *testing.T) {
				assert.False(t, buildingAfter)
			})
		})
	})
}
