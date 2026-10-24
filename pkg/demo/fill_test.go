package demo

import (
	"encoding/json"
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

func turn(at string, cacheCreate, cacheRead, output int) string {
	return turnSaying(at, `{"type":"text","text":"done"}`, cacheCreate, cacheRead, output)
}

func turnSaying(at, block string, cacheCreate, cacheRead, output int) string {
	return `{"type":"assistant","sessionId":"` + recordedID + `","cwd":"` + cwd + `","timestamp":"` + at +
		`","message":{"id":"msg_rec_` + at + `","role":"assistant","model":"claude-sonnet-5-5","content":[` + block + `],` +
		`"usage":{"input_tokens":4,"cache_creation_input_tokens":` + strconv.Itoa(cacheCreate) +
		`,"cache_read_input_tokens":` + strconv.Itoa(cacheRead) + `,"output_tokens":` + strconv.Itoa(output) + `}}}`
}

// usageCorpus is one recorded tidepool session that opened at 10:00 and
// took three turns: an answer at 10:05, a turn that only ran a tool at
// 10:07, and an answer at 10:10.
func usageCorpus(t *testing.T) Corpus {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tidepool", recordedID+".jsonl"), strings.Join([]string{
		line("user", "2026-09-01T10:00:00.000Z"),
		turn("2026-09-01T10:05:00.000Z", 3000, 10000, 200),
		turnSaying("2026-09-01T10:07:00.000Z", `{"type":"tool_use","id":"toolu_1","name":"Bash","input":{}}`, 2000, 15000, 60),
		turn("2026-09-01T10:10:00.000Z", 1000, 20000, 400),
	}, "\n")+"\n")
	return Corpus{Dir: dir}
}

type stagedTurn struct {
	at          time.Time
	id, model   string
	content     []any
	cacheRead   int64
	output      int64
	contextUsed int64
}

func mainTurns(t *testing.T, path string) []stagedTurn {
	t.Helper()
	var out []stagedTurn
	for _, l := range readLines(t, path) {
		var rec struct {
			Type      string `json:"type"`
			Timestamp string `json:"timestamp"`
			Message   struct {
				ID      string `json:"id"`
				Model   string `json:"model"`
				Content []any  `json:"content"`
				Usage   struct {
					Input       int64 `json:"input_tokens"`
					CacheCreate int64 `json:"cache_creation_input_tokens"`
					CacheRead   int64 `json:"cache_read_input_tokens"`
					Output      int64 `json:"output_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		require.NoError(t, json.Unmarshal(l, &rec))
		if rec.Type != "assistant" {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, rec.Timestamp)
		require.NoError(t, err)
		u := rec.Message.Usage
		out = append(out, stagedTurn{at: at, id: rec.Message.ID, model: rec.Message.Model, content: rec.Message.Content,
			cacheRead: u.CacheRead, output: u.Output, contextUsed: u.Input + u.CacheCreate + u.CacheRead})
	}
	return out
}

func TestStageFillsAPacedSessionsHours(t *testing.T) {
	now := time.Date(2026, 10, 7, 15, 0, 0, 0, time.UTC)
	corpus := usageCorpus(t)
	transcript := func(home string) string {
		return filepath.Join(home, ".claude", "projects", claude.ProjectFolder(cwd), recordedID+".jsonl")
	}
	pace := Duration(30 * time.Second)
	working := Placement{Ref: "tidepool/6f1d", State: state.Working, Ago: Duration(2 * time.Minute), Started: Duration(3 * time.Hour), Context: "60%"}

	t.Run("when a session is placed with started hours before its recording, in a paced scenario", func(t *testing.T) {
		got := stage(t, corpus, Scenario{Pace: pace, Sessions: []Placement{working}}, now)
		require.NoError(t, got.err)
		turns := mainTurns(t, transcript(got.home))
		recordingStarts := now.Add(-7 * time.Minute)
		var earlier []stagedTurn
		for _, tr := range turns {
			if tr.at.Before(recordingStarts) {
				earlier = append(earlier, tr)
			}
		}

		t.Run("it should add one main-line turn per pace between the opening and the recording", func(t *testing.T) {
			assert.Len(t, earlier, int((3*time.Hour-7*time.Minute)/(30*time.Second))-1)
		})

		t.Run("it should keep the recording's own newest line as the transcript's newest", func(t *testing.T) {
			latest, _ := Latest(readLines(t, transcript(got.home)))
			assert.Equal(t, now.Add(-2*time.Minute), latest)
		})

		t.Run("it should leave the context the placement sets on the last turn", func(t *testing.T) {
			assert.EqualValues(t, 120_000, turns[len(turns)-1].contextUsed)
		})

		t.Run("it should read no earlier turn's cache above that context", func(t *testing.T) {
			over := 0
			for _, tr := range earlier {
				if tr.cacheRead > 120_000 {
					over++
				}
			}
			assert.Zero(t, over)
		})

		t.Run("it should give every earlier turn the recording's model", func(t *testing.T) {
			models := map[string]bool{}
			for _, tr := range earlier {
				models[tr.model] = true
			}
			assert.Equal(t, map[string]bool{"claude-sonnet-5-5": true}, models)
		})

		t.Run("it should have every earlier turn write what the recording's tool-only turns write", func(t *testing.T) {
			outputs := map[int64]bool{}
			for _, tr := range earlier {
				outputs[tr.output] = true
			}
			assert.Equal(t, map[int64]bool{60: true}, outputs)
		})

		t.Run("it should give every turn a message id of its own", func(t *testing.T) {
			ids := map[string]bool{}
			for _, tr := range turns {
				if tr.id != "" {
					ids[tr.id] = true
				}
			}
			assert.Len(t, ids, len(turns))
		})

		t.Run("it should give the earlier turns no text, tool call or prompt", func(t *testing.T) {
			var content []any
			for _, tr := range earlier {
				content = append(content, tr.content...)
			}
			assert.Empty(t, content)
		})
	})

	t.Run("when the scenario sets no pace", func(t *testing.T) {
		got := stage(t, corpus, Scenario{Sessions: []Placement{working}}, now)
		require.NoError(t, got.err)

		t.Run("it should add no turns", func(t *testing.T) {
			assert.Len(t, mainTurns(t, transcript(got.home)), 3)
		})
	})

	t.Run("when a session is placed without started", func(t *testing.T) {
		unstarted := working
		unstarted.Started = 0
		got := stage(t, corpus, Scenario{Pace: pace, Sessions: []Placement{unstarted}}, now)
		require.NoError(t, got.err)

		t.Run("it should add no turns", func(t *testing.T) {
			assert.Len(t, mainTurns(t, transcript(got.home)), 3)
		})
	})
}
