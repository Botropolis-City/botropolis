package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/pkg/ui"
)

func usd(v float64) stats.Cost { return stats.Cost{Low: v, High: v} }

func msgs(n int) stats.Counters { return stats.Counters{Output: int64(100 * n), Messages: n} }

func someBreakdown() city.Breakdown {
	return city.Breakdown{
		Window: city.LastDay, Counters: stats.Counters{Input: 10, Write1h: 20, Read: 300, Output: 40, Messages: 4}, Cost: usd(0.35), Priced: true,
		ByModel:   []city.Share{{Key: "claude-opus-5", Counters: msgs(3), Cost: usd(0.3)}, {Key: "claude-sonnet-5", Counters: msgs(1), Cost: usd(0.05)}},
		ByProject: []city.Share{{Key: "cinders", Counters: msgs(4), Cost: usd(0.35)}},
		BySession: []city.Share{{Key: "a", Counters: msgs(3), Cost: usd(0.3)}, {Key: "b", Counters: msgs(1), Cost: usd(0.05)}},
	}
}

func unpricedBreakdown() city.Breakdown {
	return city.Breakdown{
		Window: city.LastDay, Counters: stats.Counters{Output: 40, WriteUnsplit: 20, Messages: 4}, Cost: stats.Cost{Unpriced: 4},
		Reason:    "statistics are off",
		ByProject: []city.Share{{Key: "cinders", Counters: msgs(4), Cost: stats.Cost{Unpriced: 4}}},
		BySession: []city.Share{{Key: "a", Counters: msgs(3), Cost: stats.Cost{Unpriced: 3}}, {Key: "b", Counters: msgs(1), Cost: stats.Cost{Unpriced: 1}}},
	}
}

func TestLayoutBreakdown(t *testing.T) {
	th := ui.NewTheme(1)
	series := []float64{0, 1, 2, 3}

	t.Run("when the panel is laid out", func(t *testing.T) {
		p := ui.LayoutBreakdown(th, 900, 700, someBreakdown(), series, map[string]string{"a": "Fix the CI queue"}, measure7)

		t.Run("it should centre the panel", func(t *testing.T) {
			assert.Equal(t, city.Point{X: 450, Y: 350}, p.Rect.Center())
		})

		t.Run("it should title it with the window's cost and messages in the one wording", func(t *testing.T) {
			assert.Equal(t, "Power: "+stats.Headline(usd(0.35), 4)+" in 24 h", p.Title.Text)
		})

		t.Run("it should give the counters apart", func(t *testing.T) {
			assert.Equal(t, stats.CountersLine(someBreakdown().Counters), p.Counters.Text)
		})

		t.Run("it should put the counters under the sparkline", func(t *testing.T) {
			assert.Greater(t, p.Counters.At.Y, p.Spark.Box.Max.Y)
		})

		t.Run("it should put the counters above the columns", func(t *testing.T) {
			assert.Less(t, p.Counters.At.Y, p.Columns[0].Head.At.Y)
		})

		t.Run("it should leave no note while priced", func(t *testing.T) {
			assert.Equal(t, "", p.Note.Text)
		})

		t.Run("it should offer the three windows as buttons with the current one marked", func(t *testing.T) {
			require.Len(t, p.Windows, 3)
			assert.Equal(t, "24 h", p.Windows[1].Label.Text)
			assert.Equal(t, city.LastDay, p.Current)
		})

		t.Run("it should lay the series as a sparkline under the title", func(t *testing.T) {
			require.Len(t, p.Spark.Points, 4)
			assert.Greater(t, p.Spark.Box.Min.Y, p.Title.At.Y)
		})

		t.Run("it should lay three columns headed by model, project and session", func(t *testing.T) {
			require.Len(t, p.Columns, 3)
			assert.Equal(t, []string{"by model", "by project", "by session"}, []string{p.Columns[0].Head.Text, p.Columns[1].Head.Text, p.Columns[2].Head.Text})
		})

		t.Run("it should list each share with its cost", func(t *testing.T) {
			require.Len(t, p.Columns[0].Rows, 2)
			assert.Equal(t, []string{"claude-opus-5", "$0.30"}, []string{p.Columns[0].Rows[0].Label.Text, p.Columns[0].Rows[0].Value.Text})
		})

		t.Run("and statistics cannot be read", func(t *testing.T) {
			p := ui.LayoutBreakdown(th, 900, 700, unpricedBreakdown(), series, nil, measure7)

			t.Run("it should title it with no dollars", func(t *testing.T) {
				assert.Equal(t, "Power: cost unknown · 4 messages in 24 h", p.Title.Text)
			})

			t.Run("it should say cost and the split by model need statistics", func(t *testing.T) {
				assert.Equal(t, "cost and by model need statistics", p.Note.Text)
			})

			t.Run("it should still head the model column", func(t *testing.T) {
				assert.Equal(t, "by model", p.Columns[0].Head.Text)
			})

			t.Run("it should leave the model column empty", func(t *testing.T) {
				assert.Empty(t, p.Columns[0].Rows)
			})

			t.Run("it should list each other share with its messages", func(t *testing.T) {
				assert.Equal(t, "3 messages", p.Columns[2].Rows[0].Value.Text)
			})
		})

		t.Run("it should name a session by its title", func(t *testing.T) {
			assert.Equal(t, "Fix the CI queue", p.Columns[2].Rows[0].Label.Text)
		})

		t.Run("it should fall back to the id for an unknown session", func(t *testing.T) {
			assert.Equal(t, "b", p.Columns[2].Rows[1].Label.Text)
		})

		t.Run("it should stack the columns side by side", func(t *testing.T) {
			assert.Greater(t, p.Columns[1].Head.At.X, p.Columns[0].Head.At.X)
			assert.Equal(t, p.Columns[0].Head.At.Y, p.Columns[1].Head.At.Y)
		})
	})

	t.Run("when a window button is hit", func(t *testing.T) {
		p := ui.LayoutBreakdown(th, 900, 700, someBreakdown(), series, nil, measure7)

		t.Run("it should say which window", func(t *testing.T) {
			w, ok := p.HitWindow(p.Windows[2].Rect.Center())
			require.True(t, ok)
			assert.Equal(t, city.LastWeek, w)
		})

		t.Run("it should miss beside the buttons", func(t *testing.T) {
			_, ok := p.HitWindow(city.Point{X: 1, Y: 1})
			assert.False(t, ok)
		})
	})
}

func TestCardSparkline(t *testing.T) {
	th := ui.NewTheme(1)

	t.Run("when a card carries a series", func(t *testing.T) {
		card := city.Card{Title: "t", Lines: []string{"one"}, Series: []float64{1, 2, 3}}
		plain := ui.LayoutCard(th, city.Card{Title: "t", Lines: []string{"one"}}, city.RectAt(0, 0, 800, 600), measure7)
		c := ui.LayoutCard(th, card, city.RectAt(0, 0, 800, 600), measure7)

		t.Run("it should lay a sparkline under the lines", func(t *testing.T) {
			require.Len(t, c.Spark.Points, 3)
			assert.Greater(t, c.Spark.Box.Min.Y, c.Lines[0].At.Y)
		})

		t.Run("it should grow to hold it", func(t *testing.T) {
			assert.Equal(t, plain.Rect.Height()+8+24, c.Rect.Height())
		})

		t.Run("it should span the card's inner width", func(t *testing.T) {
			assert.Equal(t, c.Rect.Width()-2*16, c.Spark.Box.Width())
		})
	})
}
