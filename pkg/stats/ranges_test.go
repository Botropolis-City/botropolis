package stats_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/stats"
)

func date(y, m, d int) stats.Date { return stats.Date{Year: y, Month: time.Month(m), Day: d} }

var today = date(2026, 10, 19)

func TestRanges(t *testing.T) {
	cases := []struct {
		name string
		got  stats.Range
		want stats.Range
	}{
		{"when month to date, it should run from the 1st of this month to today",
			stats.MonthToDate(today), stats.Range{First: date(2026, 10, 1), Last: today}},
		{"when last month, it should be the previous calendar month in full",
			stats.LastMonth(today), stats.Range{First: date(2026, 9, 1), Last: date(2026, 9, 30)}},
		{"when last month, and it is January, it should be December of the year before",
			stats.LastMonth(date(2026, 1, 15)), stats.Range{First: date(2025, 12, 1), Last: date(2025, 12, 31)}},
		{"when last 30 days, it should be today and the 29 days before",
			stats.Last30Days(today), stats.Range{First: date(2026, 9, 20), Last: today}},
		{"when last 3 months, it should start on the same date three months ago",
			stats.Last3Months(today), stats.Range{First: date(2026, 7, 19), Last: today}},
		{"when last 3 months, and that date does not exist, it should clamp to the end of the month",
			stats.Last3Months(date(2026, 5, 31)), stats.Range{First: date(2026, 2, 28), Last: date(2026, 5, 31)}},
		{"when last 3 months, and that date does not exist in a leap year, it should clamp to 29 February",
			stats.Last3Months(date(2028, 5, 31)), stats.Range{First: date(2028, 2, 29), Last: date(2028, 5, 31)}},
		{"when last 3 months, and it is January, it should reach into the year before",
			stats.Last3Months(date(2026, 1, 10)), stats.Range{First: date(2025, 10, 10), Last: date(2026, 1, 10)}},
		{"when last 12 months, it should start on the same date a year ago",
			stats.Last12Months(today), stats.Range{First: date(2025, 10, 19), Last: today}},
		{"when last 12 months, and it is 29 February, it should clamp to 28 February",
			stats.Last12Months(date(2028, 2, 29)), stats.Range{First: date(2027, 2, 28), Last: date(2028, 2, 29)}},
		{"when year to date, it should run from 1 January to today",
			stats.YearToDate(today), stats.Range{First: date(2026, 1, 1), Last: today}},
		{"when custom, it should keep the dates given",
			stats.Custom(date(2026, 3, 1), date(2026, 3, 9)), stats.Range{First: date(2026, 3, 1), Last: date(2026, 3, 9)}},
		{"when custom, and the dates are reversed, it should put them in order",
			stats.Custom(date(2026, 3, 9), date(2026, 3, 1)), stats.Range{First: date(2026, 3, 1), Last: date(2026, 3, 9)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.got)
		})
	}
}

func TestMonthComparison(t *testing.T) {
	cases := []struct {
		name    string
		today   stats.Date
		sameDay bool
		want    stats.Range
	}{
		{"when the toggle is off, it should compare with all of last month",
			today, false, stats.Range{First: date(2026, 9, 1), Last: date(2026, 9, 30)}},
		{"when the toggle is on, it should stop last month on the same day",
			today, true, stats.Range{First: date(2026, 9, 1), Last: date(2026, 9, 19)}},
		{"when the toggle is on, and it is 31 March, it should stop on 28 February",
			date(2026, 3, 31), true, stats.Range{First: date(2026, 2, 1), Last: date(2026, 2, 28)}},
		{"when the toggle is on, and it is 31 March in a leap year, it should stop on 29 February",
			date(2028, 3, 31), true, stats.Range{First: date(2028, 2, 1), Last: date(2028, 2, 29)}},
		{"when the toggle is on, and it is 31 January, it should stop on 31 December",
			date(2026, 1, 31), true, stats.Range{First: date(2025, 12, 1), Last: date(2025, 12, 31)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, stats.MonthComparison(c.today, c.sameDay).Last)
		})
	}

	t.Run("when comparing, it should put month to date on this side", func(t *testing.T) {
		assert.Equal(t, stats.MonthToDate(today), stats.MonthComparison(today, true).This)
	})
}
