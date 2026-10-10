package city

import (
	"sort"
	"strings"

	"botropolis.city/botropolis/pkg/claude"
)

// Camp is a team on the map: its lead's building and its members',
// tied together so the crew reads as one whatever districts they sit in.
type Camp struct {
	Name    string
	Lead    *Building
	Members []*Building
}

// placeCamps ties each team's sessions together from the team rosters
// and the sessions' own team names.
func (c *City) placeCamps(teams []claude.Team) {
	c.Camps = nil
	byID := map[string]*Building{}
	for _, b := range c.Buildings() {
		byID[b.Session.ID] = b
	}
	named := map[string]*Camp{}
	for _, t := range teams {
		camp := &Camp{Name: t.Name, Lead: byID[t.LeadSessionID]}
		named[t.Name] = camp
		c.Camps = append(c.Camps, camp)
	}
	for _, b := range c.Buildings() {
		if b.Session.Team == "" {
			continue
		}
		camp, ok := named[b.Session.Team]
		if !ok {
			camp = &Camp{Name: b.Session.Team}
			named[b.Session.Team] = camp
			c.Camps = append(c.Camps, camp)
		}
		if camp.Lead == b {
			continue
		}
		if camp.Lead == nil && b.Session.Agent == "" {
			camp.Lead = b
			continue
		}
		camp.Members = append(camp.Members, b)
	}
	kept := c.Camps[:0]
	for _, camp := range c.Camps {
		if camp.Lead != nil || len(camp.Members) > 0 {
			kept = append(kept, camp)
		}
	}
	c.Camps = kept
	sort.Slice(c.Camps, func(i, j int) bool { return c.Camps[i].Name < c.Camps[j].Name })
}

// Camp is the camp a building belongs to, if any.
func (c *City) Camp(b *Building) *Camp {
	for _, camp := range c.Camps {
		if camp.Lead == b {
			return camp
		}
		for _, m := range camp.Members {
			if m == b {
				return camp
			}
		}
	}
	return nil
}

// Card is the camp's card: who leads and who follows.
func (camp *Camp) Card(c *City) Card {
	lines := []string{}
	if camp.Lead != nil {
		lines = append(lines, "lead     "+camp.Lead.Card(c.Time).Title)
	}
	names := make([]string, 0, len(camp.Members))
	for _, m := range camp.Members {
		name := m.Session.Agent
		if name == "" {
			name = m.Card(c.Time).Title
		}
		names = append(names, name)
	}
	if len(names) > 0 {
		lines = append(lines, "members  "+strings.Join(names, ", "))
	}
	return Card{Title: "team " + camp.Name, Lines: lines}
}
