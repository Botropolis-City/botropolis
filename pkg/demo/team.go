package demo

import (
	"path/filepath"
	"sort"
	"time"
)

const teamLead = "team-lead"

// writeTeams writes the config Claude Code keeps for each team whose
// lead stands live, naming every placed member with its working
// directory as staged, which is where Roads sends a member's messages.
// Claude Code removes a team's config when its lead's session ends, so
// a team whose lead rests has none.
func writeTeams(dir string, at map[int]placed, now time.Time) error {
	order := make([]int, 0, len(at))
	for i := range at {
		order = append(order, i)
	}
	sort.Ints(order)
	for _, i := range order {
		lead := at[i]
		team := lead.leads
		if lead.pid == 0 || team == "" {
			continue
		}
		joined := now.UnixMilli()
		members := []map[string]any{member(teamLead, team, "team-lead", "in-process", lead.cwd, joined)}
		for _, j := range order {
			mate := at[j]
			if j == i || mate.team != team || mate.agent == "" || mate.agent == teamLead {
				continue
			}
			members = append(members, member(mate.agent, team, "general-purpose", "tmux", mate.cwd, joined))
		}
		config := map[string]any{
			"name":          team,
			"createdAt":     joined,
			"leadAgentId":   teamLead + "@" + team,
			"leadSessionId": lead.id,
			"members":       members,
		}
		if err := writeJSON(filepath.Join(dir, ".claude", "teams", team, "config.json"), config); err != nil {
			return err
		}
	}
	return nil
}

func member(name, team, agentType, backend, cwd string, joined int64) map[string]any {
	return map[string]any{
		"name":        name,
		"agentId":     name + "@" + team,
		"agentType":   agentType,
		"backendType": backend,
		"cwd":         cwd,
		"joinedAt":    joined,
	}
}
