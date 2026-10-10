package city

import (
	"fmt"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/format"
)

// The five things on the map that move, and what each one means.
//
// Every other object here could be asked what it stood for: a building,
// a district, a landmark, a road, a beam, a wire. These five could not,
// and the cost of that was not theoretical — the person who designed the
// map could not tell a drone from a car, and did not know what the flags
// were. Three of the five are vehicles, which is most of why they
// blurred into each other.
//
// So the rule in §1 — every object means one datum, and hovering it
// shows the number it stands for — now holds for everything that is
// drawn rather than for everything that stands still. A card that has to
// be explained in prose somewhere else is a card the map is not carrying.

// WorkerCard is the rover at a door: the session's own main thread. It
// waits at the door while the session is mid-turn and drives out to the
// kerb and back for each tool call, which is the whole of what a trip
// means.
func WorkerCard(b *Building) Card {
	card := Card{Title: "worker" + arrow + b.Card(time.Time{}).Title}
	card.Lines = append(card.Lines, "one trip out and back is one tool call")
	tool := b.Session.Tool
	if tool == "" {
		tool = format.Unknown
	}
	card.Lines = append(card.Lines, fmt.Sprintf("now      %s", tool))
	card.Lines = append(card.Lines, fmt.Sprintf("state    %s", b.Session.State))
	return card
}

// SubagentCard is one drone circling a roof: one subagent in flight. The
// drones are the only thing on the map that counts something happening
// somewhere else on the machine.
func SubagentCard(b *Building) Card {
	return Card{Title: "subagent" + arrow + b.Card(time.Time{}).Title, Lines: []string{
		fmt.Sprintf("in flight   %d", b.Session.SubagentsInFlight),
		fmt.Sprintf("this turn   %d spawned", b.Session.Subagents),
	}}
}

// CarCard is one car on a street: traffic between two repos, meaning
// messages passed and files touched across the boundary. A car could not
// say which pair it belonged to, because the street it drives is routed
// on the grid and runs along whichever districts happen to be adjacent.
func CarCard(r *RoadLine, from, to string) Card {
	return Card{Title: "traffic" + arrow + from + " ↔ " + to, Lines: []string{
		fmt.Sprintf("messages   %d", r.Messages),
		fmt.Sprintf("files      %d", r.Files),
		fmt.Sprintf("sessions   %d", r.Sessions),
	}}
}

// FlagCard is one flag on a roof: one pull request, up to three, in the
// accent open, green merged and slate closed.
func FlagCard(pr claude.PR) Card {
	title := fmt.Sprintf("pr #%d", pr.Number)
	if pr.Repository != "" {
		title += " " + pr.Repository
	}
	state := pr.State
	if state == "" {
		state = claude.PROpen
	}
	card := Card{Title: title, Lines: []string{fmt.Sprintf("state      %s", state)}}
	if pr.URL != "" {
		card.Lines = append(card.Lines, pr.URL)
	}
	return card
}

// SmokeCard is a plume over a roof: the session's API errors. It is the
// one mover that counts something going wrong rather than something
// being done.
func SmokeCard(b *Building) Card {
	return Card{Title: "api errors" + arrow + b.Card(time.Time{}).Title, Lines: []string{
		fmt.Sprintf("errors     %d", b.Session.APIErrors),
		"a plume for each, up to three",
	}}
}
