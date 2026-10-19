package stats_test

import (
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/stats"
)

const (
	app         = "/w/app"
	libWorktree = lib + "/.claude/worktrees/item-1"
)

var ny = func() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(err)
	}
	return loc
}()

var everywhere = stats.Filter{Root: city.ProjectRoot}

func only(roots ...string) stats.Filter { return stats.Filter{Root: city.ProjectRoot, Only: roots} }

func msg(id, at, cwd string, u usage) string {
	return reply{session: sessionA, id: id, at: at, cwd: cwd, usage: u}.line()
}

func modelMsg(id, at, model string, u usage) string {
	return reply{session: sessionA, id: id, at: at, cwd: lib, model: model, usage: u}.line()
}

func (f *fixture) seed(lines ...string) {
	f.t.Helper()
	f.transcript(sessionA, lines...)
	f.ingest()
}

func (f *fixture) gap(off, on string) {
	f.t.Helper()
	_, err := f.store.DB().Exec("INSERT INTO gaps (off_at, on_at) VALUES (?, ?)", ms(f.t, off), ms(f.t, on))
	require.NoError(f.t, err)
}

func (f *fixture) series(r stats.Range, filter stats.Filter, unit stats.Unit) []stats.Bucket {
	f.t.Helper()
	buckets, err := f.store.Series(r, filter, unit, ny)
	require.NoError(f.t, err)
	return buckets
}

func (f *fixture) day(d stats.Date, filter stats.Filter) stats.Bucket {
	f.t.Helper()
	buckets := f.series(stats.Range{First: d.AddDays(-1), Last: d.AddDays(1)}, filter, stats.Days)
	require.Len(f.t, buckets, 3)
	return buckets[1]
}

func (f *fixture) coverage(r stats.Range, filter stats.Filter) stats.Coverage {
	f.t.Helper()
	c, err := f.store.Coverage(r, filter, ny)
	require.NoError(f.t, err)
	return c
}

func (f *fixture) mix(r stats.Range) []stats.Share {
	f.t.Helper()
	shares, err := f.store.ModelMix(r, everywhere, ny)
	require.NoError(f.t, err)
	return shares
}

func TestSeries(t *testing.T) {
	t.Run("when a message is late on a local day but early the next UTC day", func(t *testing.T) {
		f := newFixture(t)
		f.seed(msg("msg_1", "2026-10-11T02:30:00.000Z", lib, usage{Output: 5}))

		t.Run("it should count on its local day", func(t *testing.T) {
			assert.Equal(t, 1, f.day(date(2026, 10, 10), everywhere).Counters.Messages)
		})
	})

	dst := []struct {
		name, at string
		day      stats.Date
	}{
		{"when a day is 23 hours long, as daylight saving starts", "2026-03-09T03:30:00.000Z", date(2026, 3, 8)},
		{"when a day is 25 hours long, as daylight saving ends", "2026-11-02T04:30:00.000Z", date(2026, 11, 1)},
	}
	for _, c := range dst {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.seed(msg("msg_0", "2026-01-01T12:00:00.000Z", lib, usage{}), msg("msg_1", c.at, lib, usage{Output: 5}))

			t.Run("it should count a message at 23:30 local on that day", func(t *testing.T) {
				assert.Equal(t, 1, f.day(c.day, everywhere).Counters.Messages)
			})
		})
	}

	t.Run("when the earliest message is on 5 October", func(t *testing.T) {
		f := newFixture(t)
		f.seed(
			msg("msg_1", "2026-10-05T14:00:00.000Z", lib, usage{Output: 5}),
			msg("msg_2", "2026-10-09T14:00:00.000Z", lib, usage{Output: 5}),
		)

		t.Run("and a day after it has no messages", func(t *testing.T) {
			b := f.day(date(2026, 10, 7), everywhere)

			t.Run("it should be zero", func(t *testing.T) {
				assert.Equal(t, 0, b.Counters.Messages)
			})

			t.Run("it should not be no data", func(t *testing.T) {
				assert.False(t, b.NoData)
			})
		})

		t.Run("and a day is before it", func(t *testing.T) {
			t.Run("it should be no data", func(t *testing.T) {
				assert.True(t, f.day(date(2026, 10, 3), everywhere).NoData)
			})
		})

		t.Run("and the day is the earliest", func(t *testing.T) {
			t.Run("it should not be no data", func(t *testing.T) {
				assert.False(t, f.day(date(2026, 10, 5), everywhere).NoData)
			})
		})
	})

	t.Run("when two messages fall on one day", func(t *testing.T) {
		f := newFixture(t)
		f.seed(
			msg("msg_1", "2026-10-05T14:00:00.000Z", lib, usage{Input: 1, Output: 2, CacheCreate: 3, CacheRead: 4}),
			msg("msg_2", "2026-10-05T15:00:00.000Z", lib, usage{Input: 10, Output: 20, CacheCreate: 30, CacheRead: 40}),
		)

		t.Run("it should sum the four counters and the messages", func(t *testing.T) {
			assert.Equal(t, stats.Counters{Input: 11, Output: 22, WriteUnsplit: 33, Read: 44, Messages: 2}, f.day(date(2026, 10, 5), everywhere).Counters)
		})
	})

	t.Run("when a fork holds a message twice", func(t *testing.T) {
		f := newFixture(t)
		f.transcript(sessionA, msg("msg_1", "2026-10-05T14:00:00.000Z", lib, usage{Output: 5}))
		f.transcript(sessionB, reply{session: sessionB, id: "msg_1", at: "2026-10-05T14:00:00.000Z", cwd: lib, usage: usage{Output: 5}}.line())
		f.ingest()

		t.Run("it should count it once", func(t *testing.T) {
			assert.Equal(t, int64(5), f.day(date(2026, 10, 5), everywhere).Counters.Output)
		})
	})

	t.Run("when bucketing by month", func(t *testing.T) {
		f := newFixture(t)
		f.seed(
			msg("msg_1", "2026-09-05T14:00:00.000Z", lib, usage{Output: 1}),
			msg("msg_2", "2026-09-30T23:00:00.000Z", lib, usage{Output: 2}),
			msg("msg_3", "2026-10-01T14:00:00.000Z", lib, usage{Output: 4}),
		)
		buckets := f.series(stats.Range{First: date(2026, 9, 1), Last: today}, everywhere, stats.Months)

		t.Run("it should give one bucket per local month", func(t *testing.T) {
			assert.Equal(t, []stats.Date{date(2026, 9, 1), date(2026, 10, 1)}, []stats.Date{buckets[0].Start, buckets[1].Start})
		})

		t.Run("it should sum each local month", func(t *testing.T) {
			assert.Equal(t, [2]int64{3, 4}, [2]int64{buckets[0].Counters.Output, buckets[1].Counters.Output})
		})
	})

	t.Run("when a gap was recorded across 5 October", func(t *testing.T) {
		f := newFixture(t)
		f.seed(msg("msg_1", "2026-10-01T14:00:00.000Z", lib, usage{Output: 1}))
		f.gap("2026-10-05T16:00:00.000Z", "2026-10-05T20:00:00.000Z")

		t.Run("and a day overlaps it", func(t *testing.T) {
			t.Run("it should be incomplete", func(t *testing.T) {
				assert.True(t, f.day(date(2026, 10, 5), everywhere).Incomplete)
			})
		})

		t.Run("and a day is clear of it", func(t *testing.T) {
			t.Run("it should not be incomplete", func(t *testing.T) {
				assert.False(t, f.day(date(2026, 10, 8), everywhere).Incomplete)
			})
		})
	})
}

func TestFilter(t *testing.T) {
	f := newFixture(t)
	f.seed(
		msg("msg_1", "2026-10-03T14:00:00.000Z", app, usage{Output: 1}),
		msg("msg_2", "2026-10-05T14:00:00.000Z", lib, usage{Output: 2}),
		msg("msg_3", "2026-10-05T16:00:00.000Z", app, usage{Output: 8}),
	)

	t.Run("when no districts are chosen", func(t *testing.T) {
		t.Run("it should count every district", func(t *testing.T) {
			assert.Equal(t, int64(10), f.day(date(2026, 10, 5), everywhere).Counters.Output)
		})
	})

	t.Run("when one district is chosen", func(t *testing.T) {
		t.Run("it should count only its messages", func(t *testing.T) {
			assert.Equal(t, int64(2), f.day(date(2026, 10, 5), only(lib)).Counters.Output)
		})

		t.Run("it should take its earliest from its own first message", func(t *testing.T) {
			assert.Equal(t, date(2026, 10, 5), f.coverage(stats.MonthToDate(today), only(lib)).Earliest)
		})
	})

	t.Run("when a message is in a worktree of the chosen district", func(t *testing.T) {
		w := newFixture(t)
		w.seed(msg("msg_1", "2026-10-05T15:00:00.000Z", libWorktree, usage{Output: 4}))

		t.Run("it should count under that district", func(t *testing.T) {
			assert.Equal(t, int64(4), w.day(date(2026, 10, 5), only(lib)).Counters.Output)
		})
	})
}

func TestModelMix(t *testing.T) {
	t.Run("when two models answered", func(t *testing.T) {
		f := newFixture(t)
		f.seed(
			modelMsg("msg_1", "2026-10-05T14:00:00.000Z", "claude-opus-5", usage{Output: 1, Speed: "standard"}),
			modelMsg("msg_2", "2026-10-05T15:00:00.000Z", "claude-sonnet-5", usage{Output: 2, Speed: "standard"}),
			modelMsg("msg_3", "2026-10-06T15:00:00.000Z", "claude-sonnet-5", usage{Output: 4, Speed: "standard"}),
		)

		t.Run("it should split the counters by model", func(t *testing.T) {
			outputs := map[string]int64{}
			for _, s := range f.mix(stats.MonthToDate(today)) {
				outputs[s.Model] = s.Counters.Output
			}
			assert.Equal(t, map[string]int64{"claude-opus-5": 1, "claude-sonnet-5": 6}, outputs)
		})

		t.Run("it should order the models by cost, dearest first", func(t *testing.T) {
			assert.Equal(t, []string{"claude-sonnet-5", "claude-opus-5"}, models(f.mix(stats.MonthToDate(today))))
		})
	})

	t.Run("when one model answered at two speeds", func(t *testing.T) {
		f := newFixture(t)
		f.seed(
			modelMsg("msg_1", "2026-10-05T14:00:00.000Z", "claude-opus-5", usage{Output: 1, Speed: "fast"}),
			modelMsg("msg_2", "2026-10-05T15:00:00.000Z", "claude-opus-5", usage{Output: 2, Speed: "standard"}),
		)

		t.Run("it should split by speed", func(t *testing.T) {
			assert.Equal(t, []string{"fast", "standard"}, speeds(f.mix(stats.MonthToDate(today))))
		})
	})

	t.Run("when a message has no speed recorded", func(t *testing.T) {
		f := newFixture(t)
		f.seed(modelMsg("msg_1", "2026-10-05T14:00:00.000Z", "claude-opus-5", usage{Output: 1}))

		t.Run("it should report it as unrecorded", func(t *testing.T) {
			assert.Equal(t, []string{stats.Unrecorded}, speeds(f.mix(stats.MonthToDate(today))))
		})
	})

	t.Run("when messages fall before and after the range", func(t *testing.T) {
		f := newFixture(t)
		f.seed(
			modelMsg("msg_1", "2026-09-30T14:00:00.000Z", "claude-haiku-5", usage{Output: 1}),
			modelMsg("msg_2", "2026-10-05T14:00:00.000Z", "claude-opus-5", usage{Output: 1}),
			modelMsg("msg_3", "2026-10-11T14:00:00.000Z", "claude-sonnet-5", usage{Output: 1}),
		)

		t.Run("it should leave them out", func(t *testing.T) {
			assert.Equal(t, []string{"claude-opus-5"}, models(f.mix(stats.Custom(date(2026, 10, 1), date(2026, 10, 10)))))
		})
	})
}

func TestCoverage(t *testing.T) {
	t.Run("when the earliest message is on 5 October", func(t *testing.T) {
		f := newFixture(t)
		f.seed(msg("msg_1", "2026-10-05T14:00:00.000Z", lib, usage{Output: 1}))

		t.Run("and the range starts before it", func(t *testing.T) {
			t.Run("it should say so", func(t *testing.T) {
				assert.True(t, f.coverage(stats.MonthToDate(today), everywhere).StartsBeforeData)
			})
		})

		t.Run("and the range starts on it", func(t *testing.T) {
			t.Run("it should not", func(t *testing.T) {
				assert.False(t, f.coverage(stats.Custom(date(2026, 10, 5), today), everywhere).StartsBeforeData)
			})
		})

		t.Run("and a comparison's last month is before it", func(t *testing.T) {
			t.Run("its last month's coverage should say so", func(t *testing.T) {
				assert.True(t, f.coverage(stats.MonthComparison(today, true).Last, everywhere).StartsBeforeData)
			})
		})
	})

	t.Run("when a gap was recorded", func(t *testing.T) {
		f := newFixture(t)
		f.seed(msg("msg_1", "2026-09-01T14:00:00.000Z", lib, usage{Output: 1}))
		f.gap("2026-10-05T16:00:00.000Z", "2026-10-07T20:00:00.000Z")

		t.Run("and the range touches it", func(t *testing.T) {
			t.Run("it should say so", func(t *testing.T) {
				assert.True(t, f.coverage(stats.MonthToDate(today), everywhere).TouchesGap)
			})
		})

		t.Run("and the range is clear of it", func(t *testing.T) {
			t.Run("it should not", func(t *testing.T) {
				assert.False(t, f.coverage(stats.Custom(date(2026, 10, 8), today), everywhere).TouchesGap)
			})
		})
	})

	t.Run("when the store is empty", func(t *testing.T) {
		f := newFixture(t)

		t.Run("it should report no data", func(t *testing.T) {
			assert.False(t, f.coverage(stats.MonthToDate(today), everywhere).HasData)
		})

		t.Run("it should mark every day no data", func(t *testing.T) {
			noData := true
			for _, b := range f.series(stats.MonthToDate(today), everywhere, stats.Days) {
				noData = noData && b.NoData
			}
			assert.True(t, noData)
		})
	})
}

func TestDistricts(t *testing.T) {
	t.Run("when messages sit in two projects and a worktree of one", func(t *testing.T) {
		f := newFixture(t)
		f.seed(
			msg("msg_1", "2026-10-05T14:00:00.000Z", lib, usage{}),
			msg("msg_2", "2026-10-05T15:00:00.000Z", libWorktree, usage{}),
			msg("msg_3", "2026-10-05T16:00:00.000Z", app, usage{}),
		)

		t.Run("it should list each project once", func(t *testing.T) {
			districts, err := f.store.Districts(city.ProjectRoot)
			require.NoError(t, err)
			assert.Equal(t, []string{app, lib}, districts)
		})
	})
}

func speeds(shares []stats.Share) []string {
	out := []string{}
	for _, s := range shares {
		out = append(out, s.Speed)
	}
	return out
}

func models(shares []stats.Share) []string {
	out := []string{}
	for _, s := range shares {
		out = append(out, s.Model)
	}
	return out
}

func TestTotals(t *testing.T) {
	t.Run("when a range is totalled", func(t *testing.T) {
		f := newFixture(t)
		f.seed(
			modelMsg("msg_1", "2026-10-05T14:00:00.000Z", "claude-opus-5-5", usage{Input: 1, Output: 2, CacheCreate: 3, CacheRead: 4, Creation: &split{FiveMinutes: 3}}),
			modelMsg("msg_2", "2026-10-06T14:00:00.000Z", "claude-opus-5-5", usage{Input: 10, Output: 20, CacheCreate: 30, CacheRead: 40, Creation: &split{OneHour: 30}}),
		)
		counters, cost := f.totals(stats.MonthToDate(today))

		t.Run("it should sum the five counters apart", func(t *testing.T) {
			assert.Equal(t, stats.Counters{Input: 11, Output: 22, Write5m: 3, Write1h: 30, Read: 44, Messages: 2}, counters)
		})

		t.Run("it should cost the sum of its messages' costs", func(t *testing.T) {
			want := stats.Prices.Price(stats.Priced{Model: "claude-opus-5-5", At: someday, Counters: counters})
			assert.InDelta(t, want.Low, cost.Low, 1e-12)
		})
	})

	t.Run("when one message's model has no price", func(t *testing.T) {
		f := newFixture(t)
		f.seed(
			modelMsg("msg_1", "2026-10-05T14:00:00.000Z", "claude-opus-5-5", usage{Output: million}),
			modelMsg("msg_2", "2026-10-06T14:00:00.000Z", "claude-opus-9-9", usage{Output: million}),
		)
		_, cost := f.totals(stats.MonthToDate(today))

		t.Run("it should count it as unpriced", func(t *testing.T) {
			assert.Equal(t, 1, cost.Unpriced)
		})

		t.Run("it should keep the others' cost", func(t *testing.T) {
			assert.InDelta(t, 20, cost.Low, 1e-9)
		})
	})

	t.Run("when a day is costed", func(t *testing.T) {
		f := newFixture(t)
		f.seed(
			modelMsg("msg_1", "2026-10-05T14:00:00.000Z", "claude-opus-5-5", usage{Output: million}),
			modelMsg("msg_2", "2026-10-05T15:00:00.000Z", "claude-sonnet-5-5", usage{Output: million}),
		)

		t.Run("it should carry every one of its messages' costs", func(t *testing.T) {
			assert.InDelta(t, 30, f.day(date(2026, 10, 5), everywhere).Cost.Low, 1e-9)
		})
	})
}

func (f *fixture) totals(r stats.Range) (stats.Counters, stats.Cost) {
	f.t.Helper()
	counters, cost, err := f.store.Totals(r, everywhere, ny)
	require.NoError(f.t, err)
	return counters, cost
}
