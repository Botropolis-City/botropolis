package demo_test

import (
	"math"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/demo"
	"botropolis.city/botropolis/pkg/state"
)

const (
	scenariosDir = "../../../demo/scenarios"
	corpusDir    = "../../../demo/corpus"
)

// stagedGroups stages a scenario from the real corpus, loads it as the
// daemon would, and names each group it detects by its members' titles.
func stagedGroups(t *testing.T, scenario demo.Scenario) []string {
	t.Helper()
	home := t.TempDir()
	now := time.Now()
	_, err := demo.NewDirector(demo.Corpus{Dir: corpusDir}, scenario, home, now, demo.Sleepers(home, 10*time.Minute))
	t.Cleanup(func() { _ = demo.Unstage(home) })
	require.NoError(t, err)
	snapshot, err := state.NewLoader(home, probes).WithGroups(filepath.Join(t.TempDir(), "groups.json")).Load(now)
	require.NoError(t, err)
	titles := map[string]string{}
	for _, s := range snapshot.Sessions {
		titles[s.ID] = s.Title
	}
	var out []string
	for _, g := range snapshot.Groups {
		var members []string
		for _, id := range g.Members {
			members = append(members, titles[id])
		}
		out = append(out, groupKey(members))
	}
	slices.Sort(out)
	return out
}

func groupKey(titles []string) string {
	titles = slices.Clone(titles)
	slices.Sort(titles)
	return strings.Join(titles, " + ")
}

func TestStagedScenariosFormTheGroupsTheyExpect(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(scenariosDir, "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	for _, path := range paths {
		scenario, err := demo.LoadScenario(path)
		require.NoError(t, err)

		t.Run("when the "+scenario.Name+" scenario is staged and loaded", func(t *testing.T) {
			var want []string
			for _, g := range scenario.Groups {
				want = append(want, groupKey(g))
			}
			slices.Sort(want)

			t.Run("it should form exactly the groups it lists", func(t *testing.T) {
				assert.Equal(t, want, stagedGroups(t, scenario))
			})
		})
	}
}

// topBarRates is what the city's top bar sums: each live session's
// fresh, cached and output tokens an hour.
func topBarRates(t *testing.T, scenario demo.Scenario) (fresh, cached, output float64) {
	t.Helper()
	home := t.TempDir()
	now := time.Now()
	_, err := demo.NewDirector(demo.Corpus{Dir: corpusDir}, scenario, home, now, demo.Sleepers(home, 10*time.Minute))
	t.Cleanup(func() { _ = demo.Unstage(home) })
	require.NoError(t, err)
	snapshot, err := state.NewLoader(home, probes).Load(now)
	require.NoError(t, err)
	for _, s := range snapshot.Sessions {
		if state.Live(s.State) {
			fresh += s.FreshTokensPerHour
			cached += s.CacheReadPerHour
			output += s.OutputPerHour
		}
	}
	return fresh, cached, output
}

// A heavy user's top bar, in round figures (item 240): each staged rate
// should land within half to twice its target.
func TestStagedMetropolisReadsAsAHeavyUser(t *testing.T) {
	scenario, err := demo.LoadScenario(filepath.Join(scenariosDir, "metropolis.yaml"))
	require.NoError(t, err)

	t.Run("when the metropolis is staged and loaded", func(t *testing.T) {
		fresh, cached, output := topBarRates(t, scenario)

		for _, tc := range []struct {
			name   string
			got    float64
			target float64
		}{
			{"burn fresh tokens within half to twice 5M/h", fresh, 5e6},
			{"read cached tokens within half to twice 100M/h", cached, 100e6},
			{"write output within half to twice 500k/h", output, 500e3},
		} {
			t.Run("it should "+tc.name, func(t *testing.T) {
				assert.InDelta(t, 0, math.Log2(tc.got/tc.target), 1, "%.0f/h against %.0f/h", tc.got, tc.target)
			})
		}
	})
}

func TestStagedMetropolisKeepsItsTeam(t *testing.T) {
	scenario, err := demo.LoadScenario(filepath.Join(scenariosDir, "metropolis.yaml"))
	require.NoError(t, err)

	t.Run("when the metropolis is staged and loaded", func(t *testing.T) {
		groups := stagedGroups(t, scenario)

		t.Run("it should put the harbor team's three sessions in one group", func(t *testing.T) {
			assert.Contains(t, groups, groupKey([]string{"Driftwood ingest", "Driftwood schema", "Saltmarsh API"}))
		})
	})
}
