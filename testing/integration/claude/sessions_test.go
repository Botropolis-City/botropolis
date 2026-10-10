package claude_test

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadSessionRecordsFromFixture(t *testing.T) {
	t.Run("when reading the sample fixture's sessions directory", func(t *testing.T) {
		dir := filepath.Join(helpers.FixtureHome(t, "sample"), ".claude", "sessions")

		records, skipped, err := claude.ReadSessionRecords(dir)
		require.NoError(t, err)
		require.Empty(t, skipped)

		t.Run("it should return one record per live session in the manifest", func(t *testing.T) {
			assert.Len(t, records, helpers.Manifest(t, "sample").LiveSessions)
		})

		t.Run("it should read each record's pid matching its file name", func(t *testing.T) {
			for _, record := range records {
				stem := strings.TrimSuffix(filepath.Base(record.Path), ".json")
				assert.Equal(t, stem, strconv.Itoa(record.PID))
			}
		})
	})
}
