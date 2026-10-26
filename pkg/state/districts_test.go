package state_test

import (
	"testing"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
)

const (
	kiln    = "/home/u/src/kiln"
	orchard = "/home/u/src/orchard"
	scratch = "/tmp/tmp.Kq81bZ"
)

func districtOf(sessions []state.Session, id string) string {
	state.AssignDistricts(sessions)
	for _, s := range sessions {
		if s.ID == id {
			return s.District
		}
	}
	return "missing"
}

func TestAssignDistricts(t *testing.T) {
	ties := []struct {
		name     string
		project  state.Session
		temp     state.Session
		expected string
	}{
		{"it is Claude Code's scratchpad for that project",
			state.Session{ID: "p", CWD: "/home/u/src/botropolis.city"},
			state.Session{ID: "t", CWD: "/tmp/claude-1000/-home-u-src-botropolis-city/5f0c/scratchpad"}, "/home/u/src/botropolis.city"},
		{"it is a teammate of a team the project's session leads",
			state.Session{ID: "p", CWD: kiln, Leads: "glaze", Team: "glaze"},
			state.Session{ID: "t", CWD: scratch, Team: "glaze", Agent: "tester"}, kiln},
		{"the project's subagents worked in its directory",
			state.Session{ID: "p", CWD: kiln, Visits: map[string]int{scratch: 2}},
			state.Session{ID: "t", CWD: scratch}, kiln},
		{"it is on the same team",
			state.Session{ID: "p", CWD: kiln, Team: "glaze", Agent: "writer"},
			state.Session{ID: "t", CWD: scratch, Team: "glaze", Agent: "tester"}, kiln},
		{"it messaged the project's session",
			state.Session{ID: "p", CWD: kiln},
			state.Session{ID: "t", CWD: scratch, Messaged: []string{"p"}}, kiln},
		{"the project's session messaged it",
			state.Session{ID: "p", CWD: kiln, Messaged: []string{"t"}},
			state.Session{ID: "t", CWD: scratch}, kiln},
		{"the project's session sent it a peer message",
			state.Session{ID: "p", CWD: kiln, PID: 4242},
			state.Session{ID: "t", CWD: scratch, PeerMessages: []claude.PeerMessage{{FromPID: 4242}}}, kiln},
		{"it edited files in the project",
			state.Session{ID: "p", CWD: kiln},
			state.Session{ID: "t", CWD: scratch, Touches: map[string]int{kiln + "/glaze.go": 3}}, kiln},
	}
	for _, tie := range ties {
		t.Run("when a temporary-directory session is tied because "+tie.name, func(t *testing.T) {
			t.Run("it should stand in that session's district", func(t *testing.T) {
				assert.Equal(t, tie.expected, districtOf([]state.Session{tie.project, tie.temp}, "t"))
			})
		})
	}

	t.Run("when a temporary-directory session is tied only through another tied one", func(t *testing.T) {
		sessions := []state.Session{
			{ID: "p", CWD: kiln},
			{ID: "t1", CWD: scratch, Messaged: []string{"p"}},
			{ID: "t2", CWD: "/var/tmp/run-7", Messaged: []string{"t1"}},
		}
		t.Run("it should stand in the same district", func(t *testing.T) {
			assert.Equal(t, kiln, districtOf(sessions, "t2"))
		})
	})

	t.Run("when a temporary-directory session is tied to two districts", func(t *testing.T) {
		sessions := []state.Session{
			{ID: "p", CWD: kiln},
			{ID: "q", CWD: orchard, Team: "plots"},
			{ID: "t", CWD: scratch, Team: "plots", Touches: map[string]int{kiln + "/a.go": 9, kiln + "/b.go": 9}},
		}
		t.Run("it should take the strongest tie", func(t *testing.T) {
			assert.Equal(t, orchard, districtOf(sessions, "t"))
		})
	})

	t.Run("when a temporary-directory session has no tie", func(t *testing.T) {
		sessions := []state.Session{{ID: "p", CWD: kiln}, {ID: "t", CWD: scratch}}
		t.Run("it should be left to the Temporary district", func(t *testing.T) {
			assert.Equal(t, "", districtOf(sessions, "t"))
		})
	})

	t.Run("when two temporary-directory sessions are tied only to each other", func(t *testing.T) {
		sessions := []state.Session{{ID: "t1", CWD: scratch, Messaged: []string{"t2"}}, {ID: "t2", CWD: "/var/tmp/run-7"}}
		t.Run("it should leave both to the Temporary district", func(t *testing.T) {
			assert.Equal(t, "", districtOf(sessions, "t1"))
		})
	})

	t.Run("when a session outside a temporary directory has ties elsewhere", func(t *testing.T) {
		sessions := []state.Session{{ID: "p", CWD: kiln}, {ID: "q", CWD: orchard, Messaged: []string{"p"}}}
		t.Run("it should stay in its own district", func(t *testing.T) {
			assert.Equal(t, "", districtOf(sessions, "q"))
		})
	})
}

func TestTemporary(t *testing.T) {
	for path, want := range map[string]bool{
		"/tmp":                         true,
		"/tmp/tmp.Xq81bZ/work":         true,
		"/var/tmp/run-7":               true,
		"/var/folders/zx/k3j9/T/x":     true,
		"/private/var/folders/zx/k3/T": true,
		"/home/u/tmp-project":          false,
		"/home/u/work/tmp":             false,
		"/tmpfoo":                      false,
		"":                             false,
	} {
		t.Run("when the directory is "+path, func(t *testing.T) {
			t.Run("it should say whether it is temporary", func(t *testing.T) {
				assert.Equal(t, want, state.Temporary(path))
			})
		})
	}
}
