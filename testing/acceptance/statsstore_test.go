package acceptance_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 190: a daemon whose statistics store will not open used to run
// with statistics off and say nothing. Over a store from a newer build
// it now logs the refusal and doctor warns, saying to update.
func TestRefusedStatisticsStore(t *testing.T) {
	bin, err := botropolisd()
	require.NoError(t, err)

	t.Run("when the daemon starts over a statistics store from a newer build", func(t *testing.T) {
		root := t.TempDir()
		home := helpers.LiveFixtureHome(t, "synthetic")
		db := filepath.Join(root, "state", "botropolis", "stats.db")
		s, err := stats.Open(db)
		require.NoError(t, err)
		for _, stmt := range []string{
			fmt.Sprintf("PRAGMA user_version = %d", stats.Version+1),
			fmt.Sprintf("UPDATE meta SET value = %d WHERE key = 'readable_from'", stats.Version+1),
		} {
			_, err = s.DB().Exec(stmt)
			require.NoError(t, err)
		}
		require.NoError(t, s.Close())

		sock := shortSocket(t)
		config := filepath.Join(root, "config.toml")
		env := append(os.Environ(), "HOME="+home, "XDG_STATE_HOME="+filepath.Join(root, "state"), "PATH=/nonexistent")
		daemon := exec.Command(bin, "--socket", sock, "--config", config)
		daemon.Env = env
		log := &syncBuffer{}
		daemon.Stderr = log
		require.NoError(t, daemon.Start())
		t.Cleanup(func() { _ = daemon.Process.Kill(); _ = daemon.Wait() })
		require.Eventually(t, func() bool { _, err := os.Stat(sock); return err == nil }, time.Minute, 20*time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		var out bytes.Buffer
		doctor := exec.CommandContext(ctx, botropolis, "doctor", "--socket", sock, "--config", config)
		doctor.Env = env
		doctor.Stdout, doctor.Stderr = &out, &out
		require.NoError(t, doctor.Run(), out.String())

		t.Run("it should log the refusal, naming the file", func(t *testing.T) {
			assert.Contains(t, log.String(), db+" is schema version")
		})

		t.Run("it should make doctor warn on statistics, saying to update", func(t *testing.T) {
			assert.Regexp(t, `statistics\s+warn\s+`+regexp.QuoteMeta(db)+` is .*update Botropolis`, out.String())
		})
	})
}
