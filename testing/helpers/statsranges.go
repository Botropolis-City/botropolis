package helpers

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/stats"
)

// StatsRangeDistricts are the districts StatsRanges fills, by name.
var StatsRangeDistricts = []string{"kiln", "lanternfish", "tidepool"}

// StatsRangesFirst is the first day StatsRanges holds data for, and the
// gap is the days it records statistics as off.
var (
	StatsRangesFirst = time.Date(2026, time.July, 31, 0, 0, 0, 0, time.Local)
	StatsRangesGap   = [2]time.Time{time.Date(2026, time.September, 14, 0, 0, 0, 0, time.Local), time.Date(2026, time.September, 17, 0, 0, 0, 0, time.Local)}
)

type rangeModel struct {
	model, speed string
	weight       int
}

var rangeModels = []rangeModel{
	{"claude-opus-5", "standard", 50}, {"claude-sonnet-5", "standard", 28}, {"claude-haiku-5-5", "standard", 10},
	{"claude-opus-5", "fast", 6}, {"claude-sonnet-5-5", "standard", 5}, {"claude-sonnet-4", "", 1},
}

// StatsRanges writes a statistics store under home's state directory,
// as a --home city reads it, from synthetic transcripts: the three
// districts' messages every day from StatsRangesFirst to now, from a
// fixed seed, with a gap recorded over StatsRangesGap. It returns the
// store's path. For tests and frames of the plant's long ranges; item 116.
func StatsRanges(t testing.TB, home string, now time.Time) string {
	t.Helper()
	path := filepath.Join(home, ".local", "state", "botropolis", "stats.db")
	projects := filepath.Join(t.TempDir(), "projects")
	rng := rand.New(rand.NewSource(116))
	for i, name := range StatsRangeDistricts {
		cwd := filepath.Join("/home/user/work", name)
		session := fmt.Sprintf("11600000-0000-4000-8000-%012d", i)
		var lines []string
		n := 0
		for day := StatsRangesFirst; day.Before(now); day = day.AddDate(0, 0, 1) {
			if day.Weekday() == time.Sunday && rng.Intn(3) > 0 {
				continue
			}
			for range rng.Intn(4+2*(2-i)) + 1 {
				n++
				at := day.Add(time.Duration(9*60+rng.Intn(10*60)) * time.Minute)
				if at.After(now) {
					continue
				}
				m := pickModel(rng)
				lines = append(lines, rangeLine(session, fmt.Sprintf("msg_%d_%d", i, n), at, cwd, m, rng))
			}
		}
		dir := filepath.Join(projects, strings.ReplaceAll(cwd, "/", "-"))
		require.NoError(t, os.MkdirAll(dir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, session+".jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o600))
	}
	store, err := stats.Open(path)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()
	require.NoError(t, store.Ingest(projects, now, nil))
	_, err = store.DB().Exec("INSERT INTO gaps (off_at, on_at) VALUES (?, ?)", StatsRangesGap[0].UnixMilli(), StatsRangesGap[1].UnixMilli())
	require.NoError(t, err)
	return path
}

func pickModel(rng *rand.Rand) rangeModel {
	n := rng.Intn(100)
	for _, m := range rangeModels {
		if n < m.weight {
			return m
		}
		n -= m.weight
	}
	return rangeModels[0]
}

func rangeLine(session, id string, at time.Time, cwd string, m rangeModel, rng *rand.Rand) string {
	write5m, write1h := int64(rng.Intn(20_000)), int64(rng.Intn(4_000))
	usage := map[string]any{
		"input_tokens": rng.Intn(3_000) + 10, "output_tokens": rng.Intn(4_000) + 50,
		"cache_creation_input_tokens": write5m + write1h, "cache_read_input_tokens": rng.Intn(300_000) + 1_000,
		"cache_creation": map[string]any{"ephemeral_5m_input_tokens": write5m, "ephemeral_1h_input_tokens": write1h},
	}
	if rng.Intn(15) == 0 {
		delete(usage, "cache_creation")
	}
	if m.speed != "" {
		usage["speed"] = m.speed
	}
	data, _ := json.Marshal(map[string]any{
		"type": "assistant", "timestamp": at.UTC().Format(time.RFC3339Nano), "sessionId": session, "cwd": cwd,
		"message": map[string]any{"id": id, "model": m.model, "role": "assistant",
			"content": []any{map[string]any{"type": "text", "text": "done"}}, "usage": usage},
	})
	return string(data)
}
