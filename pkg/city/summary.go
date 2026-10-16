package city

import (
	"fmt"
	"strings"

	"botropolis.city/botropolis/pkg/state"
)

// Summary is the city-wide tally drawn along the top of the window, the
// way a strategy game keeps its resources in view.
type Summary struct {
	Working    int
	NeedsYou   int
	Waiting    int
	Unattended int
	Resting    int
	FreshPerH  float64
	CachedPerH float64
	CostUSD    float64
	CostKnown  bool
	HitRatio   float64
	Subagents  int
	PRs        int
	MCPCalls   int
	Errors     int
	// BudgetUSD is the daily target from config, 0 for none.
	BudgetUSD float64
}

// BudgetShare is how much of the daily budget the last 24 h spent, or 0
// when there is no budget.
func (s Summary) BudgetShare() float64 {
	if s.BudgetUSD <= 0 {
		return 0
	}
	return s.CostUSD / s.BudgetUSD
}

// Live is every session that is not resting.
func (s Summary) Live() int {
	return s.Working + s.NeedsYou + s.Waiting + s.Unattended
}

func (c *City) Summary() Summary {
	var s Summary
	for _, b := range c.Buildings() {
		if b.Packed {
			continue
		}
		switch b.Session.State {
		case state.Working:
			s.Working++
		case state.NeedsYou:
			s.NeedsYou++
		case state.Waiting:
			s.Waiting++
		case state.Unattended:
			s.Unattended++
		case state.Resting:
			s.Resting++
		}
		if state.Live(b.Session.State) {
			s.FreshPerH += b.Session.FreshTokensPerHour
			s.CachedPerH += b.Session.CacheReadPerHour
			s.Subagents += b.Session.SubagentsInFlight
			s.Errors += b.Session.APIErrors
		}
		s.PRs += len(b.Session.PRs)
	}
	s.CostUSD = c.Plant.Power.CostUSD
	s.CostKnown = c.Plant.Power.CostKnown
	s.HitRatio = c.Plant.Power.HitRatio()
	s.BudgetUSD = c.Plant.BudgetUSD
	for _, t := range c.Towers {
		s.MCPCalls += t.Server.Calls
	}
	return s
}

// Counts is the tally by state, for anything that walks state.Order.
func (s Summary) Counts() map[state.State]int {
	return map[state.State]int{
		state.NeedsYou:   s.NeedsYou,
		state.Working:    s.Working,
		state.Waiting:    s.Waiting,
		state.Unattended: s.Unattended,
		state.Resting:    s.Resting,
	}
}

// Headline is the one-line form for a window title: the live states by
// urgency, zero counts left out.
func (s Summary) Headline() string {
	var parts []string
	for _, c := range state.Nonzero(s.Counts(), state.Live) {
		parts = append(parts, CountLabel(c))
	}
	if len(parts) == 0 {
		return "quiet"
	}
	return strings.Join(parts, " · ")
}

// CountLabel is a state count as prose: "2 need you", "1 working".
func CountLabel(c state.Count) string {
	if c.State == state.NeedsYou {
		return fmt.Sprintf("%d need you", c.N)
	}
	return fmt.Sprintf("%d %s", c.N, c.State)
}

// StateCard lists the sessions in a state, for hovering a strip chip.
func (c *City) StateCard(st state.State) Card {
	var lines []string
	for _, b := range c.Buildings() {
		if b.Session.State != st {
			continue
		}
		line := b.Card(c.Time).Title
		if doing := b.Session.Doing(); doing != "" {
			line += "  " + doing
		}
		lines = append(lines, line)
		if len(lines) == maxStateCardLines {
			lines = append(lines, fmt.Sprintf("... and %d more", c.Summary().Counts()[st]-maxStateCardLines))
			break
		}
	}
	if len(lines) == 0 {
		lines = []string{"none"}
	}
	return Card{Title: string(st), Lines: lines}
}

const maxStateCardLines = 12
