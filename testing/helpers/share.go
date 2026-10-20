package helpers

import (
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// ShareSecrets are the words in ShareSnapshot that identify the person:
// each is unique, so a sharing test can look for it in whatever was drawn
// (item 168). A word that appears in a path is listed by the folder name.
var ShareSecrets = []string{
	"Qzhomeowner", "Qzalphaproject", "Qzbetaproject",
	"Qzalphatitle", "Qzbetatitle", "Qzgammatitle", "Qzsessionname",
	"Qzbranchname", "Qzlastprompt", "Qzrecaptext", "Qznotetext", "Qzsubjecttext",
	"Qzorderline", "Qzagentname", "Qzsubagentname", "Qzteamname", "Qzmembername",
	"Qzgroupname", "Qzgroupshort", "Qzgroupwhy", "Qzgroupreason", "Qzgroupword",
	"Qzjoinedwhy", "Qzjournalnext", "Qzservername", "Qzskillname", "Qzrepository",
	"Qztouchedfile", "Qzpeername", "Qztoolserver",
}

// ShareHome is the home folder ShareSnapshot's projects live under.
const ShareHome = "/home/Qzhomeowner/work"

// ShareSnapshot is a city of two projects whose every identifying word
// is one of ShareSecrets, with spend and token figures set, so a test of
// sharing mode can tell whether any of it reached the screen.
func ShareSnapshot(now time.Time) state.Snapshot {
	alpha, beta := ShareHome+"/Qzalphaproject", ShareHome+"/Qzbetaproject"
	session := func(id, cwd, title string, st state.State) state.Session {
		return state.Session{
			ID: id, Title: title, CWD: cwd, State: st, Alive: st != state.Resting,
			Branch: "Qzbranchname", Model: "claude-opus-5",
			LastPrompt: "Qzlastprompt please", Recap: claude.Recap{Text: "Qzrecaptext", At: now},
			Note: "Qznotetext", Subject: "Qzsubjecttext", Tool: "mcp__Qztoolserver__search",
			OrderLines: []claude.OrderLine{{Text: "Qzorderline", At: now}},
			Usage:      claude.Usage{Input: 1_200_000, Output: 340_000},
			CostUSD:    12.34, CostKnown: true,
			ContextTokens: 120_000, ContextWindow: 200_000, ContextKnown: true, ContextPercent: 60,
			TokensPerHour: 900_000, FreshTokensPerHour: 300_000, CacheReadPerHour: 500_000, OutputPerHour: 100_000,
			Storeys:   3,
			StartedAt: now.Add(-2 * time.Hour), LastActivity: now.Add(-time.Minute),
		}
	}
	a := session("aaaa-1", alpha, "Qzalphatitle", state.Working)
	a.Name = "Qzsessionname"
	a.MCPCalls = map[string]int{"Qzservername": 4}
	a.Skills = map[string]int{"Qzskillname": 2}
	a.PRs = []claude.PR{{Number: 7, URL: "https://example.com/Qzrepository/pull/7", Repository: "Qzrepository", State: claude.PROpen}}
	a.Touches = map[string]int{ShareHome + "/Qzalphaproject/Qztouchedfile.go": 3}
	a.SubagentNames = []string{"Qzsubagentname"}
	a.Agent = "Qzagentname"
	a.Team = "Qzteamname"
	a.Group, a.GroupJoined, a.GroupWords = "g1", "Qzjoinedwhy", []string{"Qzgroupword"}
	a.PeerMessages = []claude.PeerMessage{{ID: "m1", FromName: "Qzpeername", At: now}}
	b := session("bbbb-2", alpha, "Qzbetatitle", state.NeedsYou)
	b.Group = "g1"
	c := session("cccc-3", beta, "Qzgammatitle", state.Resting)
	return state.Snapshot{
		At:       now,
		Sessions: []state.Session{a, b, c},
		Servers:  []state.Server{{Name: "Qzservername", Calls: 4, Sessions: 1, Configured: true}},
		Skills:   []state.Skill{{Name: "Qzskillname", Calls: 2, Sessions: 1}},
		Teams:    []claude.Team{{Name: "Qzteamname", LeadSessionID: "aaaa-1", Members: []claude.TeamMember{{Name: "Qzmembername", CWD: alpha}}}},
		Groups: []state.Group{{
			ID: "g1", Name: "Qzgroupname", Short: "Qzgroupshort", Members: []string{"aaaa-1", "bbbb-2"},
			MemberDirs: []string{alpha, alpha}, Reason: "Qzgroupreason", Why: "Qzgroupwhy",
			Journal: state.Journal{Next: "Qzjournalnext"},
		}},
		GroupsReviewed: true,
		Power: state.Power{Since: now.Add(-24 * time.Hour), Fresh: 300_000, Cached: 9_000_000, Output: 200_000,
			ByModel: map[string]claude.Usage{"claude-opus-5": {Output: 200_000}}},
	}
}
