package city_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 285: a temporary-directory session tied to a project counts its
// spend where its building stands, so each district's figures are the
// sum of the buildings drawn in it.
func TestTiedSpendFollowsTheBuilding(t *testing.T) {
	const (
		kiln    = "/home/user/src/kiln"
		scratch = "/tmp/tmp.Kq81bZ"
		loose   = "/var/tmp/run-7"
		project = "00000285-0000-4000-8000-000000000001"
		tied    = "00000285-0000-4000-8000-000000000002"
		untied  = "00000285-0000-4000-8000-000000000003"
	)
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	ts := now.Add(-time.Hour).Format(time.RFC3339Nano)
	home := helpers.NewHome(t)
	home.Transcript(project, kiln, helpers.UserPrompt(project, kiln, ts), helpers.AssistantReply(project, "msg_285_1", ts))
	home.Transcript(tied, scratch, helpers.UserPrompt(tied, scratch, ts),
		helpers.AssistantFileEdit(tied, "msg_285_2", ts, "toolu_285_2", kiln+"/glaze.go"),
		helpers.AssistantReply(tied, "msg_285_3", ts))
	home.Transcript(untied, loose, helpers.UserPrompt(untied, loose, ts), helpers.AssistantReply(untied, "msg_285_4", ts))

	probes := state.Probes{Alive: func(int) bool { return false }, Attached: func(string) bool { return false }}
	snapshot, err := state.NewLoader(home.Path, probes).Load(now)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "stats.db")
	store, err := stats.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.Ingest(filepath.Join(home.ClaudeDir(), "projects"), now, nil))
	ledger := stats.NewLedger(path)
	t.Cleanup(ledger.Close)
	c := city.BuildWithLedger(snapshot, city.NewLayout(), ledger)
	require.True(t, c.Plant.Day.Priced, c.Plant.Day.Reason)

	drawn := map[string]float64{}
	for _, d := range c.Districts {
		for _, b := range d.Buildings {
			drawn[d.Root] += b.Cost.High
		}
	}
	names := map[string]string{kiln: "kiln", city.TemporaryRoot: city.TemporaryName}
	views := []struct {
		name string
		cost func(root string) float64
	}{
		{"the 24 h breakdown", func(root string) float64 {
			for _, share := range c.Plant.Day.ByProject {
				if share.Key == names[root] {
					return share.Cost.High
				}
			}
			return 0
		}},
		{"the long ranges", func(root string) float64 {
			today := stats.DateOf(now)
			_, cost, err := store.Totals(stats.Range{First: today, Last: today},
				stats.Filter{Root: city.ProjectRoot, Tied: c.Ties(), Only: []string{root}}, time.UTC)
			require.NoError(t, err)
			return cost.High
		}},
	}
	for _, v := range views {
		for _, root := range []string{kiln, city.TemporaryRoot} {
			t.Run("when "+v.name+" counts "+names[root]+", it should cost what the buildings drawn in it cost", func(t *testing.T) {
				require.NotZero(t, drawn[root])
				assert.InDelta(t, drawn[root], v.cost(root), 1e-9)
			})
		}
	}
}
