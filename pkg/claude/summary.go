package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"botropolis.city/botropolis/pkg/statedir"
)

// excerptWindow is how much of a transcript's tail an excerpt reads. A
// transcript can run to megabytes, almost all of it tool output, and a
// summary of where a session got to wants the end of it.
const excerptWindow = 2 << 20

// SummaryModel is the model a summary is generated with: a summary does
// not need the session's own model, and a small one answers faster.
const SummaryModel = "haiku"

const summaryPrompt = `Below is the end of a Claude Code session's conversation.
In two or three sentences of plain prose, say what the work is, where it got to, and what is next.
No preamble, no headings, no lists.

`

type excerptLineJSON struct {
	Type        string `json:"type"`
	IsSidechain bool   `json:"isSidechain"`
	IsMeta      bool   `json:"isMeta"`
	Message     struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type excerptBlockJSON struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Excerpt is the end of a session's conversation as plain text: the
// main line's typed prompts and replies, newest last, without tool
// calls, tool output, subagents or the CLI's own wrapped messages, and
// no longer than limit bytes. Older turns are dropped whole.
func Excerpt(path string, limit int) (string, error) {
	f, err := openTranscript(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	partial := info.Size() > excerptWindow
	if partial {
		if _, err := f.Seek(info.Size()-excerptWindow, io.SeekStart); err != nil {
			return "", err
		}
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(nil, maxTranscriptLine)
	var turns []string
	for first := true; scanner.Scan(); first = false {
		if first && partial {
			continue
		}
		var rec excerptLineJSON
		if err := json.Unmarshal(bytes.TrimLeft(scanner.Bytes(), "\x00"), &rec); err != nil {
			continue
		}
		if turn := excerptTurn(rec); turn != "" {
			turns = append(turns, turn)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	kept, size := 0, 0
	for i := len(turns) - 1; i >= 0; i-- {
		size += len(turns[i]) + 2
		if size-2 > limit {
			break
		}
		kept++
	}
	return strings.Join(turns[len(turns)-kept:], "\n\n"), nil
}

func excerptTurn(rec excerptLineJSON) string {
	if rec.IsSidechain || rec.IsMeta {
		return ""
	}
	var speaker string
	switch rec.Type {
	case "user":
		speaker = "User: "
	case "assistant":
		speaker = "Claude: "
	default:
		return ""
	}
	var text string
	if err := json.Unmarshal(rec.Message.Content, &text); err != nil {
		var blocks []excerptBlockJSON
		if json.Unmarshal(rec.Message.Content, &blocks) != nil {
			return ""
		}
		var parts []string
		for _, b := range blocks {
			if b.Type == "tool_result" {
				return ""
			}
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		text = strings.Join(parts, "\n")
	}
	text = strings.TrimSpace(text)
	if text == "" || strings.HasPrefix(text, "<") {
		return ""
	}
	return speaker + text
}

// ErrPrivate is why a run that is not the person's own city does not
// summarise: a frame must not spend tokens (item 111).
var ErrPrivate = errors.New("this run is not your own city, so it generates no summaries")

// Summarise asks Claude Code for a summary of an excerpt, through the
// user's own CLI and its own login.
//
// The run must not become a building, and each flag closes one way it
// could. --no-session-persistence writes no transcript, so there is no
// parked session afterwards and nothing to prune. The working directory
// is SummaryDir, which the loader fences out, because a print run still
// registers a live session record for as long as it runs. An empty
// --setting-sources loads no user settings, so neither the user's hooks
// nor their plugins fire for a run they did not start. And --tools ""
// leaves it nothing to do but answer.
//
// --strict-mcp-config is not about the map. Without it every connected
// MCP server's tool definitions ride along, and on a machine with a few
// connectors that was ~320k tokens of prompt for a one-paragraph answer:
// over the window, so the run failed outright. With it, ~6.5k.
//
// Not --bare, which skips hooks too: it also refuses OAuth, so it would
// need an API key, which is the thing using the CLI was chosen to avoid.
//
// The excerpt goes on stdin: it is the session's own words, and the
// command line is visible to every process on the machine.
func Summarise(ctx context.Context, bin, excerpt string) (string, error) {
	if statedir.IsPrivate() {
		return "", ErrPrivate
	}
	dir := SummaryDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if bin == "" {
		bin = "claude"
	}
	cmd := exec.CommandContext(ctx, bin, "-p", "--no-session-persistence",
		"--setting-sources", "", "--strict-mcp-config", "--tools", "", "--model", SummaryModel)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(summaryPrompt + excerpt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	removeEmptySummaryFolder()
	if err != nil {
		// Claude Code reports some failures on stdout, a prompt too long
		// among them, so stderr is not the only place to look.
		for _, out := range []string{stderr.String(), stdout.String()} {
			if msg := strings.TrimSpace(out); msg != "" {
				return "", fmt.Errorf("claude: %s", lastLine(msg))
			}
		}
		return "", fmt.Errorf("claude: %w", err)
	}
	summary := strings.TrimSpace(stdout.String())
	if summary == "" {
		return "", errors.New("claude returned an empty summary")
	}
	return summary, nil
}

// removeEmptySummaryFolder clears what an unpersisted run still leaves:
// Claude Code makes the project folder and an empty memory folder in it
// even when it writes no transcript. os.Remove refuses a folder with
// anything in it, so this can only ever take away what is empty.
func removeEmptySummaryFolder() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	folder := filepath.Join(home, ".claude", "projects", ProjectFolder(SummaryDir()))
	_ = os.Remove(filepath.Join(folder, "memory"))
	_ = os.Remove(folder)
}

func lastLine(s string) string {
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}
