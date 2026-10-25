package demo_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/demo"
	"botropolis.city/botropolis/pkg/state"
)

// recordedTeam writes a lead and its teammate into the corpus, both in
// tidepool, as Claude Code files them: the teammate is a session of its
// own carrying the lead's team, and it reports with SendMessage, as the lead asks it to.
func recordedTeam(t *testing.T, corpus, leadID, mateID, mate string) {
	t.Helper()
	cwd := demo.Root + "/tidepool"
	team := demo.TeamOf(leadID)
	line := func(id, kind, at, agent, extra string) string {
		l := `{"type":"` + kind + `","sessionId":"` + id + `","cwd":"` + cwd + `","timestamp":"` + at + `","teamName":"` + team + `"`
		if agent != "" {
			l += `,"agentName":"` + agent + `"`
		}
		return l + extra + "}"
	}
	send := `,"message":{"id":"msg_send","role":"assistant","content":[{"type":"tool_use","id":"toolu_send","name":"SendMessage","input":{"to":"team-lead","message":"done"}}]}`
	files := map[string][]string{
		leadID: {line(leadID, "user", "2026-09-01T10:00:00.000Z", "", ""), line(leadID, "assistant", "2026-09-01T10:04:00.000Z", "", strings.ReplaceAll(strings.ReplaceAll(send, "team-lead", mate), "_send", "_ask"))},
		mateID: {line(mateID, "user", "2026-09-01T10:01:00.000Z", mate, ""), line(mateID, "assistant", "2026-09-01T10:03:00.000Z", mate, send)},
	}
	for id, lines := range files {
		path := filepath.Join(corpus, "tidepool", id+".jsonl")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
	}
}

func TestStagedTeamSendsRoadTraffic(t *testing.T) {
	now := time.Now()
	corpus := t.TempDir()
	const leadID, mateID = "7e4d0b2a-0000-4000-8000-000000000001", "9c1f5e3d-0000-4000-8000-000000000002"
	recordedTeam(t, corpus, leadID, mateID, "reader")
	scenario := demo.Scenario{Name: "team", Sessions: []demo.Placement{
		{Ref: "tidepool/7e4d", State: state.Working, Ago: demo.Duration(time.Minute)},
		{Ref: "tidepool/7e4d@reader", Project: "kiln", State: state.Working, Ago: demo.Duration(time.Minute)},
	}}
	home := t.TempDir()
	require.NoError(t, demo.Stage(demo.Corpus{Dir: corpus}, scenario, home, now, demo.Sleepers(home, time.Minute)))
	t.Cleanup(func() { _ = demo.Unstage(home) })
	snapshot, err := state.Load(home, probes, now)
	require.NoError(t, err)

	t.Run("when a scenario stages a lead and a teammate in another project", func(t *testing.T) {
		messages := 0
		for _, r := range snapshot.Roads {
			ends := map[string]bool{r.From: true, r.To: true}
			if ends[demo.Root+"/tidepool"] && ends[demo.Root+"/kiln"] {
				messages += r.Messages + r.BackMessages
			}
		}

		t.Run("it should show both messages, each way, as road traffic between the two districts", func(t *testing.T) {
			assert.Equal(t, 2, messages)
		})
	})
}
