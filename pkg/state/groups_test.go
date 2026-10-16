package state_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	libDir = "/home/avesta/workspaces/github/acme/lib"
	apiDir = "/home/avesta/workspaces/github/acme/api"
)

// groupedHome is two closed sessions in two directories whose prompts
// share a ticket key.
func groupedHome(t *testing.T) *helpers.Home {
	t.Helper()
	home := helpers.NewHome(t)
	home.Transcript(sidA, libDir,
		helpers.UserPrompt(sidA, libDir, "2026-09-16T10:00:00.000Z"),
		helpers.AssistantReply(sidA, "msg_01", "2026-09-16T10:00:10.000Z"))
	home.Transcript(sidB, apiDir,
		helpers.UserPrompt(sidB, apiDir, "2026-09-16T14:00:00.000Z"),
		helpers.AssistantReply(sidB, "msg_02", "2026-09-16T14:00:10.000Z"))
	home.Prompt(sidA, libDir, "start PROJ-123 in the library", time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC))
	home.Prompt(sidB, apiDir, "bump the client for PROJ-123", time.Date(2026, 9, 16, 14, 0, 0, 0, time.UTC))
	return home
}

func TestLoaderStartedFrom(t *testing.T) {
	t.Run("when memory says a live session's job was started from a group", func(t *testing.T) {
		home := groupedHome(t).Session(4242, sidA, libDir, "interactive", "idle")
		path := filepath.Join(t.TempDir(), "groups.json")
		require.NoError(t, group.Save(path, group.Memory{
			Groups:      []group.Stored{{ID: "g-new", Name: "Gateway", Renamed: true}},
			StartedFrom: map[string]string{"0898d7e4": "g-new"},
		}))
		snapshot, err := state.NewLoader(home.Path, alive).WithGroups(path).Load(now)
		require.NoError(t, err)

		t.Run("it should put the session in that group", func(t *testing.T) {
			assert.Equal(t, "g-new", sessionByID(t, snapshot, sidA).Group)
		})
	})
}

func TestLoaderGroups(t *testing.T) {
	t.Run("when two sessions in different directories share a ticket key in their prompts", func(t *testing.T) {
		home := groupedHome(t)
		loader := state.NewLoader(home.Path, dead)
		snapshot, err := loader.Load(now)
		require.NoError(t, err)

		t.Run("it should put one group in the snapshot", func(t *testing.T) {
			assert.Len(t, snapshot.Groups, 1)
		})

		t.Run("it should mark each session with the group's id", func(t *testing.T) {
			require.NotEmpty(t, snapshot.Groups)
			assert.Equal(t, []string{snapshot.Groups[0].ID, snapshot.Groups[0].ID},
				[]string{sessionByID(t, snapshot, sidA).Group, sessionByID(t, snapshot, sidB).Group})
		})

		t.Run("it should say why the later session joined", func(t *testing.T) {
			assert.Equal(t, "mentions PROJ-123", sessionByID(t, snapshot, sidB).GroupJoined)
		})

		t.Run("it should take a session's last prompt from the history when its transcript has none", func(t *testing.T) {
			assert.Equal(t, "bump the client for PROJ-123", sessionByID(t, snapshot, sidB).LastPrompt)
		})

		t.Run("it should carry each member's directory", func(t *testing.T) {
			require.NotEmpty(t, snapshot.Groups)
			assert.Equal(t, []string{libDir, apiDir}, snapshot.Groups[0].MemberDirs)
		})

		t.Run("it should say why the group formed", func(t *testing.T) {
			require.NotEmpty(t, snapshot.Groups)
			assert.Equal(t, "mentions PROJ-123", snapshot.Groups[0].Reason)
		})

		t.Run("it should say the first run has not been reviewed", func(t *testing.T) {
			assert.False(t, snapshot.GroupsReviewed)
		})

		t.Run("and the first run has been reviewed", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "groups.json")
			at := now.Add(-time.Hour)
			require.NoError(t, group.Save(path, group.Memory{Reviewed: &at}))
			snapshot, err := state.NewLoader(home.Path, dead).WithGroups(path).Load(now)
			require.NoError(t, err)

			t.Run("it should say so", func(t *testing.T) {
				assert.True(t, snapshot.GroupsReviewed)
			})
		})

		t.Run("it should give the group its move", func(t *testing.T) {
			require.NotEmpty(t, snapshot.Groups)
			assert.Equal(t, state.MoveNobody, snapshot.Groups[0].Move)
		})

		t.Run("and a store path is set", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "groups.json")
			_, err := state.NewLoader(home.Path, dead).WithGroups(path).Load(now)
			require.NoError(t, err)
			memory, err := group.Load(path)
			require.NoError(t, err)

			t.Run("it should remember the group there", func(t *testing.T) {
				require.Len(t, memory.Groups, 1)
				assert.ElementsMatch(t, []string{sidA, sidB}, memory.Groups[0].Members)
			})
		})

		t.Run("and none is set", func(t *testing.T) {
			state := t.TempDir()
			t.Setenv("XDG_STATE_HOME", state)
			_, err := loader.Load(now.Add(time.Second))
			require.NoError(t, err)
			entries, _ := os.ReadDir(state)

			t.Run("it should write nothing", func(t *testing.T) {
				assert.Empty(t, entries)
			})
		})

		t.Run("and a third session was started within a minute of one of them", func(t *testing.T) {
			const sidC = "2222aaaa-0000-0000-0000-000000000000"
			home := groupedHome(t)
			home.Transcript(sidC, apiDir,
				helpers.UserPrompt(sidC, apiDir, "2026-09-16T14:00:30.000Z"),
				helpers.AssistantReply(sidC, "msg_03", "2026-09-16T14:00:40.000Z"))
			home.Prompt(sidC, apiDir, "tidy the readme", time.Date(2026, 9, 16, 14, 0, 30, 0, time.UTC))
			snapshot, err := state.NewLoader(home.Path, dead).Load(now)
			require.NoError(t, err)

			t.Run("it should say its join is a guess", func(t *testing.T) {
				assert.True(t, strings.HasPrefix(sessionByID(t, snapshot, sidC).GroupJoined, "likely: "))
			})

			t.Run("and a prompt is sent in it after it joined", func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "groups.json")
				loader := state.NewLoader(home.Path, dead).WithGroups(path)
				_, err := loader.Load(now)
				require.NoError(t, err)
				home.Prompt(sidC, apiDir, "and the changelog", now.Add(30*time.Second))
				snapshot, err := loader.Load(now.Add(time.Minute))
				require.NoError(t, err)
				memory, err := group.Load(path)
				require.NoError(t, err)

				t.Run("it should be confirmed in memory", func(t *testing.T) {
					assert.Contains(t, memory.Confirmed, sidC)
				})

				t.Run("its tie should no longer be a guess", func(t *testing.T) {
					assert.False(t, strings.HasPrefix(sessionByID(t, snapshot, sidC).GroupJoined, "likely: "))
				})
			})
		})

		t.Run("and a correction is written to the store", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "groups.json")
			loader := state.NewLoader(home.Path, dead).WithGroups(path)
			_, err := loader.Load(now)
			require.NoError(t, err)
			memory, err := group.Load(path)
			require.NoError(t, err)
			require.NoError(t, group.Save(path, memory.WithMove(sidB, "")))
			snapshot, err := loader.Load(now.Add(time.Second))
			require.NoError(t, err)

			t.Run("it should detect again and honour it", func(t *testing.T) {
				assert.Equal(t, "", sessionByID(t, snapshot, sidB).Group)
			})
		})

		t.Run("and it loads again with nothing changed", func(t *testing.T) {
			before := loader.Detections()
			_, err := loader.Load(now.Add(2 * time.Second))
			require.NoError(t, err)

			t.Run("it should not detect again", func(t *testing.T) {
				assert.Equal(t, before, loader.Detections())
			})
		})
	})
}
