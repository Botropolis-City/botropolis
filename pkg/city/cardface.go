package city

import (
	"fmt"
	"time"

	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/state"
)

// face is the building's card as a view reads it: the sentence first,
// the view's three numbers, and today's table folded into Details
// (item 88). generated is a summary botropolis asked for, if any.
func (b *Building) face(now time.Time, v View, generated string) Card {
	s := b.Session
	title := s.Title
	if title == "" {
		title = state.ShortID(s.ID)
	}
	kind := "session in "
	if b.Vacant {
		kind = "empty plot in "
	}
	move, _ := state.GroupMove([]state.Session{s})
	card := Card{
		Title:    title,
		Kind:     kind + b.Where,
		Move:     movePhrase(move),
		Sentence: sentence(s, generated),
		Numbers:  b.numbers(v, now),
		Details:  b.table(now),
		Series:   b.Series(LastDay, now),
	}
	if g := b.Group; g != nil {
		card.Group = g.Name
		if b.Station > 0 {
			card.Group += fmt.Sprintf(" — station %d of %d", b.Station, b.Stations)
		}
	}
	return card
}

// movePhrase is a move as the card says it.
func movePhrase(m state.Move) string {
	if m == state.MoveYours {
		return "your move"
	}
	return string(m) + " move"
}

// MovePhrase is a move as the card and the plan's legend say it.
func MovePhrase(m state.Move) string { return movePhrase(m) }

// sentence is what the card leads with, in Claude's own words where
// there are any: the question it is asking, its recap, a summary
// generated for it, and failing those the prompt it was last given.
func sentence(s state.Session, generated string) string {
	switch {
	case s.State == state.NeedsYou && s.Note != "":
		return s.Note
	case s.Recap.Text != "":
		return s.Recap.Text
	case generated != "":
		return generated
	case s.LastPrompt != "":
		return "You asked: “" + s.LastPrompt + "”"
	}
	return ""
}

// numbers are the three figures a view puts on the face, the one it
// colours by first.
func (b *Building) numbers(v View, now time.Time) []Number {
	s := b.Session
	context := Number{Label: "context", Value: format.Percent(s.ContextPercent)}
	spent := Number{Label: "spent", Value: b.Spent()}
	idle := Number{Label: "idle", Value: format.Age(now.Sub(s.LastActivity))}
	fresh := Number{Label: "fresh/h", Value: format.Tokens(s.FreshTokensPerHour)}
	switch v {
	case ViewSpend:
		return []Number{spent, fresh, context}
	case ViewPressure:
		return []Number{context, {Label: "compacted", Value: fmt.Sprintf("%d×", s.Compactions)}, {Label: "tokens", Value: format.Tokens(float64(s.ContextTokens))}}
	case ViewStaleness:
		return []Number{idle, {Label: "age", Value: format.Age(now.Sub(s.StartedAt))}, context}
	case ViewServers:
		calls := 0
		for _, n := range s.MCPCalls {
			calls += n
		}
		return []Number{{Label: "server calls", Value: fmt.Sprint(calls)}, {Label: "servers", Value: fmt.Sprint(len(s.MCPCalls))}, idle}
	case ViewTraffic:
		crossings := 0
		others := map[string]bool{}
		for other, n := range s.Messages {
			crossings += n
			others[other] = true
		}
		for other, n := range s.Touches {
			crossings += n
			others[other] = true
		}
		return []Number{{Label: "crossings", Value: fmt.Sprint(crossings)}, {Label: "projects", Value: fmt.Sprint(len(others))}, idle}
	case ViewModels:
		return []Number{{Label: "model", Value: format.Dash(s.Model)}, spent, context}
	case ViewFanout:
		return []Number{{Label: "in flight", Value: fmt.Sprint(s.SubagentsInFlight)}, {Label: "subagents", Value: fmt.Sprint(s.Subagents)}, fresh}
	case ViewHealth:
		return []Number{{Label: "errors", Value: fmt.Sprint(s.APIErrors)}, {Label: "PRs", Value: fmt.Sprint(len(s.PRs))}, idle}
	case ViewGroups:
		station, members := "—", "—"
		if b.Group != nil {
			members = fmt.Sprint(len(b.Group.Members))
			if b.Station > 0 {
				station = fmt.Sprintf("%d of %d", b.Station, b.Stations)
			}
		}
		return []Number{{Label: "station", Value: station}, {Label: "in group", Value: members}, idle}
	}
	return []Number{context, spent, idle}
}

// face is the district's card: what it is, whose move its sessions
// leave, the reason, and its table folded into Details.
func (d *District) face(lines []string) Card {
	sessions := make([]state.Session, 0, len(d.Buildings))
	for _, b := range d.Buildings {
		sessions = append(sessions, b.Session)
	}
	move, why := state.GroupMove(sessions)
	prs := 0
	for _, s := range sessions {
		prs += len(s.PRs)
	}
	return Card{
		Title:    d.Name,
		Kind:     "project",
		Move:     movePhrase(move),
		Sentence: why,
		Numbers: []Number{
			{Label: "sessions", Value: fmt.Sprint(len(sessions))},
			{Label: "PRs", Value: fmt.Sprint(prs)},
		},
		Details: lines,
		Series:  d.Series(LastDay, d.at),
	}
}
