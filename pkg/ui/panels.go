package ui

import "botropolis.city/botropolis/pkg/city"

// Every panel starts with one line of what it is, under its title
// (item 88).
const (
	PanelLenses   = "lenses"
	PanelPower    = "power"
	PanelTimeline = "timeline"
	PanelSettings = "settings"
	PanelReader   = "reader"
)

// PanelAbout is each panel's line.
var PanelAbout = map[string]string{
	PanelLenses:   "what the map can show you; press a number to switch",
	PanelPower:    "where the tokens and the money went, by model, project and session",
	PanelTimeline: "what has happened, newest first; enter goes to the session",
	PanelSettings: "how the city looks and behaves; changes are kept",
	PanelReader:   "the conversation, to read without leaving the city",
}

// subtitled is a title's height with its panel's line under it.
func subtitled(titleH float64, measure Measure) float64 {
	_, h := measure("", Small)
	return titleH + h
}

// subtitleUnder is a panel's line, placed under its title.
func subtitleUnder(title Text, panel string, measure Measure) Text {
	_, h := measure("", Title)
	return Text{Text: PanelAbout[panel], At: title.At.Add(city.Point{Y: h}), Size: Small}
}
