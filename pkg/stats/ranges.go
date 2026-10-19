package stats

import "time"

// Date is a local calendar day, with no time of day and no zone: the
// ranges are arithmetic on these, and only a query turns one into
// instants, in the zone it is given. Item 116.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// DateOf is the calendar day t falls on, in t's own location.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

// Start is the instant the day begins in loc.
func (d Date) Start(loc *time.Location) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, loc)
}

// AddDays is the day n days after d, or before it when n is negative.
func (d Date) AddDays(n int) Date {
	return DateOf(time.Date(d.Year, d.Month, d.Day+n, 12, 0, 0, 0, time.UTC))
}

// Before reports whether d is an earlier day than e.
func (d Date) Before(e Date) bool {
	if d.Year != e.Year {
		return d.Year < e.Year
	}
	if d.Month != e.Month {
		return d.Month < e.Month
	}
	return d.Day < e.Day
}

// Range is the days from First to Last, both included.
type Range struct{ First, Last Date }

// Comparison is month to date beside last month.
type Comparison struct{ This, Last Range }

func clamped(year int, month time.Month, day int) Date {
	first := DateOf(time.Date(year, month, 1, 12, 0, 0, 0, time.UTC))
	last := daysIn(first.Year, first.Month)
	if day > last {
		day = last
	}
	return Date{Year: first.Year, Month: first.Month, Day: day}
}

func daysIn(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 12, 0, 0, 0, time.UTC).Day()
}

func monthsBack(today Date, n int) Date {
	return clamped(today.Year, today.Month-time.Month(n), today.Day)
}

// MonthToDate is the 1st of this month to today.
func MonthToDate(today Date) Range {
	return Range{First: Date{Year: today.Year, Month: today.Month, Day: 1}, Last: today}
}

// LastMonth is the previous calendar month in full.
func LastMonth(today Date) Range {
	first := clamped(today.Year, today.Month-1, 1)
	return Range{First: first, Last: clamped(first.Year, first.Month, 31)}
}

// Last30Days is today and the 29 days before it.
func Last30Days(today Date) Range { return Range{First: today.AddDays(-29), Last: today} }

// Last3Months starts on the same date three months ago, clamped to the
// end of a shorter month, and runs to today.
func Last3Months(today Date) Range { return Range{First: monthsBack(today, 3), Last: today} }

// Last12Months starts on the same date a year ago, clamped, and runs to
// today.
func Last12Months(today Date) Range { return Range{First: monthsBack(today, 12), Last: today} }

// YearToDate is 1 January to today.
func YearToDate(today Date) Range {
	return Range{First: Date{Year: today.Year, Month: time.January, Day: 1}, Last: today}
}

// Custom is the days between a and b, whichever order they come in.
func Custom(a, b Date) Range {
	if b.Before(a) {
		a, b = b, a
	}
	return Range{First: a, Last: b}
}

// MonthComparison sets month to date beside last month; sameDay stops
// last month on today's day of the month, clamped to its end.
func MonthComparison(today Date, sameDay bool) Comparison {
	last := LastMonth(today)
	if sameDay {
		last.Last = clamped(last.First.Year, last.First.Month, today.Day)
	}
	return Comparison{This: MonthToDate(today), Last: last}
}
