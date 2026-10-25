package demo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/stats"
)

// Run is a claude the recorder started interactively, which it stops
// when the team is done, over its budget, out of time or interrupted.
// Stop ends it and everything it started, and may be called again.
type Run interface {
	Stop() error
}

// Terminal starts claude interactively in dir with env added: agent
// teams spawn no teammates under -p, so a team is recorded this way
// (item 105).
type Terminal func(ctx context.Context, dir string, args, env []string) (Run, error)

// TeamLimits are how often a team recording is checked, how long every
// transcript must be still after the lead's turn ends for it to count
// as done, and the most time it may take.
type TeamLimits struct {
	Poll, Quiet, Time time.Duration
}

// DefaultTeamLimits are a team recording's limits when none are given.
var DefaultTeamLimits = TeamLimits{Poll: 2 * time.Second, Quiet: 30 * time.Second, Time: 10 * time.Minute}

// teamsEnv turns agent teams on for the run.
const teamsEnv = "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1"

func teamArgs(s ScriptedSession, id, prompt, mcpConfig string) []string {
	return []string{prompt, "--model", s.Model, "--dangerously-skip-permissions",
		"--mcp-config", mcpConfig, "--session-id", id, "--teammate-mode", "tmux"}
}

// recordTeam records a team session from its one prompt. --max-budget-usd
// is a -p flag, so the recorder holds the run to its budget itself, by
// pricing every transcript in the team as it grows.
func (r Recorder) recordTeam(ctx context.Context, project string, s ScriptedSession, id string) (Prompted, error) {
	entry := Prompted{Project: project, Label: s.Label}
	limits := r.Team
	if limits == (TeamLimits{}) {
		limits = DefaultTeamLimits
	}
	run, err := r.Terminal(ctx, filepath.Join(r.Src, project), teamArgs(s, id, s.Prompts[0], r.MCPConfig), []string{teamsEnv})
	if err != nil {
		return entry, err
	}
	defer func() { _ = run.Stop() }()
	start := time.Now()
	changed, last := start, ""
	tick := time.NewTicker(limits.Poll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return entry, fmt.Errorf("interrupted, so the run was stopped: %w", ctx.Err())
		case <-tick.C:
		}
		files, err := r.teamFiles(project, id)
		if err != nil {
			return entry, err
		}
		now := time.Now()
		if sig := signature(files); sig != last {
			last, changed = sig, now
		}
		cost, err := price(files)
		if err != nil {
			return entry, err
		}
		entry.CostUSD = cost
		switch {
		case cost > s.BudgetUSD:
			return entry, fmt.Errorf("the team has cost $%.2f, over its budget of $%.2f, so the run was stopped", cost, s.BudgetUSD)
		case leadEnded(files[0]) && now.Sub(changed) >= limits.Quiet:
			if err := run.Stop(); err != nil {
				return entry, err
			}
			return entry, r.exportTeam(project, id, files)
		case now.Sub(start) >= limits.Time:
			return entry, fmt.Errorf("ran out of time after %s, so the run was stopped", limits.Time)
		}
	}
}

// teamFiles is the lead's transcript first, then every other transcript
// in the home that carries its team. Before the lead has written, it is
// the lead's path alone.
func (r Recorder) teamFiles(project, id string) ([]string, error) {
	projects := filepath.Join(r.Home, ".claude", "projects")
	lead := filepath.Join(projects, claude.ProjectFolder(filepath.Join(r.Src, project)), id+".jsonl")
	paths, err := filepath.Glob(filepath.Join(projects, "*", "*.jsonl"))
	if err != nil {
		return nil, err
	}
	out := []string{""}
	team := TeamOf(id)
	for _, p := range paths {
		if filepath.Base(p) == id+".jsonl" {
			out[0] = p
			continue
		}
		t, err := claude.ReadTranscript(p)
		if err == nil && t.TeamName == team {
			out = append(out, p)
		}
	}
	if out[0] == "" {
		out[0] = lead
	}
	return out, nil
}

func signature(files []string) string {
	var b strings.Builder
	for _, p := range files {
		if info, err := os.Stat(p); err == nil {
			fmt.Fprintf(&b, "%s:%d:%d;", p, info.Size(), info.ModTime().UnixNano())
		}
	}
	return b.String()
}

// price is what the transcripts have cost at list prices, at the high
// end where a cache write's lifetime is unknown. A transcript nothing
// prices fails the run rather than be counted as free.
func price(files []string) (float64, error) {
	var total float64
	for _, p := range files {
		t, err := claude.ReadTranscript(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return 0, err
		}
		u := t.Usage
		if u.Input+u.Output+u.CacheRead+u.CacheCreate == 0 {
			continue
		}
		c := stats.Prices.Price(stats.Priced{Model: modelOf(t), At: t.LastAt, Counters: stats.Counters{
			Input: u.Input, Output: u.Output, WriteUnsplit: u.CacheCreate, Read: u.CacheRead, Messages: u.Messages}})
		if c.Unpriced > 0 {
			return 0, fmt.Errorf("%s: model %q has no price, so the budget cannot be held", p, modelOf(t))
		}
		total += c.High
	}
	return total, nil
}

func modelOf(t claude.Transcript) string {
	if t.ModelID != "" {
		return t.ModelID
	}
	return t.Model
}

// leadEnded is whether the lead's newest turn is a reply that ended:
// Claude Code is waiting on the user, not on a tool or a teammate.
func leadEnded(path string) bool {
	lines, err := readJSONL(path)
	if err != nil {
		return false
	}
	for i := len(lines) - 1; i >= 0; i-- {
		var rec struct {
			Type    string `json:"type"`
			Message struct {
				StopReason string `json:"stop_reason"`
			} `json:"message"`
		}
		if json.Unmarshal(lines[i], &rec) != nil {
			continue
		}
		switch rec.Type {
		case "assistant":
			return rec.Message.StopReason == "end_turn"
		case "user":
			return false
		}
	}
	return false
}

// exportTeam exports the lead as any session is exported, and each
// teammate, with its subagents, under the project its cwd is in.
func (r Recorder) exportTeam(project, id string, files []string) error {
	if err := r.export(project, id); err != nil {
		return err
	}
	for _, p := range files[1:] {
		t, err := claude.ReadTranscript(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(r.Src, t.CWD)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("%s: teammate works in %q, outside %s", p, t.CWD, r.Src)
		}
		mate := strings.TrimSuffix(filepath.Base(p), ".jsonl")
		into := filepath.Join(r.Corpus, strings.Split(rel, string(filepath.Separator))[0])
		if err := exportFile(p, filepath.Join(into, mate+".jsonl")); err != nil {
			return err
		}
		subs, _ := filepath.Glob(filepath.Join(filepath.Dir(p), mate, "subagents", "*"))
		for _, s := range subs {
			if err := exportFile(s, filepath.Join(into, mate, "subagents", filepath.Base(s))); err != nil {
				return err
			}
		}
	}
	return nil
}
