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
