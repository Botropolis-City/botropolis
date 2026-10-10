package demo

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

func TestOverlay(t *testing.T) {
	now := time.Date(2026, 10, 7, 15, 0, 0, 0, time.UTC)
	corpus := newCorpus(t)
	read := func(t *testing.T, home, project, id string) claude.Transcript {
		t.Helper()
		tr, err := claude.ReadTranscript(filepath.Join(home, ".claude", "projects", claude.ProjectFolder(Root+"/"+project), id+".jsonl"))
		require.NoError(t, err)
		return tr
	}

	t.Run("when a session is given PRs, errors, a team and a title", func(t *testing.T) {
		got := stage(t, corpus, Scenario{Sessions: []Placement{{
			Ref: "tidepool/6f1d", State: state.Working, Ago: Duration(time.Minute),
			PRs: []string{"open", "merged", "closed"}, Errors: 2, Team: "harbor", Agent: "lead", Title: "Leap-day fix",
		}}}, now)
		require.NoError(t, got.err)
		tr := read(t, got.home, "tidepool", recordedID)

		t.Run("it should raise a flag per PR in its state", func(t *testing.T) {
			var states []string
			for _, pr := range tr.PRs {
				states = append(states, pr.State)
			}
			assert.Equal(t, []string{claude.PROpen, claude.PRMerged, claude.PRClosed}, states)
		})

		t.Run("it should file the PRs under the session's own project", func(t *testing.T) {
			assert.Equal(t, "botropolis-demo/tidepool", tr.PRs[0].Repository)
		})

		t.Run("it should count the errors", func(t *testing.T) {
			assert.Equal(t, 2, tr.APIErrors)
		})

		t.Run("it should put the session in the team", func(t *testing.T) {
			assert.Equal(t, [2]string{"harbor", "lead"}, [2]string{tr.TeamName, tr.AgentName})
		})

		t.Run("it should take the title", func(t *testing.T) {
			assert.Equal(t, "Leap-day fix", tr.Title)
		})

		t.Run("it should still end where the scenario says", func(t *testing.T) {
			assert.Equal(t, now.Add(-time.Minute), tr.LastAt)
		})
	})

	t.Run("when a recorded session is cloned into another project", func(t *testing.T) {
		scenario := Scenario{Name: "city", Sessions: []Placement{
			{Ref: "tidepool/6f1d", State: state.Working},
			{Ref: "tidepool/6f1d", Project: "driftwood", State: state.Working},
		}}
		got := stage(t, corpus, scenario, now)
		require.NoError(t, got.err)
		clone := plotID("city", 1)
		tr := read(t, got.home, "driftwood", clone)

		t.Run("it should work in the new project", func(t *testing.T) {
			assert.Equal(t, Root+"/driftwood", tr.CWD)
		})

		t.Run("it should be a session of its own", func(t *testing.T) {
			assert.Equal(t, clone, tr.SessionID)
		})

		t.Run("it should bring its subagents along under its own id", func(t *testing.T) {
			assert.FileExists(t, filepath.Join(got.home, ".claude", "projects", claude.ProjectFolder(Root+"/driftwood"), clone, "subagents", "agent-a1.jsonl"))
		})

		t.Run("it should move its subagents into the new project too", func(t *testing.T) {
			lines := readLines(t, filepath.Join(got.home, ".claude", "projects", claude.ProjectFolder(Root+"/driftwood"), clone, "subagents", "agent-a1.jsonl"))
			assert.Equal(t, Root+"/driftwood", firstCWD(lines))
		})

		t.Run("it should leave the original where it was", func(t *testing.T) {
			assert.Equal(t, Root+"/tidepool", read(t, got.home, "tidepool", recordedID).CWD)
		})
	})

	t.Run("when a PR state is not one the city draws", func(t *testing.T) {
		got := stage(t, corpus, Scenario{Sessions: []Placement{{Ref: "tidepool/6f1d", State: state.Working, PRs: []string{"draft"}}}}, now)

		t.Run("it should refuse", func(t *testing.T) {
			assert.Error(t, got.err)
		})
	})
}

func TestContextOverlay(t *testing.T) {
	now := time.Date(2026, 10, 7, 15, 0, 0, 0, time.UTC)
	corpus := t.TempDir()
	usage := func(read int) string {
		return `{"type":"assistant","sessionId":"` + recordedID + `","cwd":"` + cwd + `","timestamp":"2026-09-01T10:10:00.000Z",` +
			`"message":{"id":"m2","model":"claude-sonnet-4-5","role":"assistant","content":[{"type":"text","text":"done"}],` +
			`"usage":{"input_tokens":10,"output_tokens":50,"cache_read_input_tokens":` + strconv.Itoa(read) + `,"cache_creation_input_tokens":90}}}`
	}
	writeFile(t, filepath.Join(corpus, "tidepool", recordedID+".jsonl"),
		line("user", "2026-09-01T10:00:00.000Z")+"\n"+usage(20000)+"\n"+line("user", "2026-09-01T10:10:30.000Z")+"\n"+
			strings.Replace(usage(5000), `"type":"assistant",`, `"type":"assistant","isSidechain":true,`, 1)+"\n")
	read := func(t *testing.T, home string) claude.Transcript {
		t.Helper()
		tr, err := claude.ReadTranscript(filepath.Join(home, ".claude", "projects", claude.ProjectFolder(cwd), recordedID+".jsonl"))
		require.NoError(t, err)
		return tr
	}

	t.Run("when a session is placed at 72% context", func(t *testing.T) {
		got := stage(t, Corpus{Dir: corpus}, Scenario{Sessions: []Placement{{Ref: "tidepool/6f1d", State: state.Working, Context: "72%"}}}, now)
		require.NoError(t, got.err)
		tr := read(t, got.home)

		t.Run("it should hold 72% of a 200k window", func(t *testing.T) {
			assert.EqualValues(t, 144000, tr.ContextTokens)
		})

		t.Run("it should leave whose turn it is alone", func(t *testing.T) {
			untouched := stage(t, Corpus{Dir: corpus}, Scenario{Sessions: []Placement{{Ref: "tidepool/6f1d", State: state.Working}}}, now)
			assert.Equal(t, read(t, untouched.home).Tail.Turn, tr.Tail.Turn)
		})
	})

	for _, raw := range []string{"0%", "100%", "lots", "72"} {
		t.Run("when the context is "+raw, func(t *testing.T) {
			got := stage(t, Corpus{Dir: corpus}, Scenario{Sessions: []Placement{{Ref: "tidepool/6f1d", State: state.Working, Context: raw}}}, now)

			t.Run("it should refuse", func(t *testing.T) {
				assert.Error(t, got.err)
			})
		})
	}
}
