package demo

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Item 273: the corpus was recorded by claude -p, so every recording
// says sdk-cli; a staged session is someone's unless its placement says
// a program ran it.
func TestStagedEntrypoint(t *testing.T) {
	now := time.Date(2026, 10, 7, 15, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	scripted := func(kind, at string) string {
		return strings.Replace(line(kind, at), `"type"`, `"entrypoint":"sdk-cli","type"`, 1)
	}
	writeFile(t, filepath.Join(dir, "tidepool", recordedID+".jsonl"),
		scripted("user", "2026-09-01T10:00:00.000Z")+"\n"+scripted("assistant", "2026-09-01T10:10:00.000Z")+"\n")
	corpus := Corpus{Dir: dir}

	t.Run("when a placement keeps two subagents at work", func(t *testing.T) {
		got := stage(t, corpus, Scenario{Sessions: []Placement{{
			Ref: "tidepool/6f1d", State: state.Working, Ago: Duration(time.Minute), Subagents: 2,
		}}}, now)
		require.NoError(t, got.err)
		path := filepath.Join(got.home, ".claude", "projects", claude.ProjectFolder(Root+"/tidepool"), recordedID+".jsonl")
		tr, err := claude.ReadTranscript(path)
		require.NoError(t, err)
		subs, _, err := claude.ReadSubagents(path)
		require.NoError(t, err)
		var linked []string
		for _, s := range subs {
			linked = append(linked, s.ToolUseID)
		}

		t.Run("it should leave two agent calls waiting at the transcript's end", func(t *testing.T) {
			assert.Len(t, tr.Tail.PendingToolIDs, 2)
		})

		t.Run("it should give each waiting call a subagent of its own", func(t *testing.T) {
			assert.ElementsMatch(t, tr.Tail.PendingToolIDs, linked)
		})
	})

	for _, c := range []struct {
		when     string
		scripted bool
		want     string
	}{
		{"when a placement says nothing of how it was started", false, "cli"},
		{"when a placement says a program ran it", true, "sdk-cli"},
	} {
		t.Run(c.when, func(t *testing.T) {
			got := stage(t, corpus, Scenario{Sessions: []Placement{{
				Ref: "tidepool/6f1d", State: state.Resting, Ago: Duration(time.Hour), Scripted: c.scripted,
			}}}, now)
			require.NoError(t, got.err)
			tr, err := claude.ReadTranscript(filepath.Join(got.home, ".claude", "projects", claude.ProjectFolder(Root+"/tidepool"), recordedID+".jsonl"))
			require.NoError(t, err)

			t.Run("it should stage the entrypoint "+c.want, func(t *testing.T) {
				assert.Equal(t, c.want, tr.Entrypoint)
			})
		})
	}
}
