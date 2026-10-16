package find_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/state"
)

var t0 = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

func stamp(at time.Time) string { return at.Format(time.RFC3339Nano) }

func prompt(id string, at time.Time, text string) string {
	return fmt.Sprintf(`{"type":"user","timestamp":%q,"sessionId":%q,"cwd":"/src/kiln","gitBranch":"main","message":{"role":"user","content":%q}}`, stamp(at), id, text)
}

func reply(id string, at time.Time, text string) string {
	return fmt.Sprintf(`{"type":"assistant","timestamp":%q,"sessionId":%q,"message":{"id":"m","role":"assistant","content":[{"type":"text","text":%q}]}}`, stamp(at), id, text)
}

func recap(id string, at time.Time, text string) string {
	return fmt.Sprintf(`{"type":"system","subtype":"away_summary","timestamp":%q,"sessionId":%q,"content":%q}`, stamp(at), id, text)
}

func command(id string, at time.Time, text string) string {
	return fmt.Sprintf(`{"type":"assistant","timestamp":%q,"sessionId":%q,"message":{"id":"m","role":"assistant","content":[{"type":"tool_use","id":"t","name":"Bash","input":{"command":%q}}]}}`, stamp(at), id, text)
}

func title(id string, at time.Time, text string) string {
	return fmt.Sprintf(`{"type":"ai-title","timestamp":%q,"sessionId":%q,"aiTitle":%q}`, stamp(at), id, text)
}

func toolOutput(id string, at time.Time, text string) string {
	return fmt.Sprintf(`{"type":"user","timestamp":%q,"sessionId":%q,"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t","content":%q}]}}`, stamp(at), id, text)
}

// home is a Claude home in a temporary directory, removed after the test.
type home struct {
	t   *testing.T
	dir string
}

func newHome(t *testing.T) home {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".claude", "projects", "-src-kiln"), 0o755))
	return home{t: t, dir: dir}
}

func (h home) transcript(id string) string {
	return filepath.Join(h.dir, ".claude", "projects", "-src-kiln", id+".jsonl")
}

func (h home) write(id string, lines ...string) home {
	h.t.Helper()
	require.NoError(h.t, os.WriteFile(h.transcript(id), []byte(strings.Join(lines, "\n")+"\n"), 0o644))
	return h
}

func (h home) append(id string, lines ...string) {
	h.t.Helper()
	f, err := os.OpenFile(h.transcript(id), os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(h.t, err)
	_, err = f.WriteString(strings.Join(lines, "\n") + "\n")
	require.NoError(h.t, err)
	require.NoError(h.t, f.Close())
	// A second write inside the clock's resolution must still be seen.
	later := time.Now().Add(time.Second)
	require.NoError(h.t, os.Chtimes(h.transcript(id), later, later))
}

func (h home) history(lines ...string) home {
	h.t.Helper()
	require.NoError(h.t, os.WriteFile(filepath.Join(h.dir, ".claude", "history.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644))
	return h
}

func (h home) index() *find.Index {
	h.t.Helper()
	ix := find.New(h.dir)
	require.NoError(h.t, ix.Refresh())
	return ix
}

func ids(hits []find.Hit) []string {
	var out []string
	for _, h := range hits {
		out = append(out, h.SessionID)
	}
	return out
}

func TestIndex(t *testing.T) {
	t.Run("when a transcript is indexed", func(t *testing.T) {
		ix := newHome(t).write("s1",
			prompt("s1", t0, "why does the kiln overheat"),
			reply("s1", t0, "The thermocouple is loose.")).index()

		t.Run("and every word of the query is in one passage", func(t *testing.T) {
			t.Run("it should find the session", func(t *testing.T) {
				assert.Equal(t, []string{"s1"}, ids(ix.Find(find.Query{Words: "kiln overheat"})))
			})
		})

		t.Run("and the words are only spread across passages", func(t *testing.T) {
			t.Run("it should not find it", func(t *testing.T) {
				assert.Empty(t, ix.Find(find.Query{Words: "kiln thermocouple"}))
			})
		})

		t.Run("and the query differs in case", func(t *testing.T) {
			t.Run("it should find the session", func(t *testing.T) {
				assert.Equal(t, []string{"s1"}, ids(ix.Find(find.Query{Words: "KILN Overheat"})))
			})
		})
	})

	t.Run("when only tool output holds the words", func(t *testing.T) {
		ix := newHome(t).write("s1",
			prompt("s1", t0, "run the tests"),
			toolOutput("s1", t0, "FAIL thermocouple_test.go")).index()

		t.Run("it should not find the session", func(t *testing.T) {
			assert.Empty(t, ix.Find(find.Query{Words: "thermocouple"}))
		})

		t.Run("and the tool-output search is asked for", func(t *testing.T) {
			t.Run("it should find it from the transcript", func(t *testing.T) {
				assert.Equal(t, []string{"s1"}, ids(ix.Find(find.Query{Words: "thermocouple", ToolOutput: true})))
			})
		})
	})

	t.Run("when the transcript is gone and history.jsonl holds its prompts", func(t *testing.T) {
		hits := newHome(t).history(
			fmt.Sprintf(`{"display":"glaze the orchard pots","timestamp":%d,"project":"/src/orchard","sessionId":"s-gone"}`, t0.UnixMilli()),
		).index().Find(find.Query{Words: "glaze"})

		t.Run("it should find it by its prompts", func(t *testing.T) {
			assert.Equal(t, []string{"s-gone"}, ids(hits))
		})

		t.Run("it should say its transcript is gone", func(t *testing.T) {
			require.Len(t, hits, 1)
			assert.True(t, hits[0].Gone)
		})
	})

	t.Run("when a transcript is deleted after indexing", func(t *testing.T) {
		h := newHome(t).write("s1", prompt("s1", t0, "why does the kiln overheat"))
		ix := h.index()
		require.Len(t, ix.Find(find.Query{Words: "kiln"}), 1, "the transcript should be found before it is deleted")
		require.NoError(t, os.Remove(h.transcript("s1")))

		t.Run("and the index is refreshed", func(t *testing.T) {
			require.NoError(t, ix.Refresh())

			t.Run("it should drop what only that transcript said", func(t *testing.T) {
				assert.Empty(t, ix.Find(find.Query{Words: "kiln"}))
			})
		})
	})

	t.Run("when a transcript grows after indexing", func(t *testing.T) {
		h := newHome(t).write("s1", prompt("s1", t0, "why does the kiln overheat"))
		ix := h.index()
		h.append("s1", reply("s1", t0, "The thermocouple is loose."))

		t.Run("and the index is refreshed", func(t *testing.T) {
			require.NoError(t, ix.Refresh())

			t.Run("it should find what was added", func(t *testing.T) {
				assert.Equal(t, []string{"s1"}, ids(ix.Find(find.Query{Words: "thermocouple"})))
			})
		})
	})

	t.Run("when the index is refreshed", func(t *testing.T) {
		h := newHome(t).write("s1", prompt("s1", t0, "why does the kiln overheat")).
			history(fmt.Sprintf(`{"display":"kiln","timestamp":%d,"project":"/src/kiln","sessionId":"s1"}`, t0.UnixMilli()))
		before := tree(t, h.dir)
		ix := h.index()
		ix.Find(find.Query{Words: "kiln", ToolOutput: true})

		t.Run("it should write nothing under the home", func(t *testing.T) {
			assert.Equal(t, before, tree(t, h.dir))
		})
	})
}

// tree is every path under dir with its size and modification time.
func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	require.NoError(t, filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		out[path] = fmt.Sprintf("%d %s", info.Size(), info.ModTime())
		return nil
	}))
	return out
}

func TestHit(t *testing.T) {
	t.Run("when a session has a recap", func(t *testing.T) {
		hits := newHome(t).write("s1",
			prompt("s1", t0, "why does the kiln overheat"),
			recap("s1", t0.Add(time.Minute), "Looking at the thermocouple."),
			recap("s1", t0.Add(2*time.Minute), "Fixing the kiln's thermocouple."),
			prompt("s1", t0.Add(3*time.Minute), "now tighten the clamp")).
			index().Find(find.Query{Words: "kiln overheat"})
		require.Len(t, hits, 1)

		t.Run("it should carry the recap with the hit", func(t *testing.T) {
			assert.Equal(t, "Fixing the kiln's thermocouple.", hits[0].Recap)
		})

		t.Run("it should carry its last prompt", func(t *testing.T) {
			assert.Equal(t, "now tighten the clamp", hits[0].LastPrompt)
		})
	})
}

func TestRanking(t *testing.T) {
	fields := []struct {
		name string
		line func(id string, at time.Time, text string) string
	}{
		{"prompt", prompt},
		{"recap", recap},
		{"reply", reply},
		{"command", command},
		{"title", title},
	}

	t.Run("when sessions match in different fields", func(t *testing.T) {
		for i := 0; i+1 < len(fields); i++ {
			stronger, weaker := fields[i], fields[i+1]
			t.Run("and one matches in a "+stronger.name+" and the other in a "+weaker.name, func(t *testing.T) {
				hits := newHome(t).
					write("weaker", weaker.line("weaker", t0.Add(time.Hour), "kiln")).
					write("stronger", stronger.line("stronger", t0, "kiln")).
					index().Find(find.Query{Words: "kiln"})

				t.Run("it should rank the "+stronger.name+" first", func(t *testing.T) {
					assert.Equal(t, []string{"stronger", "weaker"}, ids(hits))
				})
			})
		}
	})

	t.Run("when two sessions match in the same field", func(t *testing.T) {
		hits := newHome(t).
			write("older", prompt("older", t0, "kiln")).
			write("newer", prompt("newer", t0.Add(time.Hour), "kiln")).
			index().Find(find.Query{Words: "kiln"})

		t.Run("it should rank the more recent first", func(t *testing.T) {
			assert.Equal(t, []string{"newer", "older"}, ids(hits))
		})
	})

	for _, st := range []state.State{state.Working, state.Waiting, state.NeedsYou, state.Unattended} {
		t.Run("when a "+string(st)+" session matches in a weaker field", func(t *testing.T) {
			hits := newHome(t).
				write("stopped", prompt("stopped", t0.Add(time.Hour), "kiln")).
				write("live", title("live", t0, "kiln")).
				index().Find(find.Query{Words: "kiln", Live: map[string]state.State{"live": st, "stopped": state.Parked}})

			t.Run("it should rank it above a stopped one", func(t *testing.T) {
				assert.Equal(t, []string{"live", "stopped"}, ids(hits))
			})
		})
	}

	t.Run("when more sessions match than the limit", func(t *testing.T) {
		h := newHome(t)
		for i := range 4 {
			id := fmt.Sprintf("s%d", i)
			h.write(id, prompt(id, t0.Add(time.Duration(i)*time.Minute), "kiln"))
		}
		hits := h.index().Find(find.Query{Words: "kiln", Limit: 2})

		t.Run("it should return only the limit", func(t *testing.T) {
			assert.Len(t, hits, 2)
		})
	})
}
