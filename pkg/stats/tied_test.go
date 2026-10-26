package stats_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/stats"
)

const scratch = "/tmp/tmp.item285"

var tiedToLib = map[string]string{sessionA: lib}

func tiedOnly(roots ...string) stats.Filter {
	return stats.Filter{Root: city.ProjectRoot, Tied: tiedToLib, Only: roots}
}

func tiedReply(session, id, at string, output int64) string {
	return reply{session: session, id: id, at: at, cwd: scratch, usage: usage{Output: output}}.line()
}

// seedTied puts a session tied to lib and an untied one in the same
// temporary directory, and a message a fork copied into both.
func (f *fixture) seedTied(untied bool) {
	f.t.Helper()
	f.transcript(sessionA,
		tiedReply(sessionA, "msg_tied", "2026-10-05T14:00:00.000Z", 1),
		tiedReply(sessionA, "msg_fork", "2026-10-05T15:00:00.000Z", 4),
	)
	if untied {
		f.transcript(sessionB,
			tiedReply(sessionB, "msg_fork", "2026-10-05T15:00:00.000Z", 4),
			tiedReply(sessionB, "msg_untied", "2026-10-05T16:00:00.000Z", 2),
		)
	}
	f.ingest()
}

func TestTiedSessions(t *testing.T) {
	day := date(2026, 10, 5)
	views := []struct {
		name   string
		output func(f *fixture, filter stats.Filter) int64
	}{
		{"Series", func(f *fixture, filter stats.Filter) int64 { return f.day(day, filter).Counters.Output }},
		{"Totals", func(f *fixture, filter stats.Filter) int64 {
			c, _, err := f.store.Totals(stats.MonthToDate(today), filter, ny)
			require.NoError(f.t, err)
			return c.Output
		}},
		{"ModelMix", func(f *fixture, filter stats.Filter) int64 {
			shares, err := f.store.ModelMix(stats.MonthToDate(today), filter, ny)
			require.NoError(f.t, err)
			var sum int64
			for _, s := range shares {
				sum += s.Counters.Output
			}
			return sum
		}},
	}

	for _, v := range views {
		t.Run("when "+v.name+" is asked of a temporary directory holding a tied and an untied session", func(t *testing.T) {
			f := newFixture(t)
			f.seedTied(true)

			t.Run("and the filter picks the tied session's project", func(t *testing.T) {
				t.Run("it should count the tied session's messages, the fork's once", func(t *testing.T) {
					assert.Equal(t, int64(5), v.output(f, tiedOnly(lib)))
				})
			})

			t.Run("and the filter picks Temporary", func(t *testing.T) {
				t.Run("it should count only the untied session's own messages", func(t *testing.T) {
					assert.Equal(t, int64(2), v.output(f, tiedOnly(city.TemporaryRoot)))
				})
			})

			t.Run("and the filter picks every district", func(t *testing.T) {
				t.Run("it should count every message once", func(t *testing.T) {
					assert.Equal(t, int64(7), v.output(f, tiedOnly()))
				})
			})

			t.Run("and no session is tied", func(t *testing.T) {
				t.Run("it should count them all under Temporary", func(t *testing.T) {
					assert.Equal(t, int64(7), v.output(f, only(city.TemporaryRoot)))
				})
			})
		})
	}

	t.Run("when Coverage is asked for a project only a tied session spent in", func(t *testing.T) {
		f := newFixture(t)
		f.seedTied(false)

		t.Run("it should have data", func(t *testing.T) {
			assert.True(t, f.coverage(stats.MonthToDate(today), tiedOnly(lib)).HasData)
		})
	})

	t.Run("when Coverage is asked for Temporary and its only session is tied", func(t *testing.T) {
		f := newFixture(t)
		f.seedTied(false)

		t.Run("it should have no data", func(t *testing.T) {
			assert.False(t, f.coverage(stats.MonthToDate(today), tiedOnly(city.TemporaryRoot)).HasData)
		})
	})

	districts := []struct {
		name   string
		untied bool
		want   []string
	}{
		{"when every temporary session is tied, Districts should list the project and not Temporary", false, []string{lib}},
		{"when an untied session is left, Districts should list Temporary and the project", true, []string{city.TemporaryRoot, lib}},
	}
	for _, c := range districts {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.seedTied(c.untied)
			got, err := f.store.Districts(city.ProjectRoot, tiedToLib)
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestTiedWindow(t *testing.T) {
	at := time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC)
	want := map[string]int64{lib: 5, city.TemporaryRoot: 2}
	paths := []struct {
		name   string
		window func(t *testing.T) stats.Window
	}{
		{"read from the store", func(t *testing.T) stats.Window {
			f := newFixture(t)
			f.seedTied(true)
			w, err := f.store.Window(at.Add(-24*time.Hour), at, 24, city.ProjectRoot, tiedToLib)
			require.NoError(t, err)
			return w
		}},
		{"answered by the ledger's live window", func(t *testing.T) stats.Window {
			f := newFixture(t)
			path := filepath.Join(t.TempDir(), "stats.db")
			f.store = open(t, path)
			f.seedTied(true)
			w, err := newLedger(t, path).Window(at, 24, city.ProjectRoot, tiedToLib)
			require.NoError(t, err)
			return w
		}},
	}
	for _, p := range paths {
		t.Run("when a window is "+p.name+" and a tied and an untied session spent in it", func(t *testing.T) {
			t.Run("it should count the tied session's spend under its project", func(t *testing.T) {
				assert.Equal(t, want, outputs(p.window(t).ByProject))
			})
		})
	}

	t.Run("when the ledger answered a window and the ties then change, the store unchanged", func(t *testing.T) {
		f := newFixture(t)
		path := filepath.Join(t.TempDir(), "stats.db")
		f.store = open(t, path)
		f.seedTied(true)
		l := newLedger(t, path)
		_, err := l.Window(at, 24, city.ProjectRoot, nil)
		require.NoError(t, err)
		w, err := l.Window(at.Add(time.Second), 24, city.ProjectRoot, tiedToLib)
		require.NoError(t, err)

		t.Run("it should answer with the new ties", func(t *testing.T) {
			assert.Equal(t, want, outputs(w.ByProject))
		})
	})
}
