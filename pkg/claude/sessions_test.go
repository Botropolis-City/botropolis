package claude_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const wellFormedRecord = `{"pid":1646630,"sessionId":"0898d7e4-f2cb-45fe-833e-1921c1f4bcaa",` +
	`"cwd":"/home/avesta/workspaces/github/auroq/botropolis","startedAt":1789617482873,` +
	`"procStart":"2189871","version":"2.1.273","peerProtocol":1,` +
	`"peerFeatures":["notify_idle","reply_across_default_dirs","artifact_yield"],` +
	`"kind":"bg","entrypoint":"cli","pidDomain":"<scrubbed>",` +
	`"messagingSocketPath":"/run/user/1000/cc-socks/1646630.sock",` +
	`"name":"scaffold milestone setup","nameSince":1789617584013,"jobId":"0898d7e4",` +
	`"status":"busy","updatedAt":1789617584013,"statusUpdatedAt":1789617580056,` +
	`"bridgeSessionId":"<scrubbed>","nameSource":"auto"}`

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestReadSessionRecords(t *testing.T) {
	t.Run("when the directory holds one well-formed record", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "1646630.json", wellFormedRecord)

		records, _, err := claude.ReadSessionRecords(dir)
		require.NoError(t, err)
		require.Len(t, records, 1)
		record := records[0]

		t.Run("it should read the pid", func(t *testing.T) {
			assert.Equal(t, 1646630, record.PID)
		})

		t.Run("it should read the session id", func(t *testing.T) {
			assert.Equal(t, "0898d7e4-f2cb-45fe-833e-1921c1f4bcaa", record.SessionID)
		})

		t.Run("it should read the cwd", func(t *testing.T) {
			assert.Equal(t, "/home/avesta/workspaces/github/auroq/botropolis", record.CWD)
		})

		t.Run("it should read the name", func(t *testing.T) {
			assert.Equal(t, "scaffold milestone setup", record.Name)
		})

		t.Run("it should read kind bg as KindBackground", func(t *testing.T) {
			assert.Equal(t, claude.KindBackground, record.Kind)
		})

		t.Run("it should read the status", func(t *testing.T) {
			assert.Equal(t, claude.StatusBusy, record.Status)
		})

		t.Run("it should convert startedAt from epoch milliseconds to UTC time", func(t *testing.T) {
			assert.Equal(t, time.Date(2026, time.September, 17, 3, 58, 2, 873_000_000, time.UTC), record.StartedAt)
		})

		t.Run("it should remember the path it was read from", func(t *testing.T) {
			assert.Equal(t, filepath.Join(dir, "1646630.json"), record.Path)
		})
	})

	t.Run("when the directory holds a .key file beside the record", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "1646630.json", wellFormedRecord)
		writeFile(t, dir, "1646630.9e625c429e8b1c27.key", `{"key":"secret"}`)

		records, _, err := claude.ReadSessionRecords(dir)
		require.NoError(t, err)

		t.Run("it should not return a record for the key file", func(t *testing.T) {
			assert.Len(t, records, 1)
		})
	})

	t.Run("when one record is not valid JSON", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "1646630.json", wellFormedRecord)
		truncated := writeFile(t, dir, "16548.json", `{"pid":16548,"sessionId":"1377da2f-4`)

		records, skipped, err := claude.ReadSessionRecords(dir)
		require.NoError(t, err)

		t.Run("it should skip it and return the others", func(t *testing.T) {
			assert.Len(t, records, 1)
		})

		t.Run("it should report the skipped file's path", func(t *testing.T) {
			require.Len(t, skipped, 1)
			assert.Equal(t, truncated, skipped[0].Path)
		})
	})

	t.Run("when the directory does not exist", func(t *testing.T) {
		records, skipped, err := claude.ReadSessionRecords(filepath.Join(t.TempDir(), "sessions"))

		t.Run("it should return no error", func(t *testing.T) {
			assert.NoError(t, err)
		})

		t.Run("it should return no records", func(t *testing.T) {
			assert.Empty(t, records)
		})

		t.Run("it should report nothing skipped", func(t *testing.T) {
			assert.Empty(t, skipped)
		})
	})
}
