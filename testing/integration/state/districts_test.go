package state_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 272: a temporary-directory session whose transcript ties it to a
// project stands in that project's district.
func TestLoadTiesTemporarySessions(t *testing.T) {
	const (
		kiln    = "/home/user/src/kiln"
		scratch = "/tmp/tmp.Kq81bZ"
		project = "00000272-0000-4000-8000-000000000001"
		tied    = "00000272-0000-4000-8000-000000000002"
		loose   = "00000272-0000-4000-8000-000000000003"
	)
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	ts := now.Add(-time.Hour).Format(time.RFC3339Nano)
	home := helpers.NewHome(t)
	home.Transcript(project, kiln, helpers.UserPrompt(project, kiln, ts), helpers.AssistantReply(project, "msg_272_1", ts))
	home.Transcript(tied, scratch, helpers.UserPrompt(tied, scratch, ts),
		helpers.AssistantFileEdit(tied, "msg_272_2", ts, "toolu_272_2", kiln+"/glaze.go"))
	home.Transcript(loose, "/var/tmp/run-7", helpers.UserPrompt(loose, "/var/tmp/run-7", ts), helpers.AssistantReply(loose, "msg_272_3", ts))

	probes := state.Probes{Alive: func(int) bool { return false }, Attached: func(string) bool { return false }}
	snapshot, err := state.NewLoader(home.Path, probes).Load(now)
	require.NoError(t, err)
	district := map[string]string{}
	for _, s := range snapshot.Sessions {
		district[s.ID] = s.District
	}

	t.Run("when a temporary-directory session edited a project's file", func(t *testing.T) {
		t.Run("it should be loaded tied to that project", func(t *testing.T) {
			assert.Equal(t, kiln, district[tied])
		})
	})

	t.Run("when a temporary-directory session has no tie", func(t *testing.T) {
		t.Run("it should be loaded with no district of its own", func(t *testing.T) {
			assert.Equal(t, "", district[loose])
		})
	})
}
