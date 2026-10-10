package city_test

import (
	"os"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 49. Three boats on the river, one per limit the plan actually
// has. A limit that does not exist gets no boat.

func subscription(t *testing.T) claude.Utilization {
	t.Helper()
	data, err := os.ReadFile("../claude/testdata/utilization-subscription.json")
	require.NoError(t, err)
	u, ok := claude.ParseUtilization(data)
	require.True(t, ok)
	return u
}

func TestGauges(t *testing.T) {
	g := city.Gauges(subscription(t))

	t.Run("when a subscription reports its three limits", func(t *testing.T) {
		t.Run("it should float one boat for each", func(t *testing.T) {
			assert.Len(t, g, 3)
		})

		t.Run("it should make the week across all models the big one", func(t *testing.T) {
			assert.Equal(t, city.GaugeBig, g[0].Size)
		})

		t.Run("it should read that week's percentage", func(t *testing.T) {
			assert.Equal(t, 21.0, g[0].Percent)
		})

		t.Run("it should name the per-model week by its model", func(t *testing.T) {
			assert.Equal(t, "this week, Fable", g[2].Title)
		})

		t.Run("it should keep a boat at zero, because zero is a reading", func(t *testing.T) {
			assert.Zero(t, g[2].Percent)
		})

		t.Run("it should say so on the card rather than showing a dash", func(t *testing.T) {
			assert.Contains(t, city.GaugeCard(g[2], 0).Lines[0], "0%")
		})
	})

	// Bug 52 moved the separation from along the river to across it.
	// The berth used to be a phase down the run, which is now the
	// reading itself, so two boats are held apart by lane instead.
	t.Run("when two of the boats share a window", func(t *testing.T) {
		t.Run("it should still lane them apart, so they never stack", func(t *testing.T) {
			assert.NotEqual(t, g[0].Lane, g[2].Lane)
		})

		t.Run("it should keep every pair apart", func(t *testing.T) {
			seen := map[int]bool{}
			for _, b := range g {
				require.False(t, seen[b.Lane], "two boats in lane %v", b.Lane)
				seen[b.Lane] = true
			}
		})
	})

	t.Run("when the plan reports no limits at all", func(t *testing.T) {
		t.Run("it should float no boats rather than three at zero", func(t *testing.T) {
			assert.Empty(t, city.Gauges(claude.Utilization{}))
		})
	})

	t.Run("when the plan caps monthly spend instead", func(t *testing.T) {
		data, err := os.ReadFile("../claude/testdata/utilization-spend.json")
		require.NoError(t, err)
		u, ok := claude.ParseUtilization(data)
		require.True(t, ok)
		boats := city.Gauges(u)

		t.Run("it should float one boat, because the plan has one limit", func(t *testing.T) {
			assert.Len(t, boats, 1)
		})

		t.Run("it should read the account's own percentage", func(t *testing.T) {
			assert.Equal(t, 97.0, boats[0].Percent)
		})

		t.Run("it should say the money on the card", func(t *testing.T) {
			assert.Contains(t, city.GaugeCard(boats[0], 0).Lines[1], "484")
		})
	})
}

func TestGaugeCard(t *testing.T) {
	g := city.Gauges(subscription(t))[0]
	card := city.GaugeCard(g, 40*time.Minute)

	t.Run("when a boat is pointed at", func(t *testing.T) {
		t.Run("it should name the window it measures", func(t *testing.T) {
			assert.Contains(t, card.Title, "this week, every model")
		})

		t.Run("it should say how old the reading is, because it is cached", func(t *testing.T) {
			assert.Contains(t, card.Lines[2], "ago")
		})

		t.Run("it should say how to get a fresh one, naming the key that does it", func(t *testing.T) {
			assert.Contains(t, card.Lines[2], city.RefreshKey+" to refresh")
		})

		t.Run("it should print the reading as a percentage", func(t *testing.T) {
			assert.Contains(t, card.Lines[0], "21%")
		})
	})

	// The window this reads is relative to now, so it cannot be asserted from a
	// fixture: the testdata's resets_at fell into the past and turned "in 2 days"
	// into "any moment", failing a test that had nothing wrong with it. Both
	// branches are built here instead, from a utilization whose reset is placed
	// relative to the clock rather than written down.
	resettingIn := func(d time.Duration) city.Card {
		u := claude.Utilization{Limits: []claude.Limit{{
			Kind: "weekly_all", Group: "weekly", Percent: 21, ResetsAt: time.Now().Add(d),
		}}}
		return city.GaugeCard(city.Gauges(u)[0], time.Minute)
	}

	t.Run("when the window it measures has not turned over yet", func(t *testing.T) {
		card := resettingIn(48 * time.Hour)

		t.Run("it should say how long there is left", func(t *testing.T) {
			assert.Contains(t, card.Lines[1], "resets   in ")
		})
	})

	t.Run("when the window it measures has already turned over", func(t *testing.T) {
		card := resettingIn(-time.Hour)

		t.Run("it should say so rather than counting down from zero", func(t *testing.T) {
			assert.Contains(t, card.Lines[1], "resets   any moment")
		})
	})
}
