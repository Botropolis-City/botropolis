package demo

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

const statsModel = "claude-opus-4-5"

// newStatsCorpus is one recorded tidepool session: a prompt at 10:00 and
// a reply at 10:10 that used tokens.
func newStatsCorpus(t *testing.T) Corpus {
	t.Helper()
	dir := t.TempDir()
	reply := `{"type":"assistant","sessionId":"` + recordedID + `","cwd":"` + cwd + `","timestamp":"2026-09-01T10:10:00.000Z",` +
		`"message":{"model":"` + statsModel + `","usage":{"input_tokens":10,"output_tokens":100,"cache_read_input_tokens":1000,"cache_creation_input_tokens":50}}}`
	writeFile(t, filepath.Join(dir, "tidepool", recordedID+".jsonl"), line("user", "2026-09-01T10:00:00.000Z")+"\n"+reply+"\n")
	return Corpus{Dir: dir}
}

func TestStageStatistics(t *testing.T) {
	now := time.Date(2026, 9, 10, 18, 0, 0, 0, time.UTC)
	corpus := newStatsCorpus(t)
	statsPath := func(home string) string { return filepath.Join(home, ".claude", "stats-cache.json") }
	sessions := []Placement{
		{Ref: "tidepool/6f1d", State: state.Resting, Ago: Duration(48 * time.Hour)},
		{Ref: "tidepool/6f1d", Project: "kiln", State: state.Working, Ago: Duration(time.Minute)},
		{Project: "beacon", State: state.Empty},
	}

	t.Run("when a scenario sets stats", func(t *testing.T) {
		t.Run("and it is staged", func(t *testing.T) {
			got := stage(t, corpus, Scenario{Stats: true, Sessions: sessions}, now)
			require.NoError(t, got.err)
			stats, err := claude.ReadStats(statsPath(got.home))
			require.NoError(t, err)

			t.Run("it should write the home's stats-cache.json", func(t *testing.T) {
				assert.FileExists(t, statsPath(got.home))
			})

			t.Run("it should count every staged session with a recording", func(t *testing.T) {
				assert.Equal(t, 2, stats.TotalSessions)
			})

			t.Run("it should count their messages", func(t *testing.T) {
				assert.Equal(t, 4, stats.TotalMessages)
			})

			t.Run("it should date the first session at the oldest staged opening", func(t *testing.T) {
				assert.Equal(t, now.Add(-48*time.Hour-10*time.Minute), stats.FirstSessionAt.UTC())
			})

			t.Run("it should be computed on the scenario's day", func(t *testing.T) {
				assert.Equal(t, "2026-09-10", stats.LastComputed)
			})

			t.Run("it should put each day's messages and sessions on the day they were staged", func(t *testing.T) {
				assert.Equal(t, []claude.DailyActivity{
					{Date: "2026-09-08", Messages: 2, Sessions: 1},
					{Date: "2026-09-10", Messages: 2, Sessions: 1},
				}, stats.DailyActivity)
			})

			t.Run("it should count each staged message's hour", func(t *testing.T) {
				var want [24]int
				want[17], want[18] = 3, 1
				assert.Equal(t, want, stats.HourCounts)
			})

			t.Run("it should give each model its staged tokens", func(t *testing.T) {
				assert.Equal(t, claude.Usage{Input: 20, Output: 200, CacheRead: 2000, CacheCreate: 100}, stats.ModelUsage[statsModel].Usage)
			})

			t.Run("it should put each day's tokens on its model", func(t *testing.T) {
				assert.Equal(t, []claude.DailyModelTokens{
					{Date: "2026-09-08", TokensByModel: map[string]int64{statsModel: 110}},
					{Date: "2026-09-10", TokensByModel: map[string]int64{statsModel: 110}},
				}, stats.DailyModelTokens)
			})
		})
	})

	t.Run("when a scenario does not set stats", func(t *testing.T) {
		got := stage(t, corpus, Scenario{Sessions: sessions}, now)
		require.NoError(t, got.err)

		t.Run("it should write no stats-cache.json", func(t *testing.T) {
			assert.NoFileExists(t, statsPath(got.home))
		})
	})
}
