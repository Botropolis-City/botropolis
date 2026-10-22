package claude_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
)

// Item 196: a settled daemon looked for every transcript by globbing
// every project on every rescan. The index lists a directory again only
// when its modification time says something was added or removed.
func TestIndex(t *testing.T) {
	const (
		first  = "aaaaaaaa-0000-4000-8000-000000000001"
		second = "bbbbbbbb-0000-4000-8000-000000000002"
		orphan = "cccccccc-0000-4000-8000-000000000003"
	)
	hourAgo := time.Now().Add(-time.Hour)
	quiet := func(t *testing.T, dirs ...string) {
		t.Helper()
		for _, dir := range dirs {
			require.NoError(t, os.Chtimes(dir, hourAgo, hourAgo))
		}
	}
	newHome := func(t *testing.T) (projects, project string) {
		t.Helper()
		projects = t.TempDir()
		project = writeProject(t, projects, "-w-lib", map[string]string{first + ".jsonl": userLine + "\n"})
		return projects, project
	}

	t.Run("when the projects hold transcripts and subagents", func(t *testing.T) {
		projects, _ := newHome(t)
		writeProject(t, projects, "-w-app", map[string]string{
			second + ".jsonl":                        userLine + "\n",
			second + "/subagents/agent-b2.jsonl":     userLine + "\n",
			second + "/subagents/agent-a1.jsonl":     userLine + "\n",
			second + "/subagents/agent-a1.meta.json": "{}\n",
			second + "/subagents/notes.jsonl":        userLine + "\n",
			second + "/tool-results/r1.txt":          "x\n",
			"memory.md":                              "x\n",
			orphan + "/subagents/agent-c3.jsonl":     userLine + "\n",
		})
		globbed := func(pattern string) []string {
			paths, err := filepath.Glob(filepath.Join(projects, pattern))
			require.NoError(t, err)
			sort.Strings(paths)
			return paths
		}
		ix := claude.NewIndex()

		t.Run("it should list the transcripts a glob of the projects finds", func(t *testing.T) {
			assert.Equal(t, globbed("*/*.jsonl"), ix.Transcripts(projects))
		})

		t.Run("it should list every subagent transcript a glob of the projects finds", func(t *testing.T) {
			assert.Equal(t, globbed("*/*/subagents/agent-*.jsonl"), ix.SubagentTranscripts(projects))
		})

		t.Run("it should list a session's subagents as SubagentPaths does", func(t *testing.T) {
			path := filepath.Join(projects, "-w-app", second+".jsonl")
			want, err := claude.SubagentPaths(path)
			require.NoError(t, err)
			assert.Equal(t, want, ix.Subagents(path))
		})

		t.Run("it should find a transcript by its session", func(t *testing.T) {
			path, ok := ix.Find(projects, second)
			assert.Equal(t, [2]any{filepath.Join(projects, "-w-app", second+".jsonl"), true}, [2]any{path, ok})
		})
	})

	t.Run("when a session's transcript is in two projects", func(t *testing.T) {
		projects, _ := newHome(t)
		writeProject(t, projects, "-w-aaa", map[string]string{first + ".jsonl": userLine + "\n"})
		want, _ := claude.FindTranscript(projects, first)

		t.Run("it should find the one FindTranscript finds", func(t *testing.T) {
			path, _ := claude.NewIndex().Find(projects, first)
			assert.Equal(t, want, path)
		})
	})

	t.Run("when no directory changed since the last listing", func(t *testing.T) {
		projects, project := newHome(t)
		quiet(t, projects, project)
		ix := claude.NewIndex()
		ix.Transcripts(projects)
		before := ix.Listings()
		ix.Transcripts(projects)

		t.Run("it should list nothing again", func(t *testing.T) {
			assert.Equal(t, before, ix.Listings())
		})
	})

	t.Run("when one load asks for the subagents of every session in a project", func(t *testing.T) {
		projects, project := newHome(t)
		writeFile(t, project, second+".jsonl", userLine+"\n")
		ix := claude.NewIndex()
		ix.Refresh()
		paths := ix.Transcripts(projects)
		before := ix.Stats()
		for _, path := range paths {
			ix.Subagents(path)
		}

		t.Run("it should look at the project's directory once", func(t *testing.T) {
			assert.Equal(t, before, ix.Stats())
		})
	})

	t.Run("when a transcript is added after the last listing", func(t *testing.T) {
		projects, project := newHome(t)
		quiet(t, projects, project)
		ix := claude.NewIndex()
		ix.Transcripts(projects)
		writeFile(t, project, second+".jsonl", userLine+"\n")

		t.Run("it should find it", func(t *testing.T) {
			_, ok := ix.Find(projects, second)
			assert.True(t, ok)
		})
	})

	t.Run("when a transcript is added within the same modification-time tick as the last listing", func(t *testing.T) {
		projects, project := newHome(t)
		info, err := os.Stat(project)
		require.NoError(t, err)
		ix := claude.NewIndex()
		ix.Transcripts(projects)
		writeFile(t, project, second+".jsonl", userLine+"\n")
		require.NoError(t, os.Chtimes(project, info.ModTime(), info.ModTime()))

		t.Run("it should still find it", func(t *testing.T) {
			_, ok := ix.Find(projects, second)
			assert.True(t, ok)
		})
	})

	t.Run("when a transcript is removed", func(t *testing.T) {
		projects, project := newHome(t)
		quiet(t, projects, project)
		ix := claude.NewIndex()
		ix.Transcripts(projects)
		require.NoError(t, os.Remove(filepath.Join(project, first+".jsonl")))

		t.Run("it should no longer find it", func(t *testing.T) {
			_, ok := ix.Find(projects, first)
			assert.False(t, ok)
		})
	})
}
