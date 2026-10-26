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

func TestLoaderGroupBound(t *testing.T) {
	t.Run("when more sessions than a group can hold share one ticket key", func(t *testing.T) {
		home := helpers.NewHome(t)
		extra := 3
		for i := 0; i < group.MaxMembers+extra; i++ {
			sid := fmt.Sprintf("00000000-0000-4000-8000-%012d", i)
			dir := fmt.Sprintf("/home/avesta/w/kiln%02d", i)
			at := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * 3 * time.Hour)
			home.Transcript(sid, dir,
				helpers.UserPrompt(sid, dir, at.Format("2006-01-02T15:04:05.000Z")),
				helpers.AssistantReply(sid, fmt.Sprintf("msg_%02d", i), at.Add(10*time.Second).Format("2006-01-02T15:04:05.000Z")))
			home.Prompt(sid, dir, "glaze the kiln for PROJ-77", at)
		}
		var logged []string
		loader := state.NewLoader(home.Path, dead).WithLog(func(format string, args ...any) {
			logged = append(logged, fmt.Sprintf(format, args...))
		})
		snapshot, err := loader.Load(now)
		require.NoError(t, err)
		_, err = loader.Load(now.Add(time.Second))
		require.NoError(t, err)

		t.Run("it should draw no group larger than the bound", func(t *testing.T) {
			require.Len(t, snapshot.Groups, 1)
			assert.Len(t, snapshot.Groups[0].Members, group.MaxMembers)
		})

		t.Run("it should log the sessions it left out, once", func(t *testing.T) {
			assert.Equal(t, []string{fmt.Sprintf(
				"botropolisd: %d sessions were left out of groups already holding %d sessions (item 271)", extra, group.MaxMembers)}, logged)
		})
	})
}
