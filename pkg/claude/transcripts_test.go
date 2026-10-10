package claude_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const otherSid = "1377da2f-4057-43fe-b3db-0293eb36bc6e"

func writeProject(t *testing.T, projects, slug string, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(projects, slug)
	for name, content := range files {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o700))
		writeFile(t, dir, name, content)
	}
	return dir
}

func TestReadTranscripts(t *testing.T) {
	t.Run("when the projects directory holds two projects with one transcript each", func(t *testing.T) {
		projects := t.TempDir()
		cinders := writeProject(t, projects, "-home-avesta-cinders", map[string]string{
			sid + ".jsonl":                            userLine + "\n",
			"sessions-index.json":                     `{"version":1,"entries":[]}`,
			".session-aliases":                        "",
			"memory/MEMORY.md":                        "",
			sid + "/subagents/agent-a1b2c3.jsonl":     userLine + "\n",
			sid + "/subagents/agent-a1b2c3.meta.json": "{}",
		})
		mullet := writeProject(t, projects, "-home-avesta-mullet", map[string]string{
			otherSid + ".jsonl": userLine + "\n",
		})

		transcripts, skipped, err := claude.ReadTranscripts(projects)
		require.NoError(t, err)

		t.Run("it should return one transcript per project-level jsonl file", func(t *testing.T) {
			assert.Len(t, transcripts, 2)
		})

		t.Run("it should return them sorted by path", func(t *testing.T) {
			assert.Equal(t,
				[]string{filepath.Join(cinders, sid+".jsonl"), filepath.Join(mullet, otherSid+".jsonl")},
				[]string{transcripts[0].Path, transcripts[1].Path})
		})

		t.Run("it should record each transcript's project directory name", func(t *testing.T) {
			assert.Equal(t,
				[]string{"-home-avesta-cinders", "-home-avesta-mullet"},
				[]string{transcripts[0].Project, transcripts[1].Project})
		})

		t.Run("it should report nothing skipped", func(t *testing.T) {
			assert.Empty(t, skipped)
		})
	})

	t.Run("when one transcript is unreadable", func(t *testing.T) {
		projects := t.TempDir()
		dir := writeProject(t, projects, "-home-avesta-cinders", map[string]string{
			sid + ".jsonl":      userLine + "\n",
			otherSid + ".jsonl": strings.ReplaceAll(userLine, sid, otherSid) + "\n",
		})
		unreadable := filepath.Join(dir, sid+".jsonl")
		require.NoError(t, os.Chmod(unreadable, 0o000))
		t.Cleanup(func() { _ = os.Chmod(unreadable, 0o600) })

		transcripts, skipped, err := claude.ReadTranscripts(projects)
		require.NoError(t, err)

		t.Run("it should report it as skipped with its path", func(t *testing.T) {
			require.Len(t, skipped, 1)
			assert.Equal(t, unreadable, skipped[0].Path)
		})

		t.Run("it should still return the readable transcript", func(t *testing.T) {
			require.Len(t, transcripts, 1)
			assert.Equal(t, otherSid, transcripts[0].SessionID)
		})
	})

	t.Run("when a project holds only a bridge stub", func(t *testing.T) {
		projects := t.TempDir()
		writeProject(t, projects, "-home-avesta-cinders", map[string]string{
			sid + ".jsonl": bridgeLine + "\n",
		})

		transcripts, _, err := claude.ReadTranscripts(projects)
		require.NoError(t, err)

		t.Run("it should return it flagged as a stub", func(t *testing.T) {
			require.Len(t, transcripts, 1)
			assert.True(t, transcripts[0].IsBridgeStub)
		})
	})

	t.Run("when the projects directory does not exist", func(t *testing.T) {
		transcripts, skipped, err := claude.ReadTranscripts(filepath.Join(t.TempDir(), "missing"))

		t.Run("it should not return an error", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should return no transcripts", func(t *testing.T) {
			assert.Empty(t, transcripts)
		})

		t.Run("it should report nothing skipped", func(t *testing.T) {
			assert.Empty(t, skipped)
		})
	})
}
