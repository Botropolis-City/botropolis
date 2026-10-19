package stats_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/stats"
)

const corpus = "../../../demo/corpus"

type costState struct {
	Type       string  `json:"type"`
	TotalUSD   float64 `json:"totalCostUSD"`
	ModelUsage map[string]struct {
		CostUSD float64 `json:"costUSD"`
	} `json:"modelUsage"`
}

// recorded is the last cost-state record in each corpus session: Claude
// Code's own total for it, priced at list prices.
func recorded(t *testing.T) map[string]costState {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(corpus, "*", "*.jsonl"))
	require.NoError(t, err)
	out := map[string]costState{}
	for _, path := range paths {
		f, err := os.Open(path)
		require.NoError(t, err)
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 1<<20), 64<<20)
		for scanner.Scan() {
			var rec costState
			if json.Unmarshal(scanner.Bytes(), &rec) == nil && rec.Type == "cost-state" {
				out[strings.TrimSuffix(filepath.Base(path), ".jsonl")] = rec
			}
		}
		require.NoError(t, scanner.Err())
		require.NoError(t, f.Close())
	}
	return out
}

// asClaudeCodePriced is the rates the Claude Code that recorded the
// corpus (2.1.286, 9 to 17 October 2026) priced with: the list prices,
// but without Sonnet 5.5's cache-read cut of 7 October, which it had not
// picked up. The guard holds the arithmetic to Claude Code's own; the
// cut itself is held by the price table's tests.
var asClaudeCodePriced = func() stats.Rates {
	var out stats.Rates
	for _, r := range stats.Prices {
		if r.Model == "claude-sonnet-5-5" && !r.From.IsZero() {
			continue
		}
		out = append(out, r)
	}
	return out
}()

func TestCorpusCostMatchesClaudeCode(t *testing.T) {
	sessions := recorded(t)
	require.NotEmpty(t, sessions, "the corpus holds no cost-state records, so this guard checks nothing")
	store, err := stats.Open(filepath.Join(t.TempDir(), "stats.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.Ingest(corpus, time.Now(), nil))

	for session, want := range sessions {
		total, byModel, err := store.SessionCost(session, asClaudeCodePriced)
		require.NoError(t, err)

		t.Run("when session "+session+" is costed", func(t *testing.T) {
			t.Run("it should match Claude Code's total", func(t *testing.T) {
				assert.InDelta(t, want.TotalUSD, total.High, 1e-7)
			})

			t.Run("it should leave nothing unpriced", func(t *testing.T) {
				assert.Equal(t, 0, total.Unpriced)
			})

			for model, cost := range want.ModelUsage {
				t.Run("it should match Claude Code's cost for "+model, func(t *testing.T) {
					assert.InDelta(t, cost.CostUSD, byModel[model].High, 1e-7)
				})
			}
		})
	}
}
