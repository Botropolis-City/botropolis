package claude

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"botropolis.city/botropolis/pkg/statedir"
)

// Utilization is Claude Code's own cached view of the account's limits,
// read straight out of ~/.claude.json. Item 49.
//
// This is the whole feature's denominator problem solved: the figures
// are the real limits, already fetched, already structured, and already
// timestamped. The first cut of this shelled out to `claude -p
// "/usage"` and parsed its prose — which worked, but wrote a transcript
// on every run, and the city is built out of transcripts, so polling it
// added a parked session to the city each time it asked. Reading a file
// is passive and costs nothing.
//
// The probe survives for exactly one job: making Claude Code refetch
// when the user asks for fresh figures. That is rare and explicit,
// which is the only place it was ever worth its cost.
type Utilization struct {
	// FetchedAt is when Claude Code last refreshed these figures, so
	// the age can be shown beside them. A cached percentage with no age
	// on it is a number that is right and means nothing.
	FetchedAt time.Time
	// Limits is one entry per gauge the plan has. Nothing here is
	// hardcoded to session-or-week: a plan with a different set
	// generalises for free.
	Limits []Limit
	// Spend is a credit or enterprise plan's monthly cap.
	Spend Spend
}

// Limit is one window's usage.
type Limit struct {
	// Kind and Group name the window: "session", "weekly_all",
	// "weekly_scoped", grouped as "session" or "weekly".
	Kind  string
	Group string
	// Percent is how much of the limit is spent.
	Percent float64
	// Severity is Claude's own judgement of it — "normal", "critical".
	// A better signal than the bare number and free with it.
	Severity string
	// Model is the model a scoped limit applies to, empty when it
	// applies to all of them.
	Model string
	// ResetsAt is when the window turns over.
	ResetsAt time.Time
	// Active is whether this is the limit currently binding.
	Active bool
}

// Spend is a monthly spend cap, in the account's own currency.
type Spend struct {
	Enabled    bool
	Percent    float64
	Severity   string
	UsedMinor  int64
	LimitMinor int64
	Currency   string
	Exponent   int
}

// Used and Limit are the spend in whole currency units.
func (s Spend) Used() float64 { return minor(s.UsedMinor, s.Exponent) }
func (s Spend) Cap() float64  { return minor(s.LimitMinor, s.Exponent) }
func minor(v int64, exp int) float64 {
	f := float64(v)
	for i := 0; i < exp; i++ {
		f /= 10
	}
	return f
}

// Shape is which kind of plan the cache describes.
type Shape int

const (
	// ShapeUnknown is no readable cache at all: the file is missing,
	// malformed, or has no utilization in it. Distinct from a plan that
	// reports no limits, and the distinction matters — one is a fact
	// about the plan and the other a fact about the reading.
	ShapeUnknown Shape = iota
	// ShapeSubscription has per-window percentages.
	ShapeSubscription
	// ShapeSpend has a monthly spend cap instead.
	ShapeSpend
)

// Shape reports which plan shape this is.
func (u Utilization) Shape() Shape {
	switch {
	case len(u.Limits) > 0:
		return ShapeSubscription
	case u.Spend.Enabled:
		return ShapeSpend
	}
	return ShapeUnknown
}

// Age is how long ago Claude Code fetched these figures.
func (u Utilization) Age(now time.Time) time.Duration {
	if u.FetchedAt.IsZero() {
		return 0
	}
	return now.Sub(u.FetchedAt)
}

// the on-disk shape, named so the JSON tags stay out of the model.
type cachedUsage struct {
	Cached struct {
		FetchedAtMs int64 `json:"fetchedAtMs"`
		Utilization struct {
			Limits []struct {
				Kind     string  `json:"kind"`
				Group    string  `json:"group"`
				Percent  float64 `json:"percent"`
				Severity string  `json:"severity"`
				ResetsAt string  `json:"resets_at"`
				Active   bool    `json:"is_active"`
				Scope    *struct {
					Model *struct {
						DisplayName string `json:"display_name"`
					} `json:"model"`
				} `json:"scope"`
			} `json:"limits"`
			Spend *struct {
				Enabled  bool    `json:"enabled"`
				Percent  float64 `json:"percent"`
				Severity string  `json:"severity"`
				Used     *struct {
					AmountMinor int64  `json:"amount_minor"`
					Currency    string `json:"currency"`
					Exponent    int    `json:"exponent"`
				} `json:"used"`
				Limit *struct {
					AmountMinor int64 `json:"amount_minor"`
				} `json:"limit"`
			} `json:"spend"`
		} `json:"utilization"`
	} `json:"cachedUsageUtilization"`
}

// ParseUtilization reads the cache out of ~/.claude.json's bytes.
func ParseUtilization(data []byte) (Utilization, bool) {
	var raw cachedUsage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Utilization{}, false
	}
	c := raw.Cached
	out := Utilization{}
	if c.FetchedAtMs > 0 {
		out.FetchedAt = time.UnixMilli(c.FetchedAtMs)
	}
	for _, l := range c.Utilization.Limits {
		lim := Limit{Kind: l.Kind, Group: l.Group, Percent: l.Percent, Severity: l.Severity, Active: l.Active}
		if l.Scope != nil && l.Scope.Model != nil {
			lim.Model = l.Scope.Model.DisplayName
		}
		if t, err := time.Parse(time.RFC3339, l.ResetsAt); err == nil {
			lim.ResetsAt = t
		}
		out.Limits = append(out.Limits, lim)
	}
	if s := c.Utilization.Spend; s != nil {
		out.Spend = Spend{Enabled: s.Enabled, Percent: s.Percent, Severity: s.Severity}
		if s.Used != nil {
			out.Spend.UsedMinor, out.Spend.Currency, out.Spend.Exponent = s.Used.AmountMinor, s.Used.Currency, s.Used.Exponent
		}
		if s.Limit != nil {
			out.Spend.LimitMinor = s.Limit.AmountMinor
		}
	}
	if len(out.Limits) == 0 && !out.Spend.Enabled && out.FetchedAt.IsZero() {
		return Utilization{}, false
	}
	return out, true
}

// UtilizationPath is where Claude Code keeps the cache.
func UtilizationPath(home string) string {
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return ".claude.json"
		}
		home = h
	}
	return filepath.Join(home, ".claude.json")
}

// ReadUtilization reads the cache off disk. Passive and cheap — no
// subprocess, no transcript, nothing added to the city.
func ReadUtilization(home string) (Utilization, bool) {
	data, err := os.ReadFile(UtilizationPath(home))
	if err != nil {
		return Utilization{}, false
	}
	return ParseUtilization(data)
}

// UsageProbeDir is the working directory the refresh probe runs in.
//
// `claude -p` writes a transcript for every run, and Botropolis builds
// its city out of transcripts, so a probe running in any ordinary
// directory adds a parked session to the city — measured at three runs
// taking the city from 22 parked to 25. Running in a directory of its
// own puts those transcripts in a project of their own, which the
// loader skips.
//
// It stays under the person's own state directory in a private run too,
// because the daemon fences these transcripts by this path; a private
// run starts no probe at all instead (item 111).
func UsageProbeDir() string {
	return filepath.Join(statedir.Default(), "usage-probe")
}

// SummaryDir is the working directory summaries are generated in. It
// sits beside the usage probe's for the same reason: a `claude -p` run
// holds a PID while it runs, and the loader fences out what runs here.
func SummaryDir() string {
	return filepath.Join(filepath.Dir(UsageProbeDir()), "summary")
}

// UsageProbeProject is the project folder Claude Code files the probe's
// transcripts under.
func UsageProbeProject() string { return ProjectFolder(UsageProbeDir()) }

// ProjectFolder is Claude Code's name for a working directory's
// transcript folder: every character that is not a letter, a digit or a
// dash becomes a dash.
func ProjectFolder(dir string) string {
	out := make([]rune, 0, len(dir))
	for _, r := range dir {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			out = append(out, r)
		default:
			out = append(out, '-')
		}
	}
	return string(out)
}

// RefreshUtilization makes Claude Code refetch the figures, by asking
// it for them. It is slow and it writes a transcript, so it belongs
// behind an explicit user action and nowhere else. The caller re-reads
// the file afterwards.
//
// It also clears up after itself. The probe's transcripts are fenced out
// of the city by where they land, but nothing removed them, so they
// accumulated one per refresh forever — see pruneProbeTranscripts for
// why that is worth the few lines despite being 4 KB a time.
func RefreshUtilization(ctx context.Context, bin string) bool {
	if statedir.IsPrivate() {
		return false
	}
	dir := UsageProbeDir()
	statedir.Check(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	if bin == "" {
		bin = "claude"
	}
	cmd := exec.CommandContext(ctx, bin, "-p", "/usage")
	cmd.Dir = dir
	ok := cmd.Run() == nil
	// After the run, so the transcript this probe just wrote is the one
	// that survives, and best-effort: a probe that fetched the figures
	// has done its job whether or not the tidying worked.
	if d := probeTranscriptDir(); d != "" {
		_ = pruneProbeTranscripts(d, probeKeep)
	}
	return ok
}
