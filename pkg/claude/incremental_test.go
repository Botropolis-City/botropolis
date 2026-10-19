package claude_test

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/measure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 141. A live transcript was read again from its first byte every
// time it changed; the reader keeps what it derived and reads only what
// was appended, and must derive exactly what a whole read derives.

const incCWD = "/home/avesta/workspaces/github/mCedar/cinders"

func incLine(kind string, args ...any) string {
	ts := "2026-09-16T19:50:00.000Z"
	if len(args) > 0 {
		if s, ok := args[0].(string); ok && strings.HasPrefix(s, "2026-") {
			ts, args = s, args[1:]
		}
	}
	base := `"sessionId":"` + sid + `","cwd":"` + incCWD + `","timestamp":"` + ts + `"`
	switch kind {
	case "prompt":
		return `{"type":"user",` + base + `,"version":"2.1.273","gitBranch":"main","entrypoint":"cli","message":{"role":"user","content":"do it"}}`
	case "meta":
		return `{"type":"user",` + base + `,"isMeta":true,"message":{"role":"user","content":"meta"}}`
	case "reply":
		return `{"type":"assistant",` + base + `,"effort":"high","message":{"id":"` + args[0].(string) + `","model":"claude-opus-5","stop_reason":"end_turn","usage":{"input_tokens":3,"output_tokens":7,"cache_read_input_tokens":100,"cache_creation_input_tokens":5,"output_tokens_details":{"thinking_tokens":2}},"content":[{"type":"text","text":"done"}]}}`
	case "text":
		return `{"type":"assistant",` + base + `,"message":{"id":"` + args[0].(string) + `","model":"claude-opus-5","usage":{"input_tokens":1,"output_tokens":1},"content":[{"type":"text","text":"` + args[1].(string) + `"}]}}`
	case "tool":
		return `{"type":"assistant",` + base + `,"attributionMcpServer":"` + args[3].(string) + `","attributionSkill":"` + args[4].(string) + `","message":{"id":"` + args[0].(string) + `","model":"claude-sonnet-5","stop_reason":"tool_use","usage":{"input_tokens":2,"output_tokens":4},"content":[{"type":"tool_use","id":"` + args[1].(string) + `","name":"` + args[2].(string) + `","input":` + args[5].(string) + `}]}}`
	case "result":
		return `{"type":"user",` + base + `,"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"` + args[0].(string) + `","content":"ok"}]},"toolUseResult":{"type":"text"}}`
	case "async":
		return `{"type":"user",` + base + `,"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"` + args[0].(string) + `","content":"launched"}]},"toolUseResult":{"isAsync":true}}`
	case "peer":
		return `{"type":"user",` + base + `,"turnOrigin":"peer","origin":{"kind":"peer","verifiedPeerPid":42,"name":"other","msg_id":"` + args[0].(string) + `"},"message":{"role":"user","content":"hi"}}`
	case "sidechain":
		return `{"type":"assistant",` + base + `,"isSidechain":true,"message":{"id":"` + args[0].(string) + `","model":"claude-haiku-5","usage":{"input_tokens":9,"output_tokens":9},"content":[{"type":"tool_use","id":"side_` + args[0].(string) + `","name":"Read","input":{"file_path":"/etc/hosts"}}]}}`
	case "recap":
		return `{"type":"system","subtype":"away_summary",` + base + `,"content":"` + args[0].(string) + `"}`
	case "compact":
		return `{"type":"system",` + base + `,"compactMetadata":{"trigger":"auto","preTokens":1000}}`
	case "apierror":
		return `{"type":"system","subtype":"api_error",` + base + `}`
	case "apierrorreply":
		return `{"type":"assistant",` + base + `,"isApiErrorMessage":true,"message":{"id":"err_` + args[0].(string) + `","content":[{"type":"text","text":"overloaded"}]}}`
	case "prlink":
		return `{"type":"pr-link",` + base + `,"prNumber":` + args[0].(string) + `,"prUrl":"https://github.com/o/r/pull/` + args[0].(string) + `","prRepository":"o/r"}`
	case "praction":
		return `{"type":"system",` + base + `,"pr":{"number":` + args[0].(string) + `,"url":"https://github.com/o/r/pull/` + args[0].(string) + `","action":"` + args[1].(string) + `"}}`
	case "cost":
		return `{"type":"cost-state","sessionId":"` + sid + `","totalCostUSD":` + args[0].(string) + `,"modelUsage":{"claude-opus-5":{"inputTokens":3,"outputTokens":7,"cacheReadInputTokens":100,"costUSD":` + args[0].(string) + `}}}`
	case "model":
		return `{"type":"attachment",` + base + `,"attachment":{"type":"model","identity":{"modelId":"` + args[0].(string) + `"}}}`
	case "lastprompt":
		return `{"type":"last-prompt","sessionId":"` + sid + `","lastPrompt":"` + args[0].(string) + `"}`
	case "aititle":
		return `{"type":"ai-title","sessionId":"` + sid + `","aiTitle":"` + args[0].(string) + `"}`
	case "customtitle":
		return `{"type":"custom-title","sessionId":"` + sid + `","customTitle":"` + args[0].(string) + `"}`
	case "summary":
		return `{"type":"summary","sessionId":"` + sid + `","summary":"` + args[0].(string) + `"}`
	case "team":
		return `{"type":"user",` + base + `,"teamName":"crew","agentName":"lead","gitBranch":"feature/x","version":"2.1.274","message":{"role":"user","content":"team"}}`
	case "bridge":
		return `{"type":"bridge-session","sessionId":"` + sid + `","bridgeSessionId":"b"}`
	}
	panic("no line kind " + kind)
}

// kitchenSink reaches every branch of a transcript's scan, in an order
// where later lines revise what earlier ones set.
func kitchenSink() []string {
	return []string{
		incLine("bridge"),
		incLine("prompt", "2026-09-16T19:50:00.000Z"),
		incLine("model", "claude-opus-5[1m]"),
		incLine("summary", "A summary"),
		incLine("aititle", "An AI title"),
		incLine("summary", "A lower summary"),
		incLine("reply", "2026-09-16T19:50:10.000Z", "msg_01"),
		incLine("text", "msg_02", "Depends-On: o/r#12"),
		incLine("tool", "msg_02", "toolu_01", "Bash", "", "", `{"command":"gh pr view https://github.com/o/r/pull/7"}`),
		incLine("tool", "msg_02", "toolu_01", "Bash", "", "", `{"command":"gh pr view https://github.com/o/r/pull/7"}`),
		incLine("result", "toolu_01"),
		incLine("tool", "2026-09-16T20:10:00.000Z", "msg_03", "toolu_02", "Read", "github", "review", `{"file_path":"/etc/other/file.go"}`),
		incLine("tool", "msg_04", "toolu_03", "Edit", "", "", `{"file_path":"`+incCWD+`/local.go"}`),
		incLine("result", "toolu_02"),
		incLine("tool", "msg_05", "toolu_04", "SendMessage", "", "", `{"to":"item 7"}`),
		incLine("tool", "msg_06", "toolu_05", "Agent", "", "", `{}`),
		incLine("async", "toolu_05"),
		incLine("peer", "pm_1"),
		incLine("peer", "pm_1"),
		incLine("sidechain", "msg_side"),
		incLine("meta"),
		incLine("recap", "2026-09-16T21:00:00.000Z", "Away for a bit"),
		incLine("prompt", "2026-09-16T21:05:00.000Z"),
		incLine("compact", "2026-09-16T21:06:00.000Z"),
		incLine("apierror", "2026-09-16T21:07:00.000Z"),
		incLine("apierrorreply", "2026-09-16T21:07:30.000Z", "a"),
		incLine("prlink", "7"),
		incLine("praction", "7", "merged"),
		incLine("praction", "9", "created"),
		incLine("prlink", "9"),
		incLine("praction", "9", "closed"),
		incLine("cost", "1.25"),
		incLine("customtitle", "My title"),
		incLine("aititle", "A later AI title"),
		incLine("team"),
		`{"type":"user",`,
		"\x00\x00\x00" + incLine("prompt", "2026-09-16T22:00:00.000Z"),
		incLine("lastprompt", "the last prompt"),
		"",
		incLine("tool", "2026-09-16T22:01:00.000Z", "msg_07", "toolu_06", "ScheduleWakeup", "", "", `{}`),
		incLine("cost", "2.5"),
		incLine("tool", "2026-09-16T22:02:00.000Z", "msg_08", "toolu_07", "AskUserQuestion", "", "", `{}`),
	}
}

// randomTranscript draws lines of every kind, seeded, with message and
// tool ids that repeat the way split messages and re-sent calls do.
func randomTranscript(seed int64, n int) []string {
	rng := rand.New(rand.NewSource(seed))
	var lines []string
	msg, tool := 0, 0
	for i := range n {
		ts := fmt.Sprintf("2026-09-%02dT%02d:%02d:%02d.000Z", 16+i/2000, (i/90)%24, (i/3)%60, i%60)
		if rng.Intn(3) > 0 {
			msg++
		}
		if rng.Intn(4) > 0 {
			tool++
		}
		m, tu := fmt.Sprintf("msg_%d", msg), fmt.Sprintf("toolu_%d", tool)
		var line string
		switch rng.Intn(16) {
		case 0, 1:
			line = incLine("prompt", ts)
		case 2, 3:
			line = incLine("reply", ts, m)
		case 4:
			line = incLine("text", ts, m, fmt.Sprintf("see v1.%d.0", rng.Intn(5)))
		case 5, 6:
			names := []string{"Bash", "Read", "Edit", "SendMessage", "ScheduleWakeup", "AskUserQuestion", "Agent"}
			input := []string{`{"command":"git push o/r#3"}`, `{"file_path":"/tmp/x"}`, `{"to":"peer"}`, `{"stop":true}`, `{}`}
			line = incLine("tool", ts, m, tu, names[rng.Intn(len(names))], []string{"", "github"}[rng.Intn(2)], []string{"", "skill"}[rng.Intn(2)], input[rng.Intn(len(input))])
		case 7, 8:
			line = incLine("result", ts, tu)
		case 9:
			line = incLine("async", ts, tu)
		case 10:
			line = incLine("peer", ts, fmt.Sprintf("pm_%d", rng.Intn(5)))
		case 11:
			line = incLine("sidechain", ts, m)
		case 12:
			line = []string{incLine("recap", ts, "away"), incLine("compact", ts), incLine("apierror", ts), incLine("meta", ts)}[rng.Intn(4)]
		case 13:
			n := fmt.Sprint(rng.Intn(4))
			line = []string{incLine("prlink", ts, n), incLine("praction", ts, n, []string{"created", "merged", "closed", "commented"}[rng.Intn(4)])}[rng.Intn(2)]
		case 14:
			line = []string{incLine("cost", fmt.Sprintf("%d.5", i)), incLine("model", ts, "claude-opus-5"), incLine("lastprompt", "p"), incLine("team", ts)}[rng.Intn(4)]
		case 15:
			line = []string{incLine("aititle", "t"), incLine("customtitle", "c"), incLine("summary", "s"), `{"broken`, "\x00\x00" + incLine("prompt", ts)}[rng.Intn(5)]
		}
		lines = append(lines, line)
	}
	return lines
}

func fixtureTranscripts(t *testing.T) map[string][]byte {
	t.Helper()
	paths, err := filepath.Glob("../../testing/helpers/fixtures/synthetic/home/.claude/projects/*/*.jsonl")
	require.NoError(t, err)
	require.NotEmpty(t, paths, "the synthetic fixture home has no transcripts")
	out := map[string][]byte{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		out["fixture "+filepath.Base(path)] = data
	}
	return out
}

func joinLines(lines []string, eol string) []byte {
	return []byte(strings.Join(lines, eol) + eol)
}

func appendBytes(t *testing.T, path string, data []byte) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = f.Write(data)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

func wholeRead(t *testing.T, path string) claude.Transcript {
	t.Helper()
	whole, err := claude.ReadTranscript(path)
	require.NoError(t, err)
	return whole
}

func incRead(t *testing.T, r *claude.TranscriptReader) claude.Transcript {
	t.Helper()
	got, err := r.Read()
	require.NoError(t, err)
	return got
}

// growth is a way a transcript reaches the disk: the sizes of its
// successive appends.
type growth struct {
	name   string
	chunks func(data []byte) [][]byte
}

var growths = []growth{
	{"line by line", func(data []byte) [][]byte {
		var out [][]byte
		for len(data) > 0 {
			i := strings.IndexByte(string(data), '\n')
			if i < 0 {
				i = len(data) - 1
			}
			out = append(out, data[:i+1])
			data = data[i+1:]
		}
		return out
	}},
	{"in byte chunks that cut lines", func(data []byte) [][]byte {
		rng := rand.New(rand.NewSource(int64(len(data))))
		var out [][]byte
		for len(data) > 0 {
			n := min(1+rng.Intn(400), len(data))
			out = append(out, data[:n])
			data = data[n:]
		}
		return out
	}},
	{"one byte at a time across the first lines", func(data []byte) [][]byte {
		var out [][]byte
		for i := 0; i < len(data) && i < 600; i++ {
			out = append(out, data[i:i+1])
		}
		if len(data) > 600 {
			out = append(out, data[600:])
		}
		return out
	}},
	{"all at once", func(data []byte) [][]byte { return [][]byte{data} }},
}

func TestTranscriptReaderEquivalence(t *testing.T) {
	corpora := map[string][]byte{
		"the kitchen sink":                joinLines(kitchenSink(), "\n"),
		"the kitchen sink with CRLF ends": joinLines(kitchenSink(), "\r\n"),
	}
	for seed := int64(1); seed <= 5; seed++ {
		corpora[fmt.Sprintf("random transcript %d", seed)] = joinLines(randomTranscript(seed, 200), "\n")
	}
	for name, data := range fixtureTranscripts(t) {
		corpora[name] = data
	}

	for name, data := range corpora {
		t.Run("when the transcript is "+name, func(t *testing.T) {
			for _, g := range growths {
				t.Run("and it grows "+g.name, func(t *testing.T) {
					path := filepath.Join(t.TempDir(), sid+".jsonl")
					require.NoError(t, os.WriteFile(path, nil, 0o600))
					reader := claude.NewTranscriptReader(path)
					var step int
					var want, got claude.Transcript
					for i, chunk := range g.chunks(data) {
						appendBytes(t, path, chunk)
						step, want, got = i, wholeRead(t, path), incRead(t, reader)
						if !assert.ObjectsAreEqual(want, got) {
							break
						}
					}

					t.Run("it should derive after every append what a whole read derives", func(t *testing.T) {
						assert.Equal(t, want, got, "first differs after append %d", step)
					})
				})
			}
		})
	}
}

func TestTranscriptReader(t *testing.T) {
	newTranscript := func(t *testing.T, lines []string) (string, *claude.TranscriptReader) {
		t.Helper()
		path := filepath.Join(t.TempDir(), sid+".jsonl")
		require.NoError(t, os.WriteFile(path, joinLines(lines, "\n"), 0o600))
		reader := claude.NewTranscriptReader(path)
		_, err := reader.Read()
		require.NoError(t, err)
		return path, reader
	}
	const headCheck = 4096 + 1

	t.Run("when a transcript is read and then a line is appended", func(t *testing.T) {
		path, reader := newTranscript(t, randomTranscript(7, 2000))
		appended := []byte(incLine("prompt", "2026-09-17T09:00:00.000Z") + "\n")
		appendBytes(t, path, appended)
		before := measure.Read(measure.BytesRead)
		got := incRead(t, reader)
		read := measure.Read(measure.BytesRead) - before

		t.Run("it should read only the appended bytes and the head check", func(t *testing.T) {
			assert.LessOrEqual(t, read, int64(len(appended)+headCheck))
		})

		t.Run("it should derive what a whole read derives", func(t *testing.T) {
			assert.Equal(t, wholeRead(t, path), got)
		})
	})

	t.Run("when a result has been returned and more is read after it", func(t *testing.T) {
		path, reader := newTranscript(t, kitchenSink())
		first := incRead(t, reader)
		snapshot := wholeRead(t, path)
		appendBytes(t, path, joinLines(randomTranscript(3, 300), "\n"))
		incRead(t, reader)

		t.Run("it should leave the earlier result as it was", func(t *testing.T) {
			assert.Equal(t, snapshot, first)
		})
	})

	t.Run("when nothing was appended since the last read", func(t *testing.T) {
		path, reader := newTranscript(t, randomTranscript(8, 500))
		before := measure.Read(measure.BytesRead)
		got := incRead(t, reader)
		read := measure.Read(measure.BytesRead) - before

		t.Run("it should read no more than the head check", func(t *testing.T) {
			assert.LessOrEqual(t, read, int64(headCheck))
		})

		t.Run("it should derive what a whole read derives", func(t *testing.T) {
			assert.Equal(t, wholeRead(t, path), got)
		})
	})

	// Each of these leaves the reader's offset pointing somewhere that
	// is no longer the end of what it read, so only a whole read is right.
	changed := []struct {
		name   string
		change func(t *testing.T, path string)
	}{
		{"the file was truncated below what was read", func(t *testing.T, path string) {
			require.NoError(t, os.WriteFile(path, joinLines(kitchenSink()[:5], "\n"), 0o600))
		}},
		{"the file was replaced by another, longer one", func(t *testing.T, path string) {
			next := path + ".next"
			require.NoError(t, os.WriteFile(next, joinLines(randomTranscript(99, 600), "\n"), 0o600))
			require.NoError(t, os.Rename(next, path))
		}},
		{"the file was replaced by another that shares its head and ends a line where it stopped", func(t *testing.T, path string) {
			old, err := os.ReadFile(path)
			require.NoError(t, err)
			lines := strings.SplitAfter(string(old), "\n")
			kept := strings.Join(lines[:len(lines)/2], "")
			pad := len(old) - len(kept) - len(`{"type":"pad","x":""}`) - 1
			require.Positive(t, pad)
			replacement := kept + `{"type":"pad","x":"` + strings.Repeat("x", pad) + `"}` + "\n" +
				string(joinLines(randomTranscript(98, 50), "\n"))
			require.Equal(t, byte('\n'), replacement[len(old)-1])
			next := path + ".next"
			require.NoError(t, os.WriteFile(next, []byte(replacement), 0o600))
			require.NoError(t, os.Rename(next, path))
		}},
		{"the file was rewritten in place with a different head and grew", func(t *testing.T, path string) {
			f, err := os.OpenFile(path, os.O_WRONLY, 0o600)
			require.NoError(t, err)
			_, err = f.WriteAt([]byte(incLine("customtitle", "Rewritten")+"\n"), 0)
			require.NoError(t, err)
			require.NoError(t, f.Close())
			appendBytes(t, path, []byte(incLine("prompt", "2026-09-18T00:00:00.000Z")+"\n"))
		}},
		{"the byte before where it stopped is no longer a newline", func(t *testing.T, path string) {
			info, err := os.Stat(path)
			require.NoError(t, err)
			f, err := os.OpenFile(path, os.O_WRONLY, 0o600)
			require.NoError(t, err)
			_, err = f.WriteAt([]byte(" "), info.Size()-1)
			require.NoError(t, err)
			require.NoError(t, f.Close())
			appendBytes(t, path, []byte(incLine("prompt", "2026-09-18T00:00:00.000Z")+"\n"))
		}},
	}
	for _, c := range changed {
		t.Run("when "+c.name, func(t *testing.T) {
			path, reader := newTranscript(t, randomTranscript(11, 3000))
			c.change(t, path)

			t.Run("it should derive what a whole read derives", func(t *testing.T) {
				assert.Equal(t, wholeRead(t, path), incRead(t, reader))
			})
		})
	}

	t.Run("when the last line is torn mid-write", func(t *testing.T) {
		line := incLine("prompt", "2026-09-17T09:00:00.000Z")
		path, reader := newTranscript(t, kitchenSink())
		appendBytes(t, path, []byte(line[:40]))
		torn := incRead(t, reader)

		t.Run("it should count it malformed, as a whole read does", func(t *testing.T) {
			assert.Equal(t, wholeRead(t, path).Malformed, torn.Malformed)
		})

		t.Run("and the rest of the line arrives", func(t *testing.T) {
			appendBytes(t, path, []byte(line[40:]+"\n"))

			before := measure.Read(measure.BytesRead)
			got := incRead(t, reader)
			read := measure.Read(measure.BytesRead) - before

			t.Run("it should derive what a whole read derives", func(t *testing.T) {
				assert.Equal(t, wholeRead(t, path), got)
			})

			t.Run("it should read only the line and the head check", func(t *testing.T) {
				assert.LessOrEqual(t, read, int64(len(line)+1+headCheck))
			})
		})
	})

	t.Run("when the last line is complete without its newline", func(t *testing.T) {
		line := incLine("reply", "2026-09-17T09:00:00.000Z", "msg_open")
		t.Run("and its newline arrives", func(t *testing.T) {
			path, reader := newTranscript(t, randomTranscript(12, 2000))
			appendBytes(t, path, []byte(line))
			incRead(t, reader)
			appendBytes(t, path, []byte("\n"+incLine("prompt", "2026-09-17T09:01:00.000Z")+"\n"))
			before := measure.Read(measure.BytesRead)
			got := incRead(t, reader)
			read := measure.Read(measure.BytesRead) - before

			t.Run("it should derive what a whole read derives", func(t *testing.T) {
				assert.Equal(t, wholeRead(t, path), got)
			})

			t.Run("it should not read the transcript again", func(t *testing.T) {
				assert.Less(t, read, int64(len(line)+headCheck+1024))
			})
		})

		t.Run("and other bytes carry on the same line", func(t *testing.T) {
			path, reader := newTranscript(t, randomTranscript(13, 200))
			appendBytes(t, path, []byte(line))
			incRead(t, reader)
			appendBytes(t, path, []byte(`garbage`+"\n"+incLine("prompt", "2026-09-17T09:01:00.000Z")+"\n"))

			t.Run("it should derive what a whole read derives", func(t *testing.T) {
				assert.Equal(t, wholeRead(t, path), incRead(t, reader))
			})
		})
	})

	t.Run("when a torn write left NULs where the next record starts", func(t *testing.T) {
		path, reader := newTranscript(t, kitchenSink())
		appendBytes(t, path, []byte("\x00\x00\x00\x00"))
		incRead(t, reader)
		appendBytes(t, path, []byte(incLine("prompt", "2026-09-17T09:00:00.000Z")+"\n"))

		t.Run("it should derive what a whole read derives", func(t *testing.T) {
			assert.Equal(t, wholeRead(t, path), incRead(t, reader))
		})
	})
}

// Item 141: a one-line append to a long transcript must cost what it
// costs on a short one. Every line here falls in one hour and reuses one
// set of names, so the two transcripts differ only in length, and any
// cost that grows with length is the reader paying for what it read
// before.
func TestTranscriptReaderAppendCost(t *testing.T) {
	sameHour := func(n int) []byte {
		var lines []string
		for i := range n {
			ts := fmt.Sprintf("2026-09-16T19:%02d:%02d.000Z", (i/60)%60, i%60)
			lines = append(lines,
				incLine("prompt", ts),
				incLine("tool", ts, fmt.Sprintf("msg_%d", i), fmt.Sprintf("toolu_%d", i), "Read", "", "", `{"file_path":"`+incCWD+`/f.go"}`),
				incLine("result", ts, fmt.Sprintf("toolu_%d", i)))
		}
		return joinLines(lines, "\n")
	}
	appendCost := func(t *testing.T, n int) uint64 {
		t.Helper()
		path := filepath.Join(t.TempDir(), sid+".jsonl")
		require.NoError(t, os.WriteFile(path, sameHour(n), 0o600))
		reader := claude.NewTranscriptReader(path)
		incRead(t, reader)
		line := []byte(incLine("prompt", "2026-09-16T19:59:59.000Z") + "\n")
		const runs = 20
		var total uint64
		for range runs {
			appendBytes(t, path, line)
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			incRead(t, reader)
			runtime.ReadMemStats(&after)
			total += after.TotalAlloc - before.TotalAlloc
		}
		return total / runs
	}

	t.Run("when a line is appended to a transcript a hundred times longer than another", func(t *testing.T) {
		short, long := appendCost(t, 200), appendCost(t, 20000)

		t.Run("it should allocate about what the short one does", func(t *testing.T) {
			assert.LessOrEqual(t, long, short*3/2+16<<10, "short %d bytes, long %d bytes", short, long)
		})
	})
}
