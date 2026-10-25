package demo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

const (
	leadID = "7e4d0b2a-0000-4000-8000-000000000001"
	mateID = "9c1f5e3d-0000-4000-8000-000000000002"
	// teamName is unrelated to either session's id, as Claude Code 2.1.310
	// names a team (item 105).
	teamName = "session-2aa29be8"
	mateName = "reader"
)

// teamLine is a teammate's line, which carries its team and name.
func teamLine(id, kind, at, agent string, extra string) string {
	l := `{"type":"` + kind + `","sessionId":"` + id + `","cwd":"` + cwd + `","timestamp":"` + at + `","teamName":"` + teamName + `"`
	if agent != "" {
		l += `,"agentName":"` + agent + `"`
	}
	return l + extra + "}"
}

// leadLine is a lead's line, which names no team.
func leadLine(kind, at, extra string) string {
	return `{"type":"` + kind + `","sessionId":"` + leadID + `","cwd":"` + cwd + `","timestamp":"` + at + `"` + extra + "}"
}

// newTeamCorpus is a lead in tidepool and its teammate, recorded beside
// it as Claude Code files one: a session of its own carrying the team,
// which reports back with SendMessage to team-lead. The lead names the
// team only in the result of the Agent call that spawned the teammate.
func newTeamCorpus(t *testing.T) Corpus {
	t.Helper()
	dir := t.TempDir()
	spawned := `,"toolUseResult":{"status":"teammate_spawned","name":"` + mateName + `","teammate_id":"` + mateName + "@" + teamName + `","team_name":"` + teamName + `"}`
	writeFile(t, filepath.Join(dir, "tidepool", leadID+".jsonl"), strings.Join([]string{
		leadLine("user", "2026-09-01T10:00:00.000Z", ""),
		leadLine("user", "2026-09-01T10:00:30.000Z", spawned),
		leadLine("assistant", "2026-09-01T10:04:00.000Z", ""),
	}, "\n")+"\n")
	send := `,"message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"SendMessage","input":{"to":"team-lead","message":"done"}}]}`
	writeFile(t, filepath.Join(dir, "tidepool", mateID+".jsonl"), strings.Join([]string{
		teamLine(mateID, "user", "2026-09-01T10:01:00.000Z", mateName, ""),
		teamLine(mateID, "assistant", "2026-09-01T10:03:00.000Z", mateName, send),
	}, "\n")+"\n")
	return Corpus{Dir: dir}
}

func teamConfig(t *testing.T, home string) claude.Team {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".claude", "teams", teamName, "config.json"))
	require.NoError(t, err)
	var team claude.Team
	require.NoError(t, json.Unmarshal(data, &team))
	return team
}

func TestCorpusFindTeammate(t *testing.T) {
	corpus := newTeamCorpus(t)

	t.Run("when a ref names a lead and a teammate's agent name", func(t *testing.T) {
		rec, err := corpus.Find("tidepool/7e4d@" + mateName)
		require.NoError(t, err)

		t.Run("it should find the teammate's transcript", func(t *testing.T) {
			assert.Equal(t, mateID, rec.ID)
		})
	})

	t.Run("when no transcript in the lead's team has that agent name", func(t *testing.T) {
		_, err := corpus.Find("tidepool/7e4d@writer")

		t.Run("it should refuse the ref, naming it", func(t *testing.T) {
			assert.ErrorContains(t, err, "tidepool/7e4d@writer")
		})
	})
}

func TestStageTeam(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	corpus := newTeamCorpus(t)
	lead := Placement{Ref: "tidepool/7e4d", State: state.Working}
	mate := Placement{Ref: "tidepool/7e4d@" + mateName, State: state.Working}
	member := func(team claude.Team, name string) claude.TeamMember {
		m, _ := team.Member(name)
		return m
	}

	t.Run("when a lead and its teammate are placed live", func(t *testing.T) {
		s := stage(t, corpus, Scenario{Name: "team", Sessions: []Placement{lead, mate}}, now)
		require.NoError(t, s.err)

		t.Run("it should write the team's config under teams/<team>/", func(t *testing.T) {
			assert.FileExists(t, filepath.Join(s.home, ".claude", "teams", teamName, "config.json"))
		})

		team := teamConfig(t, s.home)

		t.Run("it should name the lead's session as leadSessionId", func(t *testing.T) {
			assert.Equal(t, leadID, team.LeadSessionID)
		})
		t.Run("it should list the lead as team-lead with the lead's working directory", func(t *testing.T) {
			assert.Equal(t, cwd, member(team, "team-lead").CWD)
		})
		t.Run("it should list the teammate under its agent name with its working directory", func(t *testing.T) {
			assert.Equal(t, cwd, member(team, mateName).CWD)
		})

		t.Run("and the teammate is cloned into another project", func(t *testing.T) {
			cloned := mate
			cloned.Project = "kiln"
			s := stage(t, corpus, Scenario{Name: "team", Sessions: []Placement{lead, cloned}}, now)
			require.NoError(t, s.err)

			t.Run("it should give the teammate that project's working directory", func(t *testing.T) {
				assert.Equal(t, Root+"/kiln", member(teamConfig(t, s.home), mateName).CWD)
			})
		})

		t.Run("and the lead is resting", func(t *testing.T) {
			resting := lead
			resting.State = state.Resting
			s := stage(t, corpus, Scenario{Name: "team", Sessions: []Placement{resting, mate}}, now)
			require.NoError(t, s.err)

			t.Run("it should write no config, as Claude Code removes it when the lead ends", func(t *testing.T) {
				assert.NoDirExists(t, filepath.Join(s.home, ".claude", "teams"))
			})
		})
	})

	t.Run("when a lead is cloned into another project", func(t *testing.T) {
		cloned := lead
		cloned.Project = "kiln"
		s := stage(t, corpus, Scenario{Name: "team", Sessions: []Placement{cloned, mate}}, now)

		t.Run("it should refuse, since the clone would lead its recording's team too", func(t *testing.T) {
			assert.ErrorContains(t, s.err, "lead")
		})
	})

	t.Run("when a placement is in no team", func(t *testing.T) {
		s := stage(t, newCorpus(t), Scenario{Name: "solo", Sessions: []Placement{{Ref: "tidepool/6f1d", State: state.Working}}}, now)
		require.NoError(t, s.err)

		t.Run("it should write no teams directory", func(t *testing.T) {
			assert.NoDirExists(t, filepath.Join(s.home, ".claude", "teams"))
		})
	})
}
