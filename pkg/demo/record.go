package demo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"go.yaml.in/yaml/v3"

	"botropolis.city/botropolis/pkg/claude"
)

// Script is what to record in one toy project: an ordered list of
// sessions, each one or more prompts.
type Script struct {
	Project  string            `yaml:"project"`
	Sessions []ScriptedSession `yaml:"sessions"`
}

type ScriptedSession struct {
	Label string `yaml:"label"`
	// Title is the session's name on the map. A `claude -p` session
	// never gets the title an interactive one is given, so the corpus
	// carries this one as the custom-title record /rename writes.
	Title     string   `yaml:"title"`
	Model     string   `yaml:"model"`
	BudgetUSD float64  `yaml:"budget_usd"`
	Visual    string   `yaml:"visual"`
	Prompts   []string `yaml:"prompts"`
	// Team records the session as an agent team's lead, interactively
	// through the recorder's Terminal, from its one prompt (item 105).
	Team bool `yaml:"team"`
}

// LoadScript reads a script, refusing unknown keys and a session with
// no label, no prompt or no budget: an uncapped prompt is the one way
// a recording run spends more than it says it will.
func LoadScript(path string) (Script, error) {
	f, err := os.Open(path)
	if err != nil {
		return Script{}, err
	}
	defer func() { _ = f.Close() }()
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	var s Script
	if err := dec.Decode(&s); err != nil {
		return Script{}, fmt.Errorf("%s: %w", path, err)
	}
	seen := map[string]bool{}
	for i, session := range s.Sessions {
		switch {
		case session.Label == "":
			return Script{}, fmt.Errorf("%s: session %d has no label", path, i)
		case seen[session.Label]:
			return Script{}, fmt.Errorf("%s: two sessions labelled %q", path, session.Label)
		case len(session.Prompts) == 0:
			return Script{}, fmt.Errorf("%s: %s has no prompts", path, session.Label)
		case session.BudgetUSD <= 0:
			return Script{}, fmt.Errorf("%s: %s has no budget_usd", path, session.Label)
		case session.Team && len(session.Prompts) != 1:
			return Script{}, fmt.Errorf("%s: %s is a team, which is recorded from one prompt, not %d", path, session.Label, len(session.Prompts))
		}
		seen[session.Label] = true
	}
	return s, nil
}

// Retitle gives every recorded session of the script that has a title
// in the script a custom-title record, once.
func Retitle(corpus string, script Script) error {
	for _, session := range script.Sessions {
		if session.Title == "" {
			continue
		}
		id := SessionID(script.Project, session.Label)
		path := filepath.Join(corpus, script.Project, id+".jsonl")
		lines, err := readJSONL(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		title, err := json.Marshal(map[string]any{"type": "custom-title", "customTitle": session.Title, "sessionId": id})
		if err != nil {
			return err
		}
		titled := false
		for _, line := range lines {
			titled = titled || bytes.Equal(line, title)
		}
		if titled {
			continue
		}
		if err := writeJSONL(path, append(lines, title)); err != nil {
			return err
		}
	}
	return nil
}

// Ceiling is the most the script can spend: every prompt at its cap.
func (s Script) Ceiling() float64 {
	var total float64
	for _, session := range s.Sessions {
		total += session.BudgetUSD * float64(len(session.Prompts))
	}
	return total
}

// SessionID is the id a scripted session is recorded under. It is
// derived from the label, so a scenario can name a session by label
// and a re-run knows what it has already recorded.
func SessionID(project, label string) string {
	sum := sha256.Sum256([]byte("botropolis-demo/" + project + "/" + label))
	sum[6] = sum[6]&0x0f | 0x40
	sum[8] = sum[8]&0x3f | 0x80
	h := fmt.Sprintf("%x", sum[:16])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// Claude runs the claude CLI in dir and returns what it printed.
type Claude func(ctx context.Context, dir string, args []string) ([]byte, error)

// Recorder records scripted sessions with the real claude CLI, inside
// the recorder container, and exports nothing but their transcripts.
type Recorder struct {
	// Home is the home claude writes to; Src holds the toy projects.
	Home, Src string
	Corpus    string
	MCPConfig string
	Claude    Claude
	// Image is the recorder image's ID, logged on every prompt, so a
	// recording made in the wrong image shows afterwards (item 105).
	Image string
	// Terminal runs a team session, and Team holds it to its limits on
	// TeamClock, the real clock when nil.
	Terminal  Terminal
	Team      TeamLimits
	TeamClock TeamClock
	Log       io.Writer
	// Progress is told of each prompt as it starts and ends.
	Progress *Progress
}

// Prompted is what one prompt cost, for the recording log.
type Prompted struct {
	Project string  `json:"project"`
	Label   string  `json:"label"`
	Prompt  int     `json:"prompt"`
	CostUSD float64 `json:"cost_usd"`
	Error   string  `json:"error,omitempty"`
	Image   string  `json:"image,omitempty"`
}

// Record records each session of the script the corpus does not hold
// yet, prompt by prompt, and exports it.
func (r Recorder) Record(ctx context.Context, script Script) ([]Prompted, error) {
	var log []Prompted
	for _, session := range script.Sessions {
		id := SessionID(script.Project, session.Label)
		if _, err := os.Stat(filepath.Join(r.Corpus, script.Project, id+".jsonl")); err == nil {
			r.logf("skip %s/%s: already in the corpus\n", script.Project, session.Label)
			continue
		}
		if session.Team {
			r.logf("record %s/%s as a team (%s, cap $%.2f)\n", script.Project, session.Label, session.Model, session.BudgetUSD)
			if r.Progress != nil {
				r.Progress.Step(fmt.Sprintf("%s/%s", script.Project, session.Label), "team, "+session.Model)
			}
			entry, err := r.recordTeam(ctx, script.Project, session, id)
			if err != nil {
				entry.Error = err.Error()
			}
			log = append(log, entry)
			if err != nil {
				return log, fmt.Errorf("%s/%s: %w", script.Project, session.Label, err)
			}
			if r.Progress != nil {
				r.Progress.Advance(1)
			}
			continue
		}
		for i, prompt := range session.Prompts {
			r.logf("record %s/%s prompt %d/%d (%s, cap $%.2f)\n", script.Project, session.Label, i+1, len(session.Prompts), session.Model, session.BudgetUSD)
			if r.Progress != nil {
				r.Progress.Step(fmt.Sprintf("%s/%s", script.Project, session.Label), fmt.Sprintf("prompt %d/%d, %s", i+1, len(session.Prompts), session.Model))
			}
			out, err := r.Claude(ctx, filepath.Join(r.Src, script.Project), claudeArgs(session, id, i, prompt, r.MCPConfig))
			entry := Prompted{Project: script.Project, Label: session.Label, Prompt: i, Image: r.Image}
			var result struct {
				SessionID string  `json:"session_id"`
				CostUSD   float64 `json:"total_cost_usd"`
			}
			if json.Unmarshal(out, &result) == nil {
				entry.CostUSD = result.CostUSD
				if result.SessionID != "" && result.SessionID != id {
					err = errors.Join(err, fmt.Errorf("claude answered as %s, not %s", result.SessionID, id))
				}
			}
			if err != nil {
				entry.Error = err.Error()
				log = append(log, entry)
				return log, fmt.Errorf("%s/%s prompt %d: %w", script.Project, session.Label, i+1, err)
			}
			log = append(log, entry)
			if r.Progress != nil {
				r.Progress.Advance(1)
			}
		}
		if err := r.export(script.Project, id); err != nil {
			return log, err
		}
	}
	return log, Retitle(r.Corpus, script)
}

func claudeArgs(s ScriptedSession, id string, i int, prompt, mcpConfig string) []string {
	args := []string{"-p", prompt, "--model", s.Model,
		"--max-budget-usd", strconv.FormatFloat(s.BudgetUSD, 'f', -1, 64),
		"--dangerously-skip-permissions", "--output-format", "json", "--mcp-config", mcpConfig}
	if i == 0 {
		return append(args, "--session-id", id)
	}
	return append(args, "--resume", id)
}

// export copies the session's transcript and its subagents, and
// nothing else, out of the recorder's home into the corpus. The home
// also holds the account's details; nothing from it but these leaves.
func (r Recorder) export(project, id string) error {
	folder := filepath.Join(r.Home, ".claude", "projects", claude.ProjectFolder(filepath.Join(r.Src, project)))
	if err := exportFile(filepath.Join(folder, id+".jsonl"), filepath.Join(r.Corpus, project, id+".jsonl")); err != nil {
		return err
	}
	subs, _ := filepath.Glob(filepath.Join(folder, id, "subagents", "*"))
	for _, p := range subs {
		if err := exportFile(p, filepath.Join(r.Corpus, project, id, "subagents", filepath.Base(p))); err != nil {
			return err
		}
	}
	return nil
}

func exportFile(src, dst string) error {
	if filepath.Ext(src) != ".jsonl" {
		return copyFile(src, dst)
	}
	lines, err := readJSONL(src)
	if err != nil {
		return err
	}
	return writeJSONL(dst, scrub(lines))
}

// kept is every line type the app reads from a Claude Code transcript.
// An attachment is kept only for the model it names. agent-name is here
// because the app reads agentName from any line, and that is the line
// Claude Code writes it on. tools/check-demo-corpus holds the same list.
var kept = map[string]bool{
	"user": true, "assistant": true, "system": true, "attachment": true,
	"custom-title": true, "ai-title": true, "summary": true, "last-prompt": true,
	"pr-link": true, "cost-state": true, "bridge-session": true, "agent-name": true,
}

// keptAttachments are the attachment types the app reads.
var keptAttachments = map[string]bool{"model": true}

func keptTypes() []string {
	out := make([]string, 0, len(kept))
	for k := range kept {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Scrub applies the export's scrub to a transcript already on disk, so a
// corpus recorded before a scrub rule existed can be brought up to it.
func Scrub(path string) error {
	lines, err := readJSONL(path)
	if err != nil {
		return err
	}
	return writeJSONL(path, scrub(lines))
}

// scrub keeps only the lines the app reads, each exactly as written, and
// drops the rest. Claude Code writes its own system prompt, tool and
// skill lists and API settings into a transcript in more shapes with
// each version, none of them ours to publish, and the environment holds
// the host's kernel, which a container shares; an allowlist drops the
// next version's new shapes by default instead of publishing them
// (item 105).
func scrub(lines [][]byte) [][]byte {
	out := make([][]byte, 0, len(lines))
	for _, line := range lines {
		var probe struct {
			Type       string `json:"type"`
			Attachment struct {
				Type string `json:"type"`
			} `json:"attachment"`
		}
		if json.Unmarshal(line, &probe) != nil || !kept[probe.Type] {
			continue
		}
		if probe.Type == "attachment" && !keptAttachments[probe.Attachment.Type] {
			continue
		}
		out = append(out, line)
	}
	return out
}

func (r Recorder) logf(format string, args ...any) {
	if r.Log != nil {
		fmt.Fprintf(r.Log, format, args...)
	}
}
