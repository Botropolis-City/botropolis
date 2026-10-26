package state_test

import (
	"fmt"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadTwice loads a home of n sessions three hours apart, each prompted
// as prompt says, twice, and gives the first snapshot and what the
// loader logged across both.
func loadTwice(t *testing.T, n int, prompt func(i int) string) (state.Snapshot, []string) {
	t.Helper()
	home := helpers.NewHome(t)
	for i := 0; i < n; i++ {
		sid := fmt.Sprintf("00000000-0000-4000-8000-%012d", i)
		dir := fmt.Sprintf("/home/avesta/w/kiln%02d", i)
		at := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * 3 * time.Hour)
		home.Transcript(sid, dir,
			helpers.UserPrompt(sid, dir, at.Format("2006-01-02T15:04:05.000Z")),
			helpers.AssistantReply(sid, fmt.Sprintf("msg_%02d", i), at.Add(10*time.Second).Format("2006-01-02T15:04:05.000Z")))
		home.Prompt(sid, dir, prompt(i), at)
	}
	var logged []string
	loader := state.NewLoader(home.Path, dead).WithLog(func(format string, args ...any) {
		logged = append(logged, fmt.Sprintf(format, args...))
	})
	snapshot, err := loader.Load(now)
	require.NoError(t, err)
	_, err = loader.Load(now.Add(time.Second))
	require.NoError(t, err)
	return snapshot, logged
}

func TestLoaderGroupBound(t *testing.T) {
	t.Run("when more sessions than a group can hold are chained by ticket keys each pair shares", func(t *testing.T) {
		snapshot, logged := loadTwice(t, group.MaxMembers+1, func(i int) string {
			return fmt.Sprintf("glaze the kiln for PROJ-%d and PROJ-%d", i, i+1)
		})

		t.Run("it should draw no group larger than the bound", func(t *testing.T) {
			require.Len(t, snapshot.Groups, 1)
			assert.Len(t, snapshot.Groups[0].Members, group.MaxMembers)
		})

		t.Run("it should log the session it left out, once", func(t *testing.T) {
			assert.Equal(t, []string{fmt.Sprintf(
				"botropolisd: groups are full at %d sessions; 1 more left out of them (item 271)", group.MaxMembers)}, logged)
		})
	})

	t.Run("when more sessions than a group can hold share one ticket key", func(t *testing.T) {
		snapshot, logged := loadTwice(t, group.MaxMembers+1, func(int) string { return "glaze the kiln for PROJ-77" })

		t.Run("it should draw no group", func(t *testing.T) {
			assert.Empty(t, snapshot.Groups)
		})

		t.Run("it should log the key, once", func(t *testing.T) {
			assert.Equal(t, []string{fmt.Sprintf(
				"botropolisd: shared by more than %d sessions, so grouping none of them: reference PROJ-77 (item 271)", group.MaxMembers)}, logged)
		})
	})
}
