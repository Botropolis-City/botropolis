package stats_test

import (
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/stats"
)

// churn is a store taking a random sequence of ingests: new messages,
// streamed messages growing, forks copying a session's messages, a
// transcript rewritten shorter, sessions forgotten, and the clock
// moving past messages.
type churn struct {
	f        *fixture
	rng      *rand.Rand
	at       time.Time
	lines    map[string][]reply
	sessions []string
	next     int
	made     int
}

var (
	churnModels = []string{"claude-opus-5-5", "claude-haiku-4-5", "claude-sonnet-4-5", "mystery-model"}
	churnDirs   = []string{lib, libWorktree, "/w/app", "/w/app/sub"}
)

func newChurn(t *testing.T, seed uint64, at time.Time) *churn {
	f := newFixture(t)
	return &churn{f: f, rng: rand.New(rand.NewPCG(seed, seed*7+1)), at: at, lines: map[string][]reply{}}
}

func (c *churn) reply(session string) reply {
	c.next++
	u := usage{Input: c.rng.Int64N(5000), Output: 1 + c.rng.Int64N(3000), CacheRead: c.rng.Int64N(50000)}
	switch c.rng.IntN(4) {
	case 0:
		u.CacheCreate = 1 + c.rng.Int64N(9000)
	case 1:
		u.CacheCreate = 1 + c.rng.Int64N(9000)
		u.Creation = &split{FiveMinutes: u.CacheCreate / 2, OneHour: u.CacheCreate - u.CacheCreate/2}
	case 2:
		u.Input = stats.LongPrompt + c.rng.Int64N(1000)
	}
	if c.rng.IntN(5) == 0 {
		u.Tools = &tools{WebSearch: c.rng.Int64N(3), WebFetch: c.rng.Int64N(3)}
	}
	if c.rng.IntN(6) == 0 {
		u.Speed = "fast"
	}
	at := c.at.Add(-time.Duration(c.rng.Int64N(int64(30 * time.Hour)))).Add(time.Duration(c.rng.Int64N(int64(20 * time.Minute))))
	return reply{session: session, id: fmt.Sprintf("msg_%d", c.next), at: at.Format("2006-01-02T15:04:05.000Z"),
		model: churnModels[c.rng.IntN(len(churnModels))], cwd: churnDirs[c.rng.IntN(len(churnDirs))], usage: u}
}

func (c *churn) write(session string) {
	lines := make([]string, len(c.lines[session]))
	for i, r := range c.lines[session] {
		r.session = session
		lines[i] = r.line()
	}
	c.f.transcript(session, lines...)
}

func (c *churn) newSession() string {
	c.made++
	id := fmt.Sprintf("cccccccc-0000-0000-0000-%012d", c.made)
	c.sessions = append(c.sessions, id)
	return id
}

func (c *churn) pick() (string, bool) {
	if len(c.sessions) == 0 {
		return "", false
	}
	return c.sessions[c.rng.IntN(len(c.sessions))], true
}

// step makes one random change and ingests it.
func (c *churn) step() {
	s, ok := c.pick()
	switch kind := c.rng.IntN(10); {
	case !ok || kind < 4:
		if !ok || c.rng.IntN(4) == 0 {
			s = c.newSession()
		}
		c.lines[s] = append(c.lines[s], c.reply(s))
	case kind < 6:
		last := c.lines[s][len(c.lines[s])-1]
		last.usage.Output += 1 + c.rng.Int64N(500)
		c.lines[s] = append(c.lines[s], last)
	case kind == 6:
		fork := c.newSession()
		c.lines[fork] = append(append([]reply(nil), c.lines[s]...), c.reply(fork))
		s = fork
	case kind == 7 && len(c.lines[s]) > 1:
		c.lines[s] = c.lines[s][:len(c.lines[s])-1]
	case kind == 8:
		require.NoError(c.f.t, os.Remove(c.f.transcriptPath(s)))
		require.NoError(c.f.t, c.f.store.Forget(s))
		delete(c.lines, s)
		for i, id := range c.sessions {
			if id == s {
				c.sessions = append(c.sessions[:i], c.sessions[i+1:]...)
				break
			}
		}
		return
	default:
		c.at = c.at.Add(time.Duration(c.rng.Int64N(int64(3 * time.Hour))))
		return
	}
	c.write(s)
	c.f.ingest()
}

func near(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}

func nearCost(a, b stats.Cost) bool {
	return near(a.Low, b.Low) && near(a.High, b.High) && a.Unpriced == b.Unpriced
}

func sliceDiff(name string, got, want []stats.Slice) string {
	g := map[string]stats.Slice{}
	for _, s := range got {
		g[s.Key] = s
	}
	if len(got) != len(want) {
		return fmt.Sprintf("%s: %d slices, want %d", name, len(got), len(want))
	}
	for _, w := range want {
		s, ok := g[w.Key]
		if !ok || s.Counters != w.Counters || !nearCost(s.Cost, w.Cost) {
			return fmt.Sprintf("%s[%s]: %+v, want %+v", name, w.Key, s, w)
		}
	}
	return ""
}

// windowDiff is how got differs from want, or "": counters exactly,
// costs to a relative 1e-9.
func windowDiff(got, want stats.Window) string {
	if got.Counters != want.Counters {
		return fmt.Sprintf("counters %+v, want %+v", got.Counters, want.Counters)
	}
	if !nearCost(got.Cost, want.Cost) {
		return fmt.Sprintf("cost %+v, want %+v", got.Cost, want.Cost)
	}
	for _, d := range []string{
		sliceDiff("by model", got.ByModel, want.ByModel),
		sliceDiff("by project", got.ByProject, want.ByProject),
		sliceDiff("by session", got.BySession, want.BySession),
	} {
		if d != "" {
			return d
		}
	}
	if len(got.Hourly) != len(want.Hourly) {
		return fmt.Sprintf("%d hours, want %d", len(got.Hourly), len(want.Hourly))
	}
	for i := range want.Hourly {
		if !nearCost(got.Hourly[i], want.Hourly[i]) {
			return fmt.Sprintf("hour %d: %+v, want %+v", i, got.Hourly[i], want.Hourly[i])
		}
	}
	return ""
}

func costsDiff(got, want map[string]stats.Cost) string {
	if len(got) != len(want) {
		return fmt.Sprintf("%d sessions, want %d", len(got), len(want))
	}
	for id, w := range want {
		if g, ok := got[id]; !ok || !nearCost(g, w) {
			return fmt.Sprintf("session %s: %+v, want %+v", id, g, w)
		}
	}
	return ""
}

// Item 215: a ledger brings the window and the sessions' costs it read
// up to date from the store's change log, and the store's own queries,
// which read everything again, are the oracle.
func TestLedgerIncremental(t *testing.T) {
	t0 := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

	for seed := uint64(1); seed <= 50; seed++ {
		hours := []int{1, 24, 168}[seed%3]
		t.Run(fmt.Sprintf("when a ledger has read a %d h window and session costs, and the store then takes 40 random ingests, seed %d", hours, seed), func(t *testing.T) {
			t.Parallel()
			c := newChurn(t, seed, t0)
			for range 5 {
				c.step()
			}
			l := newLedger(t, c.f.db)
			_, err := l.WindowNow(c.at, hours, city.ProjectRoot)
			require.NoError(t, err)
			_, err = l.SessionCostsNow(c.at)
			require.NoError(t, err)
			var windows, costs string
			for i := 0; i < 40 && windows == "" && costs == ""; i++ {
				c.step()
				got, err := l.WindowNow(c.at, hours, city.ProjectRoot)
				require.NoError(t, err)
				want, err := c.f.store.Window(c.at.Add(-time.Duration(hours)*time.Hour), c.at, hours, city.ProjectRoot)
				require.NoError(t, err)
				if d := windowDiff(got, want); d != "" {
					windows = fmt.Sprintf("after step %d: %s", i+1, d)
				}
				gotCosts, err := l.SessionCostsNow(c.at)
				require.NoError(t, err)
				wantCosts, err := c.f.store.SessionCosts()
				require.NoError(t, err)
				if d := costsDiff(gotCosts, wantCosts); d != "" {
					costs = fmt.Sprintf("after step %d: %s", i+1, d)
				}
			}

			t.Run("its window should equal Store.Window's after every step", func(t *testing.T) {
				assert.Empty(t, windows)
			})

			t.Run("its session costs should equal Store.SessionCosts' after every step", func(t *testing.T) {
				assert.Empty(t, costs)
			})
		})
	}

	t.Run("when the store has no change log, as an older writer leaves it", func(t *testing.T) {
		c := newChurn(t, 99, t0)
		for range 5 {
			c.step()
		}
		for _, q := range []string{
			"DROP TRIGGER changed_on_insert", "DROP TRIGGER changed_on_update", "DROP TRIGGER changed_on_delete",
			"DROP TRIGGER changed_on_link", "DROP TRIGGER changed_on_unlink",
			"DROP TABLE changed_messages", "DROP TABLE changed_sessions", "DELETE FROM meta WHERE key = 'change_seq'",
		} {
			_, err := c.f.store.DB().Exec(q)
			require.NoError(t, err)
		}
		l := newLedger(t, c.f.db)
		_, err := l.WindowNow(c.at, 24, city.ProjectRoot)
		require.NoError(t, err)
		for range 10 {
			c.step()
		}
		got, err := l.WindowNow(c.at, 24, city.ProjectRoot)
		require.NoError(t, err)
		want, err := c.f.store.Window(c.at.Add(-24*time.Hour), c.at, 24, city.ProjectRoot)
		require.NoError(t, err)

		t.Run("it should read the window whole and equal Store.Window", func(t *testing.T) {
			assert.Empty(t, windowDiff(got, want))
		})
	})

	t.Run("when three sessions were priced and one of them then changes", func(t *testing.T) {
		c := newChurn(t, 7, t0)
		for _, s := range []string{c.newSession(), c.newSession(), c.newSession()} {
			c.lines[s] = []reply{c.reply(s)}
			c.write(s)
		}
		c.f.ingest()
		l := newLedger(t, c.f.db)
		_, err := l.SessionCostsNow(c.at)
		require.NoError(t, err)
		before := l.SessionsPriced()
		s := c.sessions[1]
		c.lines[s] = append(c.lines[s], c.reply(s))
		c.write(s)
		c.f.ingest()
		_, err = l.SessionCostsNow(c.at)
		require.NoError(t, err)

		t.Run("it should price only that session again", func(t *testing.T) {
			assert.Equal(t, 1, l.SessionsPriced()-before)
		})
	})

	t.Run("when a window was read and nothing changed since", func(t *testing.T) {
		c := newChurn(t, 8, t0)
		for range 5 {
			c.step()
		}
		l := newLedger(t, c.f.db)
		_, err := l.WindowNow(c.at, 24, city.ProjectRoot)
		require.NoError(t, err)
		before := l.MessageReads()
		_, err = l.WindowNow(c.at.Add(time.Minute), 24, city.ProjectRoot)
		require.NoError(t, err)

		t.Run("it should read no messages again", func(t *testing.T) {
			assert.Equal(t, 0, l.MessageReads()-before)
		})
	})
}
