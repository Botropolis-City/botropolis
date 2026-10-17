package city

import (
	"fmt"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/claude"
)

// The things on the map that move, and what each one means.
//
// Every other object here could be asked what it stood for: a building,
// a district, a landmark, a road, a beam, a wire. The first five movers
// could not, and the cost of that was not theoretical — the person who
// designed the map could not tell a drone from a car, and did not know
// what the flags were. Three of the five were vehicles, which is most of
// why they blurred into each other. The rover was one of them, and it
// went in item 89: it stood for the session, which the building already
// is, so the building now shows its own work.
//
// So the rule in §1 — every object means one datum, and hovering it
// shows the number it stands for — now holds for everything that is
// drawn rather than for everything that stands still. A card that has to
// be explained in prose somewhere else is a card the map is not carrying.

// SubagentCard is one drone at work on a building: one subagent in flight. The
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

// InspectorCard is the inspector's car outside a building: the session's
// open pull requests, which it waits on. Item 89.
func InspectorCard(b *Building) Card {
	var open []string
	for _, pr := range b.Session.PRs {
		if pr.State == "" || pr.State == claude.PROpen {
			open = append(open, fmt.Sprintf("#%d", pr.Number))
		}
	}
	return Card{Title: "inspector" + arrow + b.Card(time.Time{}).Title, Lines: []string{
		"waiting on " + strings.Join(open, ", "),
	}}
}

// DeliveryCard is a truck taking a merged pull request's crate away.
// Item 89.
func DeliveryCard(d Delivery) Card {
	title := fmt.Sprintf("merged"+arrow+"#%d", d.PR.Number)
	if d.PR.Repository != "" {
		title += " " + d.PR.Repository
	}
	card := Card{Title: title, Lines: []string{"leaving by the bridge"}}
	if d.PR.URL != "" {
		card.Lines = append(card.Lines, d.PR.URL)
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
