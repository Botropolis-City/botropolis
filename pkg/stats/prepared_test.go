package stats_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/stats"
)

// countingDriver is the store's own driver, with every SQL text it is
// handed to compile counted. It offers no Execer or Queryer, so
// database/sql hands it every statement through Prepare.
type countingDriver struct {
	inner    driver.Driver
	prepared atomic.Int64
}

func (d *countingDriver) Open(name string) (driver.Conn, error) {
	c, err := d.inner.Open(name)
	if err != nil {
		return nil, err
	}
	return &countingConn{inner: c, driver: d}, nil
}

type countingConn struct {
	inner  driver.Conn
	driver *countingDriver
}

func (c *countingConn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

func (c *countingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	c.driver.prepared.Add(1)
	if p, ok := c.inner.(driver.ConnPrepareContext); ok {
		return p.PrepareContext(ctx, query)
	}
	return c.inner.Prepare(query)
}

func (c *countingConn) Close() error { return c.inner.Close() }

func (c *countingConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *countingConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if b, ok := c.inner.(driver.ConnBeginTx); ok {
		return b.BeginTx(ctx, opts)
	}
	return c.inner.Begin() //nolint:staticcheck // the fallback database/sql itself takes
}

var (
	counting     = &countingDriver{}
	registerOnce sync.Once
	countingMu   sync.Mutex
)

// openCounting is a store whose compiled statements are counted. The
// count is shared, so tests using it take countingMu.
func openCounting(t *testing.T, path string) (*stats.Store, *atomic.Int64) {
	t.Helper()
	registerOnce.Do(func() {
		db, err := sql.Open("sqlite", ":memory:")
		require.NoError(t, err)
		counting.inner = db.Driver()
		_ = db.Close()
		sql.Register("sqlite-counting", counting)
	})
	s, err := stats.OpenWith(path, "sqlite-counting")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s, &counting.prepared
}

// Item 213: every write the store made compiled its SQL again, which
// was an eighth of a settled daemon's CPU on a 2,000-session home with
// one session writing. The statements a by-path ingest uses are
// compiled once a store.
func TestSavePreparesOnce(t *testing.T) {
	countingMu.Lock()
	t.Cleanup(countingMu.Unlock)

	t.Run("when a store has ingested a transcript once", func(t *testing.T) {
		const session = "00000000-0000-4213-a000-000000000001"
		at := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
		n := 0
		line := func() string {
			n++
			at = at.Add(time.Minute)
			return reply{session: session, id: fmt.Sprintf("msg_213_%d", n), at: at.Format(time.RFC3339Nano), cwd: lib,
				usage: usage{Input: int64(n), Output: 1}}.line()
		}
		setup := func(t *testing.T) (*fixture, *atomic.Int64, string) {
			f := newFixture(t)
			store, prepared := openCounting(t, filepath.Join(t.TempDir(), "counted.db"))
			f.store = store
			path := f.transcript(session, line())
			require.NoError(t, f.store.IngestPaths(f.projects, []string{path}, false, f.now, nil))
			return f, prepared, path
		}

		t.Run("and the transcript gains a reply and is ingested by path, five times", func(t *testing.T) {
			f, prepared, path := setup(t)
			before := prepared.Load()
			for range 5 {
				f.appendRaw(path, line()+"\n")
				require.NoError(t, f.store.IngestPaths(f.projects, []string{path}, false, f.now, nil))
			}

			t.Run("it should prepare no statement after the first ingest", func(t *testing.T) {
				assert.Equal(t, int64(0), prepared.Load()-before)
			})
		})

		t.Run("and the transcript is rewritten from the start and ingested again", func(t *testing.T) {
			f, prepared, path := setup(t)
			before := prepared.Load()
			f.write(path, line())
			earlier := time.Now().Add(-time.Hour)
			require.NoError(t, os.Chtimes(path, earlier, earlier))
			require.NoError(t, f.store.IngestPaths(f.projects, []string{path}, false, f.now, nil))
			compiled := prepared.Load() - before
			require.Equal(t, 1, f.links(), "the rewritten transcript was not read again from the start")

			t.Run("it should prepare no statement after the first ingest", func(t *testing.T) {
				assert.Equal(t, int64(0), compiled)
			})
		})
	})
}
