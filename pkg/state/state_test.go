package state_test

import (
	"fmt"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sidA = "08b1d655-276f-495b-a61f-2214af35a97a"
	sidB = "1377da2f-4057-43fe-b3db-0293eb36bc6e"
)

var (
	now      = time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC)
	started  = now.Add(-2 * time.Hour)
	alive    = state.Probes{Alive: func(int) bool { return true }, Attached: func(string) bool { return false }}
	attached = state.Probes{Alive: func(int) bool { return true }, Attached: func(string) bool { return true }}
	dead     = state.Probes{Alive: func(int) bool { return false }, Attached: func(string) bool { return false }}
)

func record(sid string, kind claude.Kind, status claude.Status) claude.SessionRecord {
	return claude.SessionRecord{
		PID: 4242, SessionID: sid, CWD: "/home/avesta/workspaces/github/mCedar/cinders",
		Kind: kind, Status: status, Name: "record name", StartedAt: started, JobID: "0898d7e4",
	}
}

var roster = map[string]claude.Worker{"0898d7e4": {JobID: "0898d7e4", PtySock: "/tmp/cc/pty/0898d7e4.sock"}}

func transcript(sid string, turn claude.Turn) claude.Transcript {
	return claude.Transcript{
		SessionID: sid, CWD: "/home/avesta/workspaces/github/mCedar/cinders", Branch: "main",
		Title: "Fix the CI queue", Model: "claude-opus-5", ModelID: "claude-opus-5[1m]",
		Usage:         claude.Usage{Input: 1000, Output: 600, CacheRead: 1000, CacheCreate: 1000, Messages: 3},
		ContextTokens: 250_000,
		Cost:          claude.Cost{TotalUSD: 8.65, Known: true},
		FirstAt:       started, LastAt: started.Add(time.Hour),
		Tail: claude.Tail{Turn: turn, Prompts: 1, LastPromptAt: started},
	}
}

func build(t *testing.T, records []claude.SessionRecord, transcripts []claude.Transcript,
	subagents map[string][]claude.Subagent, probes state.Probes) []state.Session {
	t.Helper()
	sessions := state.Build(state.Sources{Records: records, Transcripts: transcripts, Subagents: subagents, Roster: roster}, probes, now)
	require.Len(t, sessions, len(records))
	return sessions
}

func TestWaitingSession(t *testing.T) {
	t.Run("when a live session is waiting on its own watch", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindBackground, claude.StatusIdle)},
			[]claude.Transcript{transcript(sidA, claude.TurnWaiting)}, nil, alive)[0]

		t.Run("it should be waiting, not needing you", func(t *testing.T) {
			assert.Equal(t, state.Waiting, session.State)
		})
	})
}

func TestBuild(t *testing.T) {
	t.Run("when a live interactive session is awaiting the user", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindInteractive, claude.StatusIdle)},
			[]claude.Transcript{transcript(sidA, claude.TurnAwaitingUser)}, nil, alive)[0]

		t.Run("it should need you", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, session.State)
		})

		t.Run("it should take the title from the transcript", func(t *testing.T) {
			assert.Equal(t, "Fix the CI queue", session.Title)
		})

		t.Run("it should take the branch from the transcript", func(t *testing.T) {
			assert.Equal(t, "main", session.Branch)
		})

		t.Run("it should measure context against a 1M window for a [1m] model id", func(t *testing.T) {
			assert.InDelta(t, 25.0, session.ContextPercent, 1e-9)
		})

		t.Run("it should show the full model id", func(t *testing.T) {
			assert.Equal(t, "claude-opus-5[1m]", session.Model)
		})

		t.Run("it should report the cost", func(t *testing.T) {
			assert.InDelta(t, 8.65, session.CostUSD, 1e-9)
		})

		t.Run("it should know the cost", func(t *testing.T) {
			assert.True(t, session.CostKnown)
		})

		t.Run("it should rate tokens over the transcript's own span", func(t *testing.T) {
			assert.InDelta(t, 3600.0, session.TokensPerHour, 1e-9)
		})

		t.Run("it should rate fresh tokens, input and cache writes, apart from cache reads and output", func(t *testing.T) {
			assert.InDelta(t, 2000.0, session.FreshTokensPerHour, 1e-9)
		})

		t.Run("it should rate output on its own", func(t *testing.T) {
			assert.InDelta(t, 600.0, session.OutputPerHour, 1e-9)
		})

		t.Run("it should rate cache reads separately from fresh tokens", func(t *testing.T) {
			assert.InDelta(t, 1000.0, session.CacheReadPerHour, 1e-9)
		})

		t.Run("it should be alive", func(t *testing.T) {
			assert.True(t, session.Alive)
		})

		t.Run("it should report the last activity", func(t *testing.T) {
			assert.Equal(t, started.Add(time.Hour), session.LastActivity)
		})
	})

	t.Run("when the transcript needs input", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindInteractive, claude.StatusBusy)},
			[]claude.Transcript{transcript(sidA, claude.TurnNeedsInput)}, nil, alive)[0]

		t.Run("it should need you", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, session.State)
		})
	})

	t.Run("when an interactive session is mid-turn", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindInteractive, claude.StatusBusy)},
			[]claude.Transcript{transcript(sidA, claude.TurnWorking)}, nil, alive)[0]

		t.Run("it should be working", func(t *testing.T) {
			assert.Equal(t, state.Working, session.State)
		})
	})

	t.Run("when a background session is mid-turn with no terminal attached", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindBackground, claude.StatusBusy)},
			[]claude.Transcript{transcript(sidA, claude.TurnWorking)}, nil, alive)[0]

		t.Run("it should be unattended", func(t *testing.T) {
			assert.Equal(t, state.Unattended, session.State)
		})

		t.Run("it should not be attached", func(t *testing.T) {
			assert.False(t, session.Attached)
		})
	})

	t.Run("when a background session is mid-turn with a terminal attached", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindBackground, claude.StatusBusy)},
			[]claude.Transcript{transcript(sidA, claude.TurnWorking)}, nil, attached)[0]

		t.Run("it should be working", func(t *testing.T) {
			assert.Equal(t, state.Working, session.State)
		})

		t.Run("it should be attached", func(t *testing.T) {
			assert.True(t, session.Attached)
		})
	})

	t.Run("when a background session has no roster entry", func(t *testing.T) {
		r := record(sidA, claude.KindBackground, claude.StatusBusy)
		r.JobID = "unknown"
		session := build(t, []claude.SessionRecord{r}, []claude.Transcript{transcript(sidA, claude.TurnWorking)}, nil, attached)[0]

		t.Run("it should be unattended", func(t *testing.T) {
			assert.Equal(t, state.Unattended, session.State)
		})
	})

	t.Run("when the pid is gone", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindInteractive, claude.StatusBusy)},
			[]claude.Transcript{transcript(sidA, claude.TurnWorking)}, nil, dead)[0]

		t.Run("it should be parked", func(t *testing.T) {
			assert.Equal(t, state.Resting, session.State)
		})

		t.Run("it should not be alive", func(t *testing.T) {
			assert.False(t, session.Alive)
		})
	})

	t.Run("when the record has no transcript yet", func(t *testing.T) {
		t.Run("and the record says busy", func(t *testing.T) {
			session := build(t, []claude.SessionRecord{record(sidA, claude.KindInteractive, claude.StatusBusy)}, nil, nil, alive)[0]

			t.Run("it should be working", func(t *testing.T) {
				assert.Equal(t, state.Working, session.State)
			})

			t.Run("it should take the title from the record", func(t *testing.T) {
				assert.Equal(t, "record name", session.Title)
			})

			t.Run("it should have no context measurement", func(t *testing.T) {
				assert.Zero(t, session.ContextPercent)
			})
		})

		t.Run("and the record says idle", func(t *testing.T) {
			session := build(t, []claude.SessionRecord{record(sidA, claude.KindInteractive, claude.StatusIdle)}, nil, nil, alive)[0]

			t.Run("it should be empty", func(t *testing.T) {
				assert.Equal(t, state.Empty, session.State)
			})
		})

		t.Run("and the record has no status", func(t *testing.T) {
			session := build(t, []claude.SessionRecord{record(sidA, claude.KindBackground, "")}, nil, nil, alive)[0]

			t.Run("it should be empty", func(t *testing.T) {
				assert.Equal(t, state.Empty, session.State)
			})
		})
	})

	t.Run("when the transcript has no model id", func(t *testing.T) {
		tr := transcript(sidA, claude.TurnAwaitingUser)
		tr.ModelID = ""
		tr.ContextTokens = 50_000

		t.Run("and cost-state names a [1m] variant of the model", func(t *testing.T) {
			tr.Cost.Models = map[string]claude.ModelCost{"claude-opus-5[1m]": {USD: 8.65}}
			session := build(t, []claude.SessionRecord{record(sidA, claude.KindInteractive, claude.StatusIdle)},
				[]claude.Transcript{tr}, nil, alive)[0]

			t.Run("it should measure context against a 1M window", func(t *testing.T) {
				assert.InDelta(t, 5.0, session.ContextPercent, 1e-9)
			})
		})

		t.Run("and nothing names a [1m] variant", func(t *testing.T) {
			tr.Cost.Models = nil
			session := build(t, []claude.SessionRecord{record(sidA, claude.KindInteractive, claude.StatusIdle)},
				[]claude.Transcript{tr}, nil, alive)[0]

			t.Run("it should measure context against a 200k window", func(t *testing.T) {
				assert.InDelta(t, 25.0, session.ContextPercent, 1e-9)
			})

			t.Run("it should fall back to the short model name", func(t *testing.T) {
				assert.Equal(t, "claude-opus-5", session.Model)
			})
		})
	})

	t.Run("when the session has subagents", func(t *testing.T) {
		spawned := claude.Subagent{ToolUseID: "toolu_A", AgentType: "Explore", Transcript: claude.Transcript{
			Usage: claude.Usage{Output: 400, Messages: 1}, Tail: claude.Tail{Turn: claude.TurnWorking},
		}}
		finished := claude.Subagent{ToolUseID: "toolu_B", Transcript: claude.Transcript{
			Usage: claude.Usage{Output: 200, Messages: 1}, Tail: claude.Tail{Turn: claude.TurnAwaitingUser},
		}}
		abandoned := claude.Subagent{ToolUseID: "toolu_C", Transcript: claude.Transcript{
			Usage: claude.Usage{Output: 600, Messages: 1}, Tail: claude.Tail{Turn: claude.TurnWorking},
		}}
		parent := transcript(sidA, claude.TurnWorking)
		parent.Tail.PendingTools = []string{"Agent"}
		parent.Tail.PendingToolIDs = []string{"toolu_A"}
		session := build(t, []claude.SessionRecord{record(sidA, claude.KindInteractive, claude.StatusBusy)},
			[]claude.Transcript{parent},
			map[string][]claude.Subagent{sidA: {spawned, finished, abandoned}}, alive)[0]

		t.Run("it should add subagent tokens to the usage", func(t *testing.T) {
			assert.Equal(t, int64(1800), session.Usage.Output)
		})

		t.Run("it should count in flight only the subagent the parent is still waiting on", func(t *testing.T) {
			assert.Equal(t, 1, session.SubagentsInFlight)
		})

		t.Run("it should count all subagents", func(t *testing.T) {
			assert.Equal(t, 3, session.Subagents)
		})

		t.Run("it should name the subagents in flight", func(t *testing.T) {
			assert.Equal(t, []string{"Explore"}, session.SubagentNames)
		})
	})

	// Item 89: an agent launched in the background is in flight while its
	// own transcript is still mid-turn, the session is alive, and it has
	// written something within BackgroundAgentQuiet.
	t.Run("when the session launched agents in the background", func(t *testing.T) {
		agent := func(id string, turn claude.Turn, last time.Time) claude.Subagent {
			return claude.Subagent{ToolUseID: id, AgentType: "Explore", Transcript: claude.Transcript{
				CWD: "/p/b", LastAt: last, Tail: claude.Tail{Turn: turn},
			}}
		}
		parent := transcript(sidA, claude.TurnAwaitingUser)
		parent.Background = map[string]bool{"toolu_A": true}
		session := func(sub claude.Subagent, probes state.Probes) state.Session {
			return build(t, []claude.SessionRecord{record(sidA, claude.KindInteractive, claude.StatusIdle)},
				[]claude.Transcript{parent}, map[string][]claude.Subagent{sidA: {sub}}, probes)[0]
		}
		fresh := now.Add(-time.Minute)

		t.Run("and one's own last turn is still going", func(t *testing.T) {
			s := session(agent("toolu_A", claude.TurnWorking, fresh), alive)

			t.Run("it should count it in flight", func(t *testing.T) {
				assert.Equal(t, 1, s.SubagentsInFlight)
			})

			t.Run("it should name it and record where it works", func(t *testing.T) {
				assert.Equal(t, [2][]string{{"Explore"}, {"/p/b"}}, [2][]string{s.SubagentNames, s.SubagentDirs})
			})
		})

		t.Run("and one's last turn has finished", func(t *testing.T) {
			s := session(agent("toolu_A", claude.TurnAwaitingUser, fresh), alive)

			t.Run("it should not count it in flight", func(t *testing.T) {
				assert.Zero(t, s.SubagentsInFlight)
			})
		})

		t.Run("and the session's process is gone", func(t *testing.T) {
			s := session(agent("toolu_A", claude.TurnWorking, fresh), dead)

			t.Run("it should not count it in flight", func(t *testing.T) {
				assert.Zero(t, s.SubagentsInFlight)
			})
		})

		t.Run("and one has been quiet for longer than BackgroundAgentQuiet", func(t *testing.T) {
			s := session(agent("toolu_A", claude.TurnWorking, now.Add(-state.BackgroundAgentQuiet-time.Second)), alive)

			t.Run("it should not count it in flight", func(t *testing.T) {
				assert.Zero(t, s.SubagentsInFlight)
			})
		})

		t.Run("and a foreground one was abandoned mid-turn", func(t *testing.T) {
			s := session(agent("toolu_Z", claude.TurnWorking, fresh), alive)

			t.Run("it should not count it in flight", func(t *testing.T) {
				assert.Zero(t, s.SubagentsInFlight)
			})
		})
	})

	// Item 89: the main thread's tool calls, for the building's pulses.
	t.Run("when the main thread has made tool calls", func(t *testing.T) {
		tr := transcript(sidA, claude.TurnWorking)
		tr.ToolCalls = 7
		session := build(t, []claude.SessionRecord{record(sidA, claude.KindInteractive, claude.StatusBusy)},
			[]claude.Transcript{tr}, nil, alive)[0]

		t.Run("it should carry the count", func(t *testing.T) {
			assert.Equal(t, 7, session.ToolCalls)
		})
	})

	// Item 89: where each subagent in flight works, for its drone.
	t.Run("when subagents are in flight", func(t *testing.T) {
		away := claude.Subagent{ToolUseID: "toolu_A", Transcript: claude.Transcript{
			CWD: "/p/a", Touches: map[string]int{"/p/b/x.go": 1, "/p/c/y.go": 3}, Tail: claude.Tail{Turn: claude.TurnWorking},
		}}
		home := claude.Subagent{ToolUseID: "toolu_B", Transcript: claude.Transcript{
			CWD: "/p/a/sub", Tail: claude.Tail{Turn: claude.TurnWorking},
		}}
		parent := transcript(sidA, claude.TurnWorking)
		parent.Tail.PendingToolIDs = []string{"toolu_A", "toolu_B"}
		session := build(t, []claude.SessionRecord{record(sidA, claude.KindInteractive, claude.StatusBusy)},
			[]claude.Transcript{parent},
			map[string][]claude.Subagent{sidA: {away, home}}, alive)[0]

		t.Run("it should name the file each touched most outside its own directory, or else its directory", func(t *testing.T) {
			assert.Equal(t, []string{"/p/c/y.go", "/p/a/sub"}, session.SubagentDirs)
		})
	})

	t.Run("when two sessions share a project", func(t *testing.T) {
		older := record(sidB, claude.KindInteractive, claude.StatusIdle)
		older.StartedAt = started.Add(-time.Hour)
		sessions := build(t, []claude.SessionRecord{record(sidA, claude.KindInteractive, claude.StatusIdle), older}, nil, nil, alive)

		t.Run("it should order them by start time", func(t *testing.T) {
			assert.Equal(t, []string{sidB, sidA}, []string{sessions[0].ID, sessions[1].ID})
		})
	})

	t.Run("when the transcript span is shorter than a minute", func(t *testing.T) {
		tr := transcript(sidA, claude.TurnWorking)
		tr.LastAt = tr.FirstAt.Add(time.Second)
		session := build(t, []claude.SessionRecord{record(sidA, claude.KindInteractive, claude.StatusBusy)},
			[]claude.Transcript{tr}, nil, alive)[0]

		t.Run("it should rate tokens over a full minute", func(t *testing.T) {
			assert.InDelta(t, 3600.0*60, session.TokensPerHour, 1e-9)
		})
	})
}

func TestParkedSessions(t *testing.T) {
	parked := func(partial bool, modelID string, cost map[string]claude.ModelCost) state.Session {
		tr := transcript(sidA, claude.TurnAwaitingUser)
		tr.Partial, tr.ModelID, tr.Cost.Models = partial, modelID, cost
		tr.ContextTokens = 400_000
		sessions := state.Build(state.Sources{Stopped: []claude.Transcript{tr}}, alive, now)
		require.Len(t, sessions, 1)
		return sessions[0]
	}

	t.Run("when a parked transcript was read in full for a known family", func(t *testing.T) {
		s := parked(false, "", nil)

		t.Run("it should be parked and not alive", func(t *testing.T) {
			assert.Equal(t, state.Resting, s.State)
			assert.False(t, s.Alive)
		})

		t.Run("it should measure context against the large window because 400k cannot fit in 200k", func(t *testing.T) {
			assert.InDelta(t, 40, s.ContextPercent, 1e-9)
		})
	})

	t.Run("when a parked transcript has hourly buckets going back a fortnight", func(t *testing.T) {
		tr := transcript(sidA, claude.TurnAwaitingUser)
		tr.Hourly = map[int64]claude.Usage{}
		for h := int64(0); h < 14*24; h++ {
			tr.Hourly[now.Unix()/3600-h] = claude.Usage{Output: 1}
		}
		sessions := state.Build(state.Sources{Stopped: []claude.Transcript{tr}}, alive, now)
		require.Len(t, sessions, 1)

		t.Run("it should carry only the newest week of them", func(t *testing.T) {
			assert.Len(t, sessions[0].Hourly, state.HourlyKeep)
		})

		t.Run("it should keep the newest bucket", func(t *testing.T) {
			_, ok := sessions[0].Hourly[now.Unix()/3600]
			assert.True(t, ok)
		})
	})

	t.Run("when a parked transcript is a partial read that names its model id", func(t *testing.T) {
		s := parked(true, "claude-opus-5[1m]", nil)

		t.Run("it should measure context against that window", func(t *testing.T) {
			assert.InDelta(t, 40, s.ContextPercent, 1e-9)
		})
	})

	t.Run("when a parked transcript is on an unknown model and never proved a window", func(t *testing.T) {
		tr := transcript(sidA, claude.TurnAwaitingUser)
		tr.Partial, tr.Model, tr.ModelID, tr.Cost.Models = true, "claude-fable-5-1", "claude-fable-5-1", nil
		tr.ContextTokens = 150_000
		sessions := state.Build(state.Sources{Stopped: []claude.Transcript{tr}}, alive, now)
		require.Len(t, sessions, 1)

		t.Run("it should not guess a percentage", func(t *testing.T) {
			assert.Zero(t, sessions[0].ContextPercent)
		})
	})
}

func TestAggregates(t *testing.T) {
	withCalls := func(id string, mcp, skills map[string]int, prs []claude.PR, errs int) claude.Transcript {
		tr := transcript(id, claude.TurnAwaitingUser)
		tr.MCPCalls, tr.SkillCalls, tr.PRs, tr.APIErrors = mcp, skills, prs, errs
		tr.LastAt = now.Add(-time.Hour)
		tr.Recap = claude.Recap{Text: "recap of " + id, At: now.Add(-2 * time.Hour), PromptsSince: 3}
		tr.LastPrompt = "last of " + id
		return tr
	}
	a := withCalls(sidA, map[string]int{"atlassian": 3, "langfuse": 1}, map[string]int{"amberPylon:umberEstuary": 2}, []claude.PR{{Number: 7, URL: "u7"}}, 2)
	b := withCalls(sidB, map[string]int{"atlassian": 5}, map[string]int{"amberPylon:umberEstuary": 1, "git-worktrees": 4}, nil, 0)
	mcp := claude.MCPConfig{Global: []claude.MCPServer{{Name: "datadog-mcp", Type: "http"}},
		Projects: map[string][]claude.MCPServer{"/p": {{Name: "langfuse", Type: "stdio"}}}}
	sessions := state.Build(state.Sources{Stopped: []claude.Transcript{a, b}, MCP: mcp}, alive, now)

	t.Run("when sessions attribute MCP calls and the config names servers", func(t *testing.T) {
		servers := state.Servers(sessions, mcp)

		t.Run("it should list configured and used servers together, sorted by name", func(t *testing.T) {
			names := []string{}
			for _, s := range servers {
				names = append(names, s.Name)
			}
			assert.Equal(t, []string{"atlassian", "datadog-mcp", "langfuse"}, names)
		})

		t.Run("it should total calls and count sessions for a used server", func(t *testing.T) {
			assert.Equal(t, state.Server{Name: "atlassian", Calls: 8, Sessions: 2}, servers[0])
		})

		t.Run("it should mark a configured but unused server", func(t *testing.T) {
			assert.Equal(t, state.Server{Name: "datadog-mcp", Type: "http", Configured: true}, servers[1])
		})

		t.Run("it should mark a project server that was also used", func(t *testing.T) {
			assert.Equal(t, state.Server{Name: "langfuse", Type: "stdio", Configured: true, Calls: 1, Sessions: 1}, servers[2])
		})
	})

	t.Run("when sessions attribute skills", func(t *testing.T) {
		skills := state.Skills(sessions)

		t.Run("it should rank them by calls", func(t *testing.T) {
			assert.Equal(t, []state.Skill{{Name: "git-worktrees", Calls: 4, Sessions: 1}, {Name: "amberPylon:umberEstuary", Calls: 3, Sessions: 2}}, skills)
		})
	})

	t.Run("when a session carries attribution", func(t *testing.T) {
		var got state.Session
		for _, s := range sessions {
			if s.ID == sidA {
				got = s
			}
		}

		t.Run("it should expose its PRs", func(t *testing.T) {
			assert.Equal(t, []claude.PR{{Number: 7, URL: "u7"}}, got.PRs)
		})

		t.Run("it should expose its API errors", func(t *testing.T) {
			assert.Equal(t, 2, got.APIErrors)
		})

		t.Run("it should expose its recap", func(t *testing.T) {
			assert.Equal(t, claude.Recap{Text: "recap of " + sidA, At: now.Add(-2 * time.Hour), PromptsSince: 3}, got.Recap)
		})

		t.Run("it should expose its last prompt", func(t *testing.T) {
			assert.Equal(t, "last of "+sidA, got.LastPrompt)
		})
	})

	t.Run("when power is summed over the last day", func(t *testing.T) {
		old := transcript("old", claude.TurnAwaitingUser)
		old.LastAt = now.Add(-48 * time.Hour)
		old.Cost.Models = map[string]claude.ModelCost{"claude-opus-5[1m]": {USD: 100, Usage: claude.Usage{Output: 1000}}}
		recent := transcript("recent", claude.TurnAwaitingUser)
		recent.LastAt = now.Add(-time.Hour)
		recent.Cost.Models = map[string]claude.ModelCost{
			"claude-opus-5[1m]": {USD: 10, Usage: claude.Usage{Input: 100, Output: 100, CacheRead: 700, CacheCreate: 100}},
		}
		hour := func(d time.Duration) int64 { return now.Add(d).Unix() / 3600 }
		recent.Hourly = map[int64]claude.Usage{
			hour(-30 * time.Hour): {Input: 50, Output: 50, CacheRead: 350, CacheCreate: 50},
			hour(-2 * time.Hour):  {Input: 50, Output: 50, CacheRead: 350, CacheCreate: 50},
		}
		power := state.PowerSince([]claude.Transcript{old, recent}, now.Add(-state.PowerWindow))

		t.Run("it should count only the fresh tokens, input and cache writes, spent inside the window", func(t *testing.T) {
			assert.Equal(t, int64(50+50), power.Fresh)
		})

		t.Run("it should count only the cache reads inside the window", func(t *testing.T) {
			assert.Equal(t, int64(350), power.Cached)
		})

		t.Run("it should count the output inside the window on its own", func(t *testing.T) {
			assert.Equal(t, int64(50), power.Output)
		})

		t.Run("it should leave out sessions older than the window", func(t *testing.T) {
			_, hasOld := power.ByModel["claude-haiku-4-5-20251001"]
			assert.False(t, hasOld)
		})

		t.Run("it should keep the lifetime split by model", func(t *testing.T) {
			assert.Equal(t, int64(100), power.ByModel["claude-opus-5[1m]"].Output)
		})
	})
}

func TestRoads(t *testing.T) {
	teams := []claude.Team{{Name: "session-26c5d638", Members: []claude.TeamMember{
		{Name: "team-lead", CWD: "/home/avesta/workspaces/github/auroq"},
		{Name: "migrate-custom", CWD: "/home/avesta/workspaces/github/mCedar/mullet"},
	}}}
	sender := func(id, cwd string, msgs map[string]int) state.Session {
		return state.Session{ID: id, CWD: cwd, Team: "session-26c5d638", Messages: msgs}
	}

	t.Run("when sessions message teammates in other projects", func(t *testing.T) {
		roads := state.Roads([]state.Session{
			sender("a", "/home/avesta/workspaces/github/mCedar/mullet", map[string]int{"team-lead": 3}),
			sender("b", "/home/avesta/workspaces/github/mCedar/cinders", map[string]int{"team-lead": 1, "migrate-custom": 2}),
			sender("c", "/home/avesta/workspaces/github/auroq", map[string]int{"team-lead": 5}),
		}, teams)

		t.Run("it should draw one road per project pair, sorted", func(t *testing.T) {
			assert.Equal(t, []state.Road{
				{From: "/home/avesta/workspaces/github/mCedar/cinders", To: "/home/avesta/workspaces/github/auroq", Messages: 1, Sessions: 1},
				{From: "/home/avesta/workspaces/github/mCedar/cinders", To: "/home/avesta/workspaces/github/mCedar/mullet", Messages: 2, Sessions: 1},
				{From: "/home/avesta/workspaces/github/mCedar/mullet", To: "/home/avesta/workspaces/github/auroq", Messages: 3, Sessions: 1},
			}, roads)
		})

		t.Run("it should not draw a road from a project to itself", func(t *testing.T) {
			for _, r := range roads {
				assert.NotEqual(t, r.From, r.To)
			}
		})
	})

	t.Run("when a session edits files in another session's project", func(t *testing.T) {
		editor := state.Session{ID: "e", CWD: "/home/avesta/workspaces/github/mCedar/mullet", Touches: map[string]int{
			"/home/avesta/workspaces/github/auroq/botropolis/pkg/city/city.go": 2,
			"/home/avesta/workspaces/github/auroq/botropolis/README.md":        1,
			"/home/avesta/.claude/settings.json":                               5,
		}}
		other := state.Session{ID: "o", CWD: "/home/avesta/workspaces/github/auroq/botropolis"}
		roads := state.Roads([]state.Session{editor, other}, nil)

		t.Run("it should draw a road carrying the file touches", func(t *testing.T) {
			require.Len(t, roads, 1)
			assert.Equal(t, state.Road{From: editor.CWD, To: other.CWD, Files: 3, Sessions: 1, Paths: roads[0].Paths}, roads[0])
		})

		t.Run("it should name the busiest files first", func(t *testing.T) {
			assert.Equal(t, []string{
				"/home/avesta/workspaces/github/auroq/botropolis/pkg/city/city.go",
				"/home/avesta/workspaces/github/auroq/botropolis/README.md",
			}, roads[0].Paths)
		})
	})

	t.Run("when a session sits in a directory that holds other sessions' projects", func(t *testing.T) {
		home := state.Session{ID: "h", CWD: "/home/avesta"}
		project := state.Session{ID: "p", CWD: "/home/avesta/workspaces/github/auroq/botropolis", Touches: map[string]int{
			"/home/avesta/workspaces/aur/botropolis-git/PKGBUILD": 3,
		}}
		roads := state.Roads([]state.Session{home, project}, nil)

		t.Run("it should not draw a road to that hub", func(t *testing.T) {
			assert.Empty(t, roads)
		})
	})

	t.Run("when the only session inside a project is in its worktree", func(t *testing.T) {
		project := state.Session{ID: "p", CWD: "/home/avesta/workspaces/github/auroq/botropolis"}
		worktree := state.Session{ID: "w", CWD: "/home/avesta/workspaces/github/auroq/botropolis/.claude/worktrees/x"}
		editor := state.Session{ID: "e", CWD: "/home/avesta/workspaces/github/mCedar/mullet", Touches: map[string]int{
			"/home/avesta/workspaces/github/auroq/botropolis/README.md": 1,
		}}
		roads := state.Roads([]state.Session{project, worktree, editor}, nil)

		t.Run("it should still draw the road to the project", func(t *testing.T) {
			assert.Equal(t, []state.Road{{From: editor.CWD, To: project.CWD, Files: 1, Sessions: 1,
				Paths: []string{"/home/avesta/workspaces/github/auroq/botropolis/README.md"}}}, roads)
		})
	})

	t.Run("when a session both messages and edits another project", func(t *testing.T) {
		worker := sender("w", "/home/avesta/workspaces/github/mCedar/mullet", map[string]int{"team-lead": 4})
		worker.Team = "session-37d10079"
		worker.Touches = map[string]int{"/home/avesta/workspaces/github/auroq/x.go": 1}
		lead := state.Session{ID: "37d10079-0000-0000-0000-000000000000", CWD: "/home/avesta/workspaces/github/auroq"}
		roads := state.Roads([]state.Session{lead, worker}, nil)

		t.Run("it should count the session once", func(t *testing.T) {
			assert.Equal(t, []state.Road{{From: worker.CWD, To: lead.CWD, Messages: 4, Files: 1, Sessions: 1,
				Paths: []string{"/home/avesta/workspaces/github/auroq/x.go"}}}, roads)
		})
	})

	t.Run("when the team config is gone but the lead session is known", func(t *testing.T) {
		lead := state.Session{ID: "37d10079-0000-0000-0000-000000000000", CWD: "/home/avesta/workspaces/github/auroq"}
		worker := sender("w", "/home/avesta/workspaces/github/mCedar/mullet", map[string]int{"team-lead": 4})
		worker.Team = "session-37d10079"
		roads := state.Roads([]state.Session{lead, worker}, nil)

		t.Run("it should route team-lead to the lead session's project", func(t *testing.T) {
			assert.Equal(t, []state.Road{{From: "/home/avesta/workspaces/github/mCedar/mullet", To: "/home/avesta/workspaces/github/auroq", Messages: 4, Sessions: 1}}, roads)
		})
	})

	// Item 253: Claude Code 2.1.310 names a team after neither session,
	// and the lead's own lines carry no team; its Agent result does.
	t.Run("when a lead's lines name no team but it leads one", func(t *testing.T) {
		const team, leadDir, mateDir = "session-2aa29be8", "/home/demo/src/tidepool", "/home/demo/src/beacon"
		lead := state.Session{ID: "3cab0685-0000-0000-0000-000000000000", CWD: leadDir, Team: team, Leads: team,
			Messages: map[string]int{"reader": 2}}
		mate := state.Session{ID: "562b571b-0000-0000-0000-000000000000", CWD: mateDir, Team: team, Agent: "reader",
			Messages: map[string]int{"team-lead": 3}}
		roads := state.Roads([]state.Session{lead, mate}, nil)

		t.Run("it should carry the lead's messages to its teammate's district, and the teammate's back, with no team config", func(t *testing.T) {
			assert.Equal(t, []state.Road{{From: mateDir, To: leadDir, Messages: 3, BackMessages: 2, Sessions: 2}}, roads)
		})
	})

	t.Run("when a team is named after its lead's id and no session leads it by an Agent result", func(t *testing.T) {
		lead := state.Session{ID: "37d10079-0000-0000-0000-000000000000", CWD: "/home/avesta/workspaces/github/auroq",
			Leads: "session-2aa29be8"}
		worker := sender("w", "/home/avesta/workspaces/github/mCedar/mullet", map[string]int{"team-lead": 4})
		worker.Team = "session-37d10079"
		roads := state.Roads([]state.Session{lead, worker}, nil)

		t.Run("it should still resolve team-lead by the name's prefix", func(t *testing.T) {
			assert.Equal(t, []state.Road{{From: worker.CWD, To: lead.CWD, Messages: 4, Sessions: 1}}, roads)
		})
	})

	t.Run("when both a lead by Agent result and a session the name's prefix matches could be team-lead", func(t *testing.T) {
		named := state.Session{ID: "2aa29be8-0000-0000-0000-000000000000", CWD: "/home/demo/src/harbour"}
		lead := state.Session{ID: "3cab0685-0000-0000-0000-000000000000", CWD: "/home/demo/src/tidepool", Leads: "session-2aa29be8"}
		mate := state.Session{ID: "562b571b-0000-0000-0000-000000000000", CWD: "/home/demo/src/beacon",
			Team: "session-2aa29be8", Agent: "reader", Messages: map[string]int{"team-lead": 3}}
		roads := state.Roads([]state.Session{named, lead, mate}, nil)

		t.Run("it should route to the session that leads it", func(t *testing.T) {
			assert.Equal(t, []state.Road{{From: mate.CWD, To: lead.CWD, Messages: 3, Sessions: 1}}, roads)
		})
	})

	t.Run("when the recipient is not a known teammate", func(t *testing.T) {
		roads := state.Roads([]state.Session{sender("a", "/p", map[string]int{"nobody": 4})}, teams)

		t.Run("it should draw nothing", func(t *testing.T) {
			assert.Empty(t, roads)
		})
	})

	t.Run("when the session belongs to no team", func(t *testing.T) {
		s := sender("a", "/p", map[string]int{"team-lead": 4})
		s.Team = ""
		roads := state.Roads([]state.Session{s}, teams)

		t.Run("it should draw nothing", func(t *testing.T) {
			assert.Empty(t, roads)
		})
	})

	// Item 89: a message one ordinary session sends another counts as a
	// message on their road, read from the recipient's side.
	t.Run("when sessions message each other directly", func(t *testing.T) {
		const from, to = "/home/demo/src/tidepool", "/home/demo/src/beacon"
		receiving := func(m claude.PeerMessage) state.Session {
			return state.Session{ID: "r", CWD: to, PeerMessages: []claude.PeerMessage{m}}
		}
		sending := state.Session{ID: "s", CWD: from, PID: 42, Name: "item 89"}

		t.Run("and the recipient names the sender's process", func(t *testing.T) {
			roads := state.Roads([]state.Session{sending, receiving(claude.PeerMessage{ID: "m", FromPID: 42})}, nil)

			t.Run("it should add one message on the road from the sender's project to the recipient's", func(t *testing.T) {
				assert.Equal(t, []state.Road{{From: from, To: to, Messages: 1, Sessions: 1}}, roads)
			})
		})

		t.Run("and the sender's process is gone but one session has its name", func(t *testing.T) {
			gone := sending
			gone.PID = 0
			roads := state.Roads([]state.Session{gone, receiving(claude.PeerMessage{ID: "m", FromPID: 42, FromName: "item 89"})}, nil)

			t.Run("it should resolve the sender by name", func(t *testing.T) {
				assert.Len(t, roads, 1)
			})
		})

		t.Run("and two sessions share the sender's name", func(t *testing.T) {
			gone, twin := sending, sending
			gone.PID, twin.PID, twin.ID = 0, 0, "s2"
			roads := state.Roads([]state.Session{gone, twin, receiving(claude.PeerMessage{ID: "m", FromName: "item 89"})}, nil)

			t.Run("it should add nothing", func(t *testing.T) {
				assert.Empty(t, roads)
			})
		})

		t.Run("and the sender cannot be resolved", func(t *testing.T) {
			roads := state.Roads([]state.Session{sending, receiving(claude.PeerMessage{ID: "m", FromPID: 7, FromName: "nobody"})}, nil)

			t.Run("it should add nothing", func(t *testing.T) {
				assert.Empty(t, roads)
			})
		})

		t.Run("and the sender is in the same project", func(t *testing.T) {
			near := sending
			near.CWD = to
			roads := state.Roads([]state.Session{near, receiving(claude.PeerMessage{ID: "m", FromPID: 42})}, nil)

			t.Run("it should add no road", func(t *testing.T) {
				assert.Empty(t, roads)
			})
		})
	})
}

func TestContextWindowInference(t *testing.T) {
	live := func(model string, contextTokens, maxContext int64) state.Session {
		tr := transcript(sidA, claude.TurnAwaitingUser)
		tr.Model, tr.ModelID, tr.Cost.Models = model, model, nil
		tr.ContextTokens, tr.MaxContext = contextTokens, maxContext
		sessions := state.Build(state.Sources{
			Records:     []claude.SessionRecord{record(sidA, claude.KindInteractive, claude.StatusIdle)},
			Transcripts: []claude.Transcript{tr},
		}, alive, now)
		require.Len(t, sessions, 1)
		return sessions[0]
	}

	t.Run("when a session on an unknown model has held more than 200k of context", func(t *testing.T) {
		s := live("claude-fable-5-1", 925_000, 929_000)

		t.Run("it should measure against 1M because context cannot exceed the window", func(t *testing.T) {
			assert.Equal(t, int64(1_000_000), s.ContextWindow)
			assert.InDelta(t, 92.5, s.ContextPercent, 1e-9)
		})

		t.Run("it should say the window is known", func(t *testing.T) {
			assert.True(t, s.ContextKnown)
		})
	})

	t.Run("when a session on an unknown model has stayed small", func(t *testing.T) {
		s := live("claude-fable-5-1", 150_000, 150_000)

		t.Run("it should not guess a percentage", func(t *testing.T) {
			assert.Zero(t, s.ContextPercent)
		})

		t.Run("it should say the window is unknown", func(t *testing.T) {
			assert.False(t, s.ContextKnown)
		})
	})

	t.Run("when a session on a known family has stayed small", func(t *testing.T) {
		s := live("claude-opus-5", 150_000, 150_000)

		t.Run("it should measure against the default window", func(t *testing.T) {
			assert.InDelta(t, 75, s.ContextPercent, 1e-9)
		})

		t.Run("it should say the window is known", func(t *testing.T) {
			assert.True(t, s.ContextKnown)
		})
	})

	t.Run("when a session on a known family once exceeded the default window", func(t *testing.T) {
		s := live("claude-sonnet-5", 100_000, 300_000)

		t.Run("it should measure against 1M from then on", func(t *testing.T) {
			assert.InDelta(t, 10, s.ContextPercent, 1e-9)
		})
	})
}

func TestRoadPaths(t *testing.T) {
	t.Run("when a session touches more files than a road can list", func(t *testing.T) {
		editor := state.Session{ID: "e", CWD: "/p/a", Touches: map[string]int{}}
		for i := 0; i < state.MaxRoadPaths+3; i++ {
			editor.Touches[fmt.Sprintf("/p/b/f%02d.go", i)] = i + 1
		}
		roads := state.Roads([]state.Session{editor, {ID: "o", CWD: "/p/b"}}, nil)

		t.Run("it should keep only the busiest", func(t *testing.T) {
			require.Len(t, roads, 1)
			assert.Equal(t, []string{"/p/b/f07.go", "/p/b/f06.go", "/p/b/f05.go", "/p/b/f04.go", "/p/b/f03.go"}, roads[0].Paths)
		})
	})
}

func TestPowerHitRatio(t *testing.T) {
	t.Run("when most prompt tokens were cache reads", func(t *testing.T) {
		t.Run("it should report their share", func(t *testing.T) {
			assert.InDelta(t, 0.9, state.Power{Fresh: 100, Cached: 900}.HitRatio(), 1e-9)
		})
	})

	t.Run("when no tokens were read", func(t *testing.T) {
		t.Run("it should report zero", func(t *testing.T) {
			assert.Zero(t, state.Power{}.HitRatio())
		})
	})
}

func TestSessionDoing(t *testing.T) {
	cwd := "/home/avesta/workspaces/github/auroq/botropolis"

	t.Run("when the tool works on a file inside the project", func(t *testing.T) {
		t.Run("it should show the path relative to the project", func(t *testing.T) {
			assert.Equal(t, "Edit pkg/city/city.go", state.Session{CWD: cwd, Tool: "Edit", Subject: cwd + "/pkg/city/city.go"}.Doing())
		})
	})

	t.Run("when the tool works on something outside the project", func(t *testing.T) {
		t.Run("it should show it as is", func(t *testing.T) {
			assert.Equal(t, "Bash make test", state.Session{CWD: cwd, Tool: "Bash", Subject: "make test"}.Doing())
		})
	})

	t.Run("when the tool has no subject", func(t *testing.T) {
		t.Run("it should show the tool alone", func(t *testing.T) {
			assert.Equal(t, "Agent", state.Session{CWD: cwd, Tool: "Agent"}.Doing())
		})
	})

	t.Run("when no tool is running", func(t *testing.T) {
		t.Run("it should be empty", func(t *testing.T) {
			assert.Empty(t, state.Session{CWD: cwd, Subject: "stale"}.Doing())
		})
	})
}

func TestRecordStatus(t *testing.T) {
	t.Run("when the record says idle while the tail says mid-turn", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindBackground, claude.StatusIdle)},
			[]claude.Transcript{transcript(sidA, claude.TurnWorking)}, nil, alive)[0]

		t.Run("it should need you", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, session.State)
		})
	})

	t.Run("when the record says idle while the tail is waiting on its watch", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindBackground, claude.StatusIdle)},
			[]claude.Transcript{transcript(sidA, claude.TurnWaiting)}, nil, alive)[0]

		t.Run("it should be waiting", func(t *testing.T) {
			assert.Equal(t, state.Waiting, session.State)
		})
	})

	t.Run("when an interactive record says busy while the tail awaits the user", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindInteractive, claude.StatusBusy)},
			[]claude.Transcript{transcript(sidA, claude.TurnAwaitingUser)}, nil, alive)[0]

		t.Run("it should be working", func(t *testing.T) {
			assert.Equal(t, state.Working, session.State)
		})
	})

	t.Run("when a background record says busy while the tail awaits the user with no terminal attached", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindBackground, claude.StatusBusy)},
			[]claude.Transcript{transcript(sidA, claude.TurnAwaitingUser)}, nil, alive)[0]

		t.Run("it should be unattended", func(t *testing.T) {
			assert.Equal(t, state.Unattended, session.State)
		})
	})

	t.Run("when the record says busy while the tail needs input", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindBackground, claude.StatusBusy)},
			[]claude.Transcript{transcript(sidA, claude.TurnNeedsInput)}, nil, alive)[0]

		t.Run("it should need you", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, session.State)
		})
	})

	t.Run("when the record says shell while the tail awaits the user", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindInteractive, claude.StatusShell)},
			[]claude.Transcript{transcript(sidA, claude.TurnAwaitingUser)}, nil, alive)[0]

		t.Run("it should be working", func(t *testing.T) {
			assert.Equal(t, state.Working, session.State)
		})

		t.Run("it should carry the record's status", func(t *testing.T) {
			assert.Equal(t, claude.StatusShell, session.RecordStatus)
		})
	})

	t.Run("when the record says a word this build has never seen while the tail awaits the user", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindInteractive, "compacting")},
			[]claude.Transcript{transcript(sidA, claude.TurnAwaitingUser)}, nil, alive)[0]

		t.Run("it should be working, since only idle means idle", func(t *testing.T) {
			assert.Equal(t, state.Working, session.State)
		})
	})

	t.Run("when the record has no status while the tail awaits the user", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindBackground, "")},
			[]claude.Transcript{transcript(sidA, claude.TurnAwaitingUser)}, nil, alive)[0]

		t.Run("it should need you", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, session.State)
		})
	})
}

func TestEmptySession(t *testing.T) {
	blank := transcript(sidA, claude.TurnUnknown)
	blank.Usage = claude.Usage{}
	blank.Tail = claude.Tail{}

	t.Run("when a live idle record's transcript holds no prompt and no reply", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindBackground, claude.StatusIdle)},
			[]claude.Transcript{blank}, nil, alive)[0]

		t.Run("it should be empty", func(t *testing.T) {
			assert.Equal(t, state.Empty, session.State)
		})
	})

	t.Run("when a live idle record's transcript holds a prompt", func(t *testing.T) {
		prompted := blank
		prompted.Tail = claude.Tail{Turn: claude.TurnWorking, Prompts: 1, LastPromptAt: started}
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindBackground, claude.StatusIdle)},
			[]claude.Transcript{prompted}, nil, alive)[0]

		t.Run("it should need you", func(t *testing.T) {
			assert.Equal(t, state.NeedsYou, session.State)
		})
	})

	t.Run("when the pid is gone and the transcript is blank", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindBackground, claude.StatusIdle)},
			[]claude.Transcript{blank}, nil, dead)[0]

		t.Run("it should be parked", func(t *testing.T) {
			assert.Equal(t, state.Resting, session.State)
		})
	})
}

func TestSoloTeams(t *testing.T) {
	teamed := func(sid, team, agent string) claude.Transcript {
		tr := transcript(sid, claude.TurnWorking)
		tr.TeamName, tr.AgentName = team, agent
		return tr
	}

	t.Run("when a session's team is named after a session and has no other member", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindBackground, claude.StatusBusy)},
			[]claude.Transcript{teamed(sidA, "session-"+sidA[:8], "review-r2")}, nil, alive)[0]

		t.Run("it should carry no team", func(t *testing.T) {
			assert.Empty(t, session.Team)
			assert.Empty(t, session.Agent)
		})
	})

	t.Run("when two sessions share a session-named team", func(t *testing.T) {
		b := record(sidB, claude.KindBackground, claude.StatusBusy)
		b.PID = 4343
		sessions := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindBackground, claude.StatusBusy), b},
			[]claude.Transcript{teamed(sidA, "session-"+sidA[:8], ""), teamed(sidB, "session-"+sidA[:8], "reviewer")}, nil, alive)

		t.Run("they should both keep it", func(t *testing.T) {
			assert.Equal(t, "session-"+sidA[:8], sessions[0].Team)
			assert.Equal(t, "session-"+sidA[:8], sessions[1].Team)
		})
	})

	// Item 253: a lead's lines name no team; its Agent results do.
	t.Run("when a lead's transcript names no team but it leads one", func(t *testing.T) {
		lead := transcript(sidA, claude.TurnWorking)
		lead.LeadsTeam = "session-2aa29be8"
		b := record(sidB, claude.KindBackground, claude.StatusBusy)
		b.PID = 4343
		sessions := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindBackground, claude.StatusBusy), b},
			[]claude.Transcript{lead, teamed(sidB, "session-2aa29be8", "reader")}, nil, alive)

		t.Run("it should be in that team", func(t *testing.T) {
			assert.Equal(t, "session-2aa29be8", sessions[0].Team)
		})

		t.Run("it should be the team's lead", func(t *testing.T) {
			assert.Equal(t, "session-2aa29be8", sessions[0].Leads)
		})

		t.Run("its teammate should keep the team", func(t *testing.T) {
			assert.Equal(t, "session-2aa29be8", sessions[1].Team)
		})
	})

	t.Run("when a solo session's team has a name of its own", func(t *testing.T) {
		session := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindBackground, claude.StatusBusy)},
			[]claude.Transcript{teamed(sidA, "review", "")}, nil, alive)[0]

		t.Run("it should keep it", func(t *testing.T) {
			assert.Equal(t, "review", session.Team)
		})
	})
}

func TestShortTitles(t *testing.T) {
	t.Run("when a live record has no name and its transcript no title", func(t *testing.T) {
		r := record(sidA, claude.KindBackground, claude.StatusBusy)
		r.Name = ""
		untitled := transcript(sidA, claude.TurnWorking)
		untitled.Title = ""
		session := build(t, []claude.SessionRecord{r}, []claude.Transcript{untitled}, nil, alive)[0]

		t.Run("it should be titled by its short id", func(t *testing.T) {
			assert.Equal(t, sidA[:8], session.Title)
		})
	})

	t.Run("when a parked transcript has no title", func(t *testing.T) {
		untitled := transcript(sidB, claude.TurnAwaitingUser)
		untitled.Title = ""
		sessions := state.Build(state.Sources{Stopped: []claude.Transcript{untitled}}, dead, now)
		require.Len(t, sessions, 1)

		t.Run("it should be titled by its short id", func(t *testing.T) {
			assert.Equal(t, sidB[:8], sessions[0].Title)
		})
	})

	t.Run("when an id is shortened", func(t *testing.T) {
		t.Run("it should keep the first eight characters and leave short ids alone", func(t *testing.T) {
			assert.Equal(t, sidA[:8], state.ShortID(sidA))
			assert.Equal(t, "abc", state.ShortID("abc"))
		})
	})
}

// Bug 54. The fence that keeps the usage probe out of the city had one
// reader and the wrong one: the loader's parked-transcript path. A
// probe that is *running* arrives through Records instead, so it became
// a live session, sailed a tug up the river and re-planned the map
// around itself — which is what Aria saw as the map refreshing weirdly
// with a fleet of tugs on it.
//
// One fact, "the probe is not a session", now has one reader: Build,
// which every session passes through whichever path it came by.
func TestBotropolisOwnRunsAreNeverSessions(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	for _, run := range []struct{ name, dir string }{
		{"the usage probe", claude.UsageProbeDir()},
		{"the summariser", claude.SummaryDir()},
	} {
		t.Run("when "+run.name+" is running right now", func(t *testing.T) {
			sessions := state.Build(state.Sources{Records: []claude.SessionRecord{
				{PID: 41, SessionID: "own-live", CWD: run.dir, StartedAt: now},
				{PID: 42, SessionID: "real", CWD: "/home/avesta/workspaces/github/auroq/botropolis", StartedAt: now},
			}}, alive, now)

			t.Run("it should not be one of the city's sessions", func(t *testing.T) {
				var ids []string
				for _, s := range sessions {
					ids = append(ids, s.ID)
				}
				assert.NotContains(t, ids, "own-live")
			})

			t.Run("it should leave the real session alone", func(t *testing.T) {
				require.Len(t, sessions, 1)
				assert.Equal(t, "real", sessions[0].ID)
			})
		})

		t.Run("when "+run.name+" has left a transcript behind", func(t *testing.T) {
			sessions := state.Build(state.Sources{Stopped: []claude.Transcript{
				{SessionID: "own-parked", Project: claude.ProjectFolder(run.dir), CWD: run.dir, LastAt: now},
			}}, alive, now)

			t.Run("it should not be a resting session either", func(t *testing.T) {
				assert.Empty(t, sessions)
			})
		})
	}
}

func TestConnections(t *testing.T) {
	const (
		mullet  = "/home/avesta/workspaces/github/mCedar/mullet"
		cinders = "/home/avesta/workspaces/github/mCedar/cinders"
		auroq   = "/home/avesta/workspaces/github/auroq"
	)
	touching := func(id, cwd string, st state.State, paths ...string) state.Session {
		s := state.Session{ID: id, CWD: cwd, State: st, Touches: map[string]int{}}
		for _, p := range paths {
			s.Touches[p]++
		}
		return s
	}

	t.Run("when A crosses to B and B crosses back to A", func(t *testing.T) {
		roads := state.Roads([]state.Session{
			touching("a", mullet, state.NeedsYou, cinders+"/main.go"),
			touching("b", cinders, state.NeedsYou, mullet+"/x.go", mullet+"/y.go"),
		}, nil)

		t.Run("it should make one road for the pair", func(t *testing.T) {
			require.Len(t, roads, 1)
		})

		t.Run("it should keep the first direction's counts", func(t *testing.T) {
			assert.Equal(t, 2, roads[0].Files)
		})

		t.Run("it should keep the other direction's counts", func(t *testing.T) {
			assert.Equal(t, 1, roads[0].BackFiles)
		})

		t.Run("it should count both travellers", func(t *testing.T) {
			assert.Equal(t, 2, roads[0].Sessions)
		})
	})

	for _, tc := range []struct {
		scenario string
		states   []state.State
		active   int
	}{
		{"and one traveller is working", []state.State{state.Working, state.NeedsYou}, 1},
		{"and three travellers are working", []state.State{state.Working, state.Working, state.Working}, 3},
		{"and every traveller needs you, waits or is parked", []state.State{state.NeedsYou, state.Resting, state.Waiting}, 0},
	} {
		t.Run("when sessions cross between two projects "+tc.scenario, func(t *testing.T) {
			var sessions []state.Session
			for i, st := range tc.states {
				sessions = append(sessions, touching(fmt.Sprint("s", i), mullet, st, cinders+"/main.go"))
			}
			sessions = append(sessions, state.Session{ID: "c", CWD: cinders, State: state.NeedsYou})
			roads := state.Roads(sessions, nil)
			require.Len(t, roads, 1)

			t.Run(fmt.Sprintf("it should count %d active", tc.active), func(t *testing.T) {
				assert.Equal(t, tc.active, roads[0].Active)
			})
		})
	}

	t.Run("when a session's subagent works in another project", func(t *testing.T) {
		sub := claude.Subagent{Transcript: claude.Transcript{CWD: auroq + "/botropolis",
			FirstAt: time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC), LastAt: time.Date(2026, 10, 16, 9, 30, 0, 0, time.UTC),
			Touches: map[string]int{mullet + "/third.go": 2, cinders + "/own.go": 1}}}
		sessions := build(t,
			[]claude.SessionRecord{record(sidA, claude.KindBackground, claude.StatusIdle)},
			[]claude.Transcript{transcript(sidA, claude.TurnWaiting)},
			map[string][]claude.Subagent{sidA: {sub}}, alive)
		parent := sessions[0]
		other := state.Session{ID: "o", CWD: auroq + "/botropolis"}
		third := state.Session{ID: "m", CWD: mullet}
		to := map[string]state.Road{}
		for _, r := range state.Roads([]state.Session{parent, other, third}, nil) {
			to[r.To] = r
		}

		t.Run("it should connect the parent's project to where the subagent worked", func(t *testing.T) {
			assert.Equal(t, 1, to[other.CWD].Visits)
		})

		t.Run("it should keep when the subagent worked there (item 90)", func(t *testing.T) {
			assert.Equal(t, state.Span{From: sub.FirstAt, To: sub.LastAt}, parent.VisitSpans[other.CWD])
		})

		t.Run("and its subagent touches a file in a third project", func(t *testing.T) {
			t.Run("it should connect the parent's project to the third", func(t *testing.T) {
				assert.Equal(t, 2, to[mullet].Files)
			})

			t.Run("it should not count a file in the parent's own project", func(t *testing.T) {
				assert.NotContains(t, parent.Touches, cinders+"/own.go")
			})
		})
	})
}
