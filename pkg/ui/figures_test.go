package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/pkg/ui"
)

func TestFiguresWording(t *testing.T) {
	cost := stats.Cost{Low: 5, High: 8, Unpriced: 1}
	counters := stats.Counters{Input: 1, Write1h: 2, Read: 3, Output: 4}

	t.Run("it should word a headline as pkg/stats does", func(t *testing.T) {
		assert.Equal(t, stats.Headline(cost, 3), ui.Headline(cost, 3))
	})

	t.Run("it should word the counters as pkg/stats does", func(t *testing.T) {
		assert.Equal(t, stats.CountersLine(counters), ui.CountersLine(counters))
	})

	t.Run("it should word a pause as pkg/stats does", func(t *testing.T) {
		assert.Equal(t, stats.PausedLine("x"), ui.PausedLine("x"))
	})
}
