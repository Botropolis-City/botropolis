package demo

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strconv"
	"time"
)

const statsDate = "2006-01-02"

type statsLine struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Message   struct {
		Model string `json:"model"`
		Usage struct {
			Input       int64 `json:"input_tokens"`
			Output      int64 `json:"output_tokens"`
			CacheRead   int64 `json:"cache_read_input_tokens"`
			CacheCreate int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

type statsDay struct {
	Date      string `json:"date"`
	Messages  int    `json:"messageCount"`
	Sessions  int    `json:"sessionCount"`
	ToolCalls int    `json:"toolCallCount"`
}

type statsDayTokens struct {
	Date          string           `json:"date"`
	TokensByModel map[string]int64 `json:"tokensByModel"`
}

type statsModelUsage struct {
	Input       int64 `json:"inputTokens"`
	Output      int64 `json:"outputTokens"`
	CacheRead   int64 `json:"cacheReadInputTokens"`
	CacheCreate int64 `json:"cacheCreationInputTokens"`
}

type statsCache struct {
	Version          int                        `json:"version"`
	LastComputed     string                     `json:"lastComputedDate"`
	DailyActivity    []statsDay                 `json:"dailyActivity"`
	DailyModelTokens []statsDayTokens           `json:"dailyModelTokens"`
	ModelUsage       map[string]statsModelUsage `json:"modelUsage"`
	TotalSessions    int                        `json:"totalSessions"`
	TotalMessages    int                        `json:"totalMessages"`
	FirstSessionAt   string                     `json:"firstSessionDate"`
	HourCounts       map[string]int             `json:"hourCounts"`
}

// writeStats writes the home's stats-cache.json, as Claude Code computes
// it, from the transcripts staged, so City Hall stands on what this city
// itself did and nothing made up (item 362).
func writeStats(dir string, transcripts []string, now time.Time) error {
	loc := now.Location()
	out := statsCache{Version: 1, LastComputed: now.Format(statsDate), ModelUsage: map[string]statsModelUsage{}, HourCounts: map[string]int{}}
	days := map[string]*statsDay{}
	tokens := map[string]map[string]int64{}
	day := func(date string) *statsDay {
		if days[date] == nil {
			days[date] = &statsDay{Date: date}
		}
		return days[date]
	}
	var first time.Time
	for _, path := range transcripts {
		lines, err := readJSONL(path)
		if err != nil {
			return err
		}
		var opened time.Time
		for _, raw := range lines {
			var l statsLine
			if json.Unmarshal(raw, &l) != nil || l.Timestamp.IsZero() || (l.Type != "user" && l.Type != "assistant") {
				continue
			}
			at := l.Timestamp.In(loc)
			if opened.IsZero() || at.Before(opened) {
				opened = at
			}
			date := at.Format(statsDate)
			day(date).Messages++
			out.TotalMessages++
			out.HourCounts[strconv.Itoa(at.Hour())]++
			if m := l.Message.Model; m != "" {
				u := l.Message.Usage
				mu := out.ModelUsage[m]
				mu.Input += u.Input
				mu.Output += u.Output
				mu.CacheRead += u.CacheRead
				mu.CacheCreate += u.CacheCreate
				out.ModelUsage[m] = mu
				if tokens[date] == nil {
					tokens[date] = map[string]int64{}
				}
				tokens[date][m] += u.Input + u.Output
			}
		}
		if opened.IsZero() {
			continue
		}
		out.TotalSessions++
		day(opened.Format(statsDate)).Sessions++
		if first.IsZero() || opened.Before(first) {
			first = opened
		}
	}
	if !first.IsZero() {
		out.FirstSessionAt = first.UTC().Format(time.RFC3339Nano)
	}
	for _, date := range sortedKeys(days) {
		out.DailyActivity = append(out.DailyActivity, *days[date])
	}
	for _, date := range sortedKeys(tokens) {
		out.DailyModelTokens = append(out.DailyModelTokens, statsDayTokens{Date: date, TokensByModel: tokens[date]})
	}
	return writeJSON(filepath.Join(dir, ".claude", "stats-cache.json"), out)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
