package demo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
)

func TestSessionID(t *testing.T) {
	t.Run("when the same label is asked for twice", func(t *testing.T) {
		t.Run("it should get the same id, so a re-run finds what it recorded", func(t *testing.T) {
			assert.Equal(t, SessionID("tidepool", "fix-leap-day"), SessionID("tidepool", "fix-leap-day"))
		})
	})

	t.Run("when two projects use one label", func(t *testing.T) {
		t.Run("it should give them different ids", func(t *testing.T) {
			assert.NotEqual(t, SessionID("tidepool", "quick"), SessionID("kiln", "quick"))
		})
	})

	t.Run("when an id is made", func(t *testing.T) {
		t.Run("it should be a version 4 shaped UUID, which --session-id requires", func(t *testing.T) {
			assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, SessionID("a", "b"))
		})
	})
}

func TestCorpusFindsByLabel(t *testing.T) {
	dir := t.TempDir()
	id := SessionID("tidepool", "fix-leap-day")
	writeFile(t, filepath.Join(dir, "tidepool", id+".jsonl"), "{}\n")

	t.Run("when a ref names a script label rather than an id", func(t *testing.T) {
		got, err := Corpus{Dir: dir}.Find("tidepool/fix-leap-day")
		require.NoError(t, err)

		t.Run("it should find the session recorded for it", func(t *testing.T) {
			assert.Equal(t, id, got.ID)
		})
	})
}

func TestClaudeArgs(t *testing.T) {
	s := ScriptedSession{Label: "l", Model: "sonnet", BudgetUSD: 0.4}

	t.Run("when the first prompt is sent", func(t *testing.T) {
		t.Run("it should start the session under its own id", func(t *testing.T) {
			assert.Equal(t, []string{"-p", "go", "--model", "sonnet", "--max-budget-usd", "0.4",
				"--dangerously-skip-permissions", "--output-format", "json", "--mcp-config", "/m.json",
				"--session-id", "ID"}, claudeArgs(s, "ID", 0, "go", "/m.json"))
		})
	})

	t.Run("when a follow-up is sent", func(t *testing.T) {
		t.Run("it should resume that session", func(t *testing.T) {
			assert.Equal(t, []string{"--resume", "ID"}, claudeArgs(s, "ID", 1, "more", "/m.json")[11:])
		})
	})
}

func TestRecord(t *testing.T) {
	ctx := context.Background()
	home, corpus := t.TempDir(), t.TempDir()
	script := Script{Project: "tidepool", Sessions: []ScriptedSession{
		{Label: "two-turns", Model: "haiku", BudgetUSD: 0.1, Prompts: []string{"one", "two"}},
	}}
	id := SessionID("tidepool", "two-turns")
	var calls [][]string
	claude := func(_ context.Context, dir string, args []string) ([]byte, error) {
		calls = append(calls, args)
		folder := filepath.Join(home, ".claude", "projects", "-home-demo-src-tidepool")
		writeFile(t, filepath.Join(folder, id+".jsonl"), `{"type":"user"}`+"\n")
		writeFile(t, filepath.Join(folder, id, "subagents", "agent-x.jsonl"), `{"type":"assistant"}`+"\n")
		return []byte(`{"session_id":"` + id + `","total_cost_usd":0.01}`), nil
	}
	r := Recorder{Home: home, Src: Root, Corpus: corpus, MCPConfig: "/m.json", Claude: claude}

	t.Run("when a two-prompt session is recorded", func(t *testing.T) {
		_, err := r.Record(ctx, script)
		require.NoError(t, err)

		t.Run("it should send both prompts", func(t *testing.T) {
			assert.Len(t, calls, 2)
		})

		t.Run("it should export the transcript into the corpus", func(t *testing.T) {
			assert.FileExists(t, filepath.Join(corpus, "tidepool", id+".jsonl"))
		})

		t.Run("it should export the subagents with it", func(t *testing.T) {
			assert.FileExists(t, filepath.Join(corpus, "tidepool", id, "subagents", "agent-x.jsonl"))
		})
	})

	t.Run("when the corpus already holds the session", func(t *testing.T) {
		calls = nil
		_, err := r.Record(ctx, script)
		require.NoError(t, err)

		t.Run("it should not spend anything recording it again", func(t *testing.T) {
			assert.Empty(t, calls)
		})
	})

	t.Run("when the home holds anything besides transcripts", func(t *testing.T) {
		writeFile(t, filepath.Join(home, ".claude.json"), `{"oauthAccount":{"emailAddress":"someone"}}`)
		require.NoError(t, os.RemoveAll(filepath.Join(corpus, "tidepool")))
		_, err := r.Record(ctx, script)
		require.NoError(t, err)
		var exported []string
		require.NoError(t, filepath.WalkDir(corpus, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				exported = append(exported, filepath.Base(p))
			}
			return err
		}))

		t.Run("it should export only the session's transcripts", func(t *testing.T) {
			assert.ElementsMatch(t, []string{id + ".jsonl", "agent-x.jsonl"}, exported)
		})
	})
}

// The scrub keeps only the lines the app reads, so whatever a new Claude
// Code version starts writing is dropped by default rather than
// published (item 105).
func TestScrub(t *testing.T) {
	dropped := map[string]string{
		"an API request":             `{"type":"api-request","params":{"betas":["x"]}}`,
		"an API request's blob":      `{"type":"api-request-blob","message":{"role":"system","content":["You are..."]}}`,
		"an API request's shape":     `{"type":"api-request-shape","shape":{"system":["You are..."],"tools":[]}}`,
		"a type made up tomorrow":    `{"type":"something-new","secret":"x"}`,
		"a queued operation":         `{"type":"queue-operation","operation":"enqueue"}`,
		"a prompt snapshot":          `{"type":"attachment","attachment":{"type":"prompt_snapshot","systemPrompt":["You are..."]}}`,
		"the host's environment":     `{"type":"attachment","attachment":{"type":"environment","snapshot":{"osVersion":"Linux 7.2.8-arch1-1"}}}`,
		"Claude Code's skill list":   `{"type":"attachment","attachment":{"type":"skill_listing","skills":["x"]}}`,
		"a line with no type at all": `{"sessionId":"x"}`,
	}
	for name, line := range dropped {
		t.Run("when a transcript holds "+name+", which the app does not read", func(t *testing.T) {
			out := scrub([][]byte{[]byte(line)})

			t.Run("it should drop it", func(t *testing.T) {
				assert.Empty(t, out)
			})
		})
	}

	for _, kind := range keptTypes() {
		line := `{"type":"` + kind + `","sessionId":"x","timestamp":"2026-10-25T10:00:00.000Z"}`
		if kind == "attachment" {
			line = `{"type":"attachment","attachment":{"type":"model","identity":{"modelId":"claude-haiku-5-5"}}}`
		}
		t.Run("when a transcript holds a "+kind+" line, which the app reads", func(t *testing.T) {
			out := scrub([][]byte{[]byte(line)})

			t.Run("it should keep it as it was", func(t *testing.T) {
				assert.Equal(t, [][]byte{[]byte(line)}, out)
			})
		})
	}

	t.Run("when a scrubbed transcript is scrubbed again", func(t *testing.T) {
		once := scrub([][]byte{[]byte(`{"type":"user"}`), []byte(dropped["an API request"])})

		t.Run("it should change nothing more", func(t *testing.T) {
			assert.Equal(t, once, scrub(once))
		})
	})
}

func TestRetitle(t *testing.T) {
	corpus := t.TempDir()
	id := SessionID("tidepool", "fix")
	path := filepath.Join(corpus, "tidepool", id+".jsonl")
	writeFile(t, path, `{"type":"user","sessionId":"`+id+`","cwd":"/home/demo/src/tidepool","timestamp":"2026-09-01T10:00:00.000Z"}`+"\n")
	script := Script{Project: "tidepool", Sessions: []ScriptedSession{{Label: "fix", Title: "Fix the leap day"}, {Label: "unrecorded", Title: "Never ran"}}}

	t.Run("when a recorded session's script gives it a title", func(t *testing.T) {
		require.NoError(t, Retitle(corpus, script))
		require.NoError(t, Retitle(corpus, script))
		tr, err := claude.ReadTranscript(path)
		require.NoError(t, err)

		t.Run("it should carry the title, as /rename would have written it", func(t *testing.T) {
			assert.Equal(t, "Fix the leap day", tr.Title)
		})

		t.Run("it should write it once however often it is run", func(t *testing.T) {
			assert.Len(t, readLines(t, path), 2)
		})
	})
}
