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

// TeamClock is what a team recording reads the time from and waits on
// between its polls (item 254).
type TeamClock interface {
	Now() time.Time
	Wait(ctx context.Context, d time.Duration) error
}

// realTeamClock is the wall clock.
type realTeamClock struct{}

func (realTeamClock) Now() time.Time { return time.Now() }

func (realTeamClock) Wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

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
	entry := Prompted{Project: project, Label: s.Label, Image: r.Image}
	limits := r.Team
	if limits == (TeamLimits{}) {
		limits = DefaultTeamLimits
	}
	clock := r.TeamClock
	if clock == nil {
		clock = realTeamClock{}
	}
	start := clock.Now()
	// since allows for a filesystem whose times are coarser than the clock.
	since := start.Add(-2 * time.Second)
	run, err := r.Terminal(ctx, filepath.Join(r.Src, project), teamArgs(s, id, s.Prompts[0], r.MCPConfig), []string{teamsEnv})
	if err != nil {
		return entry, err
	}
	defer func() { _ = run.Stop() }()
	changed, last := start, ""
	for {
		if err := clock.Wait(ctx, limits.Poll); err != nil {
			return entry, fmt.Errorf("interrupted, so the run was stopped: %w", err)
		}
		written, err := r.writtenSince(since)
		if err != nil {
			return entry, err
		}
		files := r.transcripts(project, id, written)
		now := clock.Now()
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
			return entry, r.exportRun(project, id, written)
		case now.Sub(start) >= limits.Time:
			return entry, fmt.Errorf("ran out of time after %s, so the run was stopped", limits.Time)
		}
	}
}

// writtenSince is every file under the home's projects written since the
// run started. Sessions are recorded one at a time, so they are all this
// run's, wherever Claude Code filed them and whatever it named their
// team: an in-process teammate is not filed the way a tmux one is
// (item 105).
func (r Recorder) writtenSince(since time.Time) ([]string, error) {
	var out []string
	root := filepath.Join(r.Home, ".claude", "projects")
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return filepath.SkipDir
		}
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.ModTime().Before(since) {
			out = append(out, p)
		}
		return nil
	})
	return out, err
}

// transcripts is the lead's transcript first, whether or not it has been
// written yet, then every other transcript among the files.
func (r Recorder) transcripts(project, id string, files []string) []string {
	lead := filepath.Join(r.Home, ".claude", "projects", claude.ProjectFolder(filepath.Join(r.Src, project)), id+".jsonl")
	out := []string{lead}
	for _, p := range files {
		if p != lead && filepath.Ext(p) == ".jsonl" {
			out = append(out, p)
		}
	}
	return out
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

// exportRun exports every file the run wrote, through the same scrub,
// into the corpus folder of the toy project whose folder Claude Code
// filed it under, keeping its path within that folder. A folder is a toy
// project's when it is the lead's or a transcript in it works in one; a
// file under any other folder is refused, since nothing outside the toy
// projects may reach the corpus.
func (r Recorder) exportRun(project, id string, files []string) error {
	folders := map[string]string{claude.ProjectFolder(filepath.Join(r.Src, project)): project}
	for _, p := range files {
		if filepath.Ext(p) != ".jsonl" {
			continue
		}
		t, err := claude.ReadTranscript(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(r.Src, t.CWD)
		if err != nil || t.CWD == "" || rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}
		name, _, _ := strings.Cut(rel, string(filepath.Separator))
		folders[claude.ProjectFolder(filepath.Join(r.Src, name))] = name
	}
	root := filepath.Join(r.Home, ".claude", "projects")
	for _, p := range files {
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		folder, inside, _ := strings.Cut(rel, string(filepath.Separator))
		into, ok := folders[folder]
		if !ok || inside == "" {
			return fmt.Errorf("%s: written outside every toy project's folder, so it is not exported", rel)
		}
		if err := exportFile(p, filepath.Join(r.Corpus, into, inside)); err != nil {
			return err
		}
	}
	return nil
}
