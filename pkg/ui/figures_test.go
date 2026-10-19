package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/pkg/ui"
)

func TestHeadline(t *testing.T) {
	cases := []struct {
		name     string
		cost     stats.Cost
		messages int
		want     string
	}{
		{"when the cost is known, it should give dollars at list prices and the messages",
			stats.Cost{Low: 12.341, High: 12.341}, 7, "$12.34 at API list prices · 7 messages"},
		{"when there is one message, it should say message",
			stats.Cost{Low: 1, High: 1}, 1, "$1.00 at API list prices · 1 message"},
		{"when the cost is a range, it should give both ends",
			stats.Cost{Low: 5, High: 8}, 1, "$5.00–$8.00 at API list prices · 1 message"},
		{"when the cost is under a cent, it should not round it to nothing",
			stats.Cost{Low: 0.003, High: 0.003}, 1, "<$0.01 at API list prices · 1 message"},
		{"when some messages are unpriced, it should say how many",
			stats.Cost{Low: 2, High: 2, Unpriced: 2}, 5, "$2.00 at API list prices · 5 messages · cost unknown for 2"},
		{"when every message is unpriced, it should give no dollars",
			stats.Cost{Unpriced: 2}, 2, "cost unknown · 2 messages"},
		{"when there are no messages, it should say so",
			stats.Cost{}, 0, "no messages"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, ui.Headline(c.cost, c.messages))
		})
	}
}

func TestCountersLine(t *testing.T) {
	cases := []struct {
		name string
		c    stats.Counters
		want string
	}{
		{"when every write's lifetime is known, it should give the five counters apart",
			stats.Counters{Input: 11, Write5m: 3, Write1h: 30_000, Read: 4_400_000, Output: 22},
			"input 11 · 5 m writes 3 · 1 h writes 30k · reads 4.4M · output 22"},
		{"when a counter is zero, it should say 0",
			stats.Counters{Output: 5},
			"input 0 · 5 m writes 0 · 1 h writes 0 · reads 0 · output 5"},
		{"when some writes have an unknown lifetime, it should give them apart too",
			stats.Counters{WriteUnsplit: 50},
			"input 0 · 5 m writes 0 · 1 h writes 0 · writes of unknown lifetime 50 · reads 0 · output 0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, ui.CountersLine(c.c))
		})
	}
}

func TestPausedLine(t *testing.T) {
	t.Run("when statistics are paused, it should say why", func(t *testing.T) {
		assert.Equal(t, "statistics paused: the store is being upgraded", ui.PausedLine("the store is being upgraded"))
	})

	t.Run("when they are not, it should be empty", func(t *testing.T) {
		assert.Equal(t, "", ui.PausedLine(""))
	})
}
