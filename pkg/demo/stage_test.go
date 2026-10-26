package demo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

const (
	recordedID = "6f1d2c3b-0000-4000-8000-000000000001"
	cwd        = Root + "/tidepool"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func line(kind, at string) string {
	return `{"type":"` + kind + `","sessionId":"` + recordedID + `","cwd":"` + cwd + `","timestamp":"` + at + `"}`
}

// newCorpus is one recorded tidepool session that ended at 10:10 and
// had a subagent that finished at 10:05.
func newCorpus(t *testing.T) Corpus {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tidepool", recordedID+".jsonl"),
		line("user", "2026-09-01T10:00:00.000Z")+"\n"+line("assistant", "2026-09-01T10:10:00.000Z")+"\n")
	writeFile(t, filepath.Join(dir, "tidepool", recordedID, "subagents", "agent-a1.jsonl"),
		line("assistant", "2026-09-01T10:05:00.000Z")+"\n")
	return Corpus{Dir: dir}
}

type staged struct {
	home string
	pids []int
	err  error
}

func stage(t *testing.T, corpus Corpus, scenario Scenario, now time.Time) staged {
	t.Helper()
	next := 4000
	var pids []int
	spawn := func() (int, error) {
		next++
		pids = append(pids, next)
		return next, nil
	}
	home := t.TempDir()
	err := Stage(corpus, scenario, home, now, spawn)
	return staged{home: home, pids: pids, err: err}
}

func records(t *testing.T, home string) []map[string]any {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(home, ".claude", "sessions", "*.json"))
	require.NoError(t, err)
	var out []map[string]any
	for _, p := range paths {
		data, err := os.ReadFile(p)
		require.NoError(t, err)
		var r map[string]any
		require.NoError(t, json.Unmarshal(data, &r))
		out = append(out, r)
	}
	return out
}

func readLines(t *testing.T, path string) [][]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var out [][]byte
	for _, l := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		out = append(out, []byte(l))
	}
	return out
}

func TestStage(t *testing.T) {
	now := time.Date(2026, 10, 7, 15, 0, 0, 0, time.UTC)
	corpus := newCorpus(t)
	transcript := func(home string) string {
		return filepath.Join(home, ".claude", "projects", claude.ProjectFolder(cwd), recordedID+".jsonl")
	}

	t.Run("when a recorded session is placed as working, two minutes ago", func(t *testing.T) {
		got := stage(t, corpus, Scenario{Sessions: []Placement{{Ref: "tidepool/6f1d", State: state.Working, Ago: Duration(2 * time.Minute)}}}, now)
		require.NoError(t, got.err)

		t.Run("it should end the transcript two minutes before now", func(t *testing.T) {
			latest, _ := Latest(readLines(t, transcript(got.home)))
			assert.Equal(t, now.Add(-2*time.Minute), latest)
		})

		t.Run("it should move the subagent by the same delta", func(t *testing.T) {
			path := filepath.Join(filepath.Dir(transcript(got.home)), recordedID, "subagents", "agent-a1.jsonl")
			latest, _ := Latest(readLines(t, path))
			assert.Equal(t, now.Add(-7*time.Minute), latest)
		})

		t.Run("it should point the record at a stand-in process", func(t *testing.T) {
			assert.EqualValues(t, got.pids[0], records(t, got.home)[0]["pid"])
		})

		t.Run("it should start the record when the transcript starts", func(t *testing.T) {
			assert.EqualValues(t, now.Add(-12*time.Minute).UnixMilli(), records(t, got.home)[0]["startedAt"])
		})
	})

	for _, tc := range []struct {
		state  state.State
		kind   claude.Kind
		status claude.Status
	}{
		{state.Working, claude.KindInteractive, claude.StatusBusy},
		{state.NeedsYou, claude.KindInteractive, claude.StatusIdle},
		{state.Unattended, claude.KindBackground, claude.StatusBusy},
	} {
		t.Run("when a recorded session is placed as "+string(tc.state), func(t *testing.T) {
			got := stage(t, corpus, Scenario{Sessions: []Placement{{Ref: "tidepool/6f1d", State: tc.state}}}, now)
			require.NoError(t, got.err)
			r := records(t, got.home)[0]

			t.Run("it should write a record the loader derives that state from", func(t *testing.T) {
				assert.Equal(t, []any{string(tc.kind), string(tc.status)}, []any{r["kind"], r["status"]})
			})
		})
	}

	t.Run("when a recorded session is placed as parked", func(t *testing.T) {
		got := stage(t, corpus, Scenario{Sessions: []Placement{{Ref: "tidepool/6f1d", State: state.Resting}}}, now)
		require.NoError(t, got.err)

		t.Run("it should write no record", func(t *testing.T) {
			assert.Empty(t, records(t, got.home))
		})

		t.Run("it should still write the transcript", func(t *testing.T) {
			assert.FileExists(t, transcript(got.home))
		})
	})

	t.Run("when a placement says started 3h", func(t *testing.T) {
		t.Run("and its recording spans ten minutes", func(t *testing.T) {
			got := stage(t, corpus, Scenario{Sessions: []Placement{{Ref: "tidepool/6f1d", State: state.Working, Ago: Duration(2 * time.Minute), Started: Duration(3 * time.Hour)}}}, now)
			require.NoError(t, got.err)

			t.Run("it should begin the staged transcript 3h before now", func(t *testing.T) {
				assert.Equal(t, now.Add(-3*time.Hour), earliest(readLines(t, transcript(got.home))))
			})

			t.Run("it should keep the newest line at ago", func(t *testing.T) {
				latest, _ := Latest(readLines(t, transcript(got.home)))
				assert.Equal(t, now.Add(-2*time.Minute), latest)
			})

			t.Run("it should record startedAt 3h before now", func(t *testing.T) {
				assert.EqualValues(t, now.Add(-3*time.Hour).UnixMilli(), records(t, got.home)[0]["startedAt"])
			})
		})

		t.Run("and it is an empty plot", func(t *testing.T) {
			got := stage(t, corpus, Scenario{Sessions: []Placement{{Project: "lanternfish", State: state.Empty, Started: Duration(3 * time.Hour)}}}, now)
			require.NoError(t, got.err)

			t.Run("it should record startedAt 3h before now", func(t *testing.T) {
				assert.EqualValues(t, now.Add(-3*time.Hour).UnixMilli(), records(t, got.home)[0]["startedAt"])
			})
		})
	})

	for name, p := range map[string]Placement{
		"a recording to start sooner than ago plus its span": {Ref: "tidepool/6f1d", State: state.Working, Ago: Duration(2 * time.Minute), Started: Duration(5 * time.Minute)},
		"an empty plot to start sooner than ago":             {Project: "lanternfish", State: state.Empty, Ago: Duration(time.Hour), Started: Duration(time.Minute)},
	} {
		t.Run("when a placement asks "+name, func(t *testing.T) {
			t.Run("it should refuse the placement", func(t *testing.T) {
				assert.Error(t, stage(t, corpus, Scenario{Sessions: []Placement{p}}, now).err)
			})
		})
	}

	t.Run("when a placement gives a recorded session a branch", func(t *testing.T) {
		got := stage(t, corpus, Scenario{Sessions: []Placement{{Ref: "tidepool/6f1d", State: state.Working, Ago: Duration(2 * time.Minute), Branch: "feat/tide-units"}}}, now)
		require.NoError(t, got.err)

		t.Run("it should put the session on that branch", func(t *testing.T) {
			staged, err := claude.ReadTranscript(transcript(got.home))
			require.NoError(t, err)
			assert.Equal(t, "feat/tide-units", staged.Branch)
		})
	})

	t.Run("when a plot is placed empty in a project", func(t *testing.T) {
		got := stage(t, corpus, Scenario{Sessions: []Placement{{Project: "lanternfish", State: state.Empty}}}, now)
		require.NoError(t, got.err)

		t.Run("it should write an idle record in that project", func(t *testing.T) {
			r := records(t, got.home)[0]
			assert.Equal(t, []any{Root + "/lanternfish", "idle"}, []any{r["cwd"], r["status"]})
		})
	})

	t.Run("when an empty plot is given a title", func(t *testing.T) {
		got := stage(t, corpus, Scenario{Sessions: []Placement{{Project: "lanternfish", State: state.Empty, Title: "Night market map"}}}, now)
		require.NoError(t, got.err)

		t.Run("it should name the record, the only title a plot with no transcript has", func(t *testing.T) {
			assert.Equal(t, "Night market map", records(t, got.home)[0]["name"])
		})
	})

	t.Run("when the scenario names MCP servers", func(t *testing.T) {
		got := stage(t, corpus, Scenario{MCP: []string{"tracker", "weather"}}, now)
		require.NoError(t, got.err)
		config, err := claude.ReadMCPConfig(filepath.Join(got.home, ".claude.json"))
		require.NoError(t, err)

		t.Run("it should configure each of them", func(t *testing.T) {
			var names []string
			for _, s := range config.Global {
				names = append(names, s.Name)
			}
			assert.ElementsMatch(t, []string{"tracker", "weather"}, names)
		})
	})

	for name, ref := range map[string]string{
		"names no corpus session": "tidepool/ffff",
		"names no corpus project": "nowhere/6f1d",
		"is not project/session":  "6f1d",
	} {
		t.Run("when a ref "+name, func(t *testing.T) {
			got := stage(t, corpus, Scenario{Sessions: []Placement{{Ref: ref, State: state.Working}}}, now)

			t.Run("it should refuse to stage", func(t *testing.T) {
				assert.Error(t, got.err)
			})
		})
	}
}
