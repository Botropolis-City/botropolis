// Package ui is the chrome's layout model: theme tokens and the pure
// geometry of every panel, chip and card, measured but never drawn.
package ui

import (
	"image/color"

	"botropolis.city/botropolis/pkg/state"
)

// Size is one of the four type sizes the chrome uses.
type Size int

const (
	Small Size = iota
	Body
	Title
	Display
)

func (s Size) String() string {
	switch s {
	case Small:
		return "small"
	case Body:
		return "body"
	case Title:
		return "title"
	case Display:
		return "display"
	}
	return "unknown"
}

// Tone is a colour with a meaning; the same tone means the same thing on
// the map, the strip, a card and in the TUI.
type Tone int

const (
	ToneNone Tone = iota
	ToneNeedsYou
	ToneWorking
	ToneWaiting
	ToneUnattended
	ToneParked
	ToneError
	ToneMerged
)

func (t Tone) String() string {
	switch t {
	case ToneNeedsYou:
		return "needs-you"
	case ToneWorking:
		return "working"
	case ToneWaiting:
		return "waiting"
	case ToneUnattended:
		return "unattended"
	case ToneParked:
		return "parked"
	case ToneError:
		return "error"
	case ToneMerged:
		return "merged"
	}
	return "none"
}

// StateTone is the tone a session state is drawn in everywhere.
func StateTone(st state.State) Tone {
	switch st {
	case state.NeedsYou:
		return ToneNeedsYou
	case state.Working:
		return ToneWorking
	case state.Waiting:
		return ToneWaiting
	case state.Unattended:
		return ToneUnattended
	case state.Parked:
		return ToneParked
	}
	return ToneNone
}

type Palette struct {
	Panel    color.NRGBA
	Hairline color.NRGBA
	Text     color.NRGBA
	Dim      color.NRGBA
	Accent   color.NRGBA
	// Plant is the power plant's own tone, for the bands the kit painted
	// amber; amber is needs-you and means nothing else on the map.
	Plant color.NRGBA

	NeedsYou   color.NRGBA
	Working    color.NRGBA
	Waiting    color.NRGBA
	Unattended color.NRGBA
	Parked     color.NRGBA
	Error      color.NRGBA
	Merged     color.NRGBA

	// Ground is the map's grass by day and by night: the green the kit
	// sprites were lit against, and its dusk.
	Ground      color.NRGBA
	GroundNight color.NRGBA
}

var amber = color.NRGBA{0xe8, 0xa0, 0x3c, 0xff}

// DefaultPalette is the one state palette and the one accent.
var DefaultPalette = Palette{
	Panel:      color.NRGBA{0x0c, 0x0e, 0x14, 0xe6},
	Hairline:   color.NRGBA{0xff, 0xff, 0xff, 0x18},
	Text:       color.NRGBA{0xe6, 0xe9, 0xef, 0xff},
	Dim:        color.NRGBA{0x8a, 0x93, 0xa5, 0xff},
	Accent:     amber,
	Plant:      color.NRGBA{0x6a, 0x8c, 0xb8, 0xff},
	NeedsYou:   amber,
	Working:    color.NRGBA{0x55, 0x9a, 0xe0, 0xff},
	Waiting:    color.NRGBA{0x4c, 0xb8, 0xa8, 0xff},
	Unattended: color.NRGBA{0x8a, 0x6c, 0xd8, 0xff},
	Parked:     color.NRGBA{0x6b, 0x72, 0x80, 0xff},
	Error:      color.NRGBA{0xe0, 0x50, 0x50, 0xff},
	Merged:     color.NRGBA{0x4c, 0xc0, 0x7a, 0xff},

	Ground:      color.NRGBA{0x82, 0x9c, 0x6e, 0xff},
	GroundNight: color.NRGBA{0x2a, 0x36, 0x30, 0xff},
}

const (
	grid     = 8.0
	radius   = 6.0
	hairline = 1.0
)

var pointSizes = map[Size]float64{Small: 12, Body: 14, Title: 16, Display: 20}

// Theme is the palette and the metrics at one display scale; every
// measure the chrome uses comes out of it already scaled.
type Theme struct {
	Scale   float64
	Palette Palette
}

func NewTheme(scale float64) Theme {
	if scale <= 0 {
		scale = 1
	}
	return Theme{Scale: scale, Palette: DefaultPalette}
}

// Px scales a 1x measure to the display.
func (t Theme) Px(v float64) float64 { return v * t.Scale }

func (t Theme) Grid() float64     { return t.Px(grid) }
func (t Theme) Radius() float64   { return t.Px(radius) }
func (t Theme) Hairline() float64 { return t.Px(hairline) }

// Pt is the pixel size of one of the four type sizes on this display.
func (t Theme) Pt(s Size) float64 { return t.Px(pointSizes[s]) }

func (t Theme) Color(tone Tone) color.NRGBA {
	switch tone {
	case ToneNeedsYou:
		return t.Palette.NeedsYou
	case ToneWorking:
		return t.Palette.Working
	case ToneWaiting:
		return t.Palette.Waiting
	case ToneUnattended:
		return t.Palette.Unattended
	case ToneParked:
		return t.Palette.Parked
	case ToneError:
		return t.Palette.Error
	case ToneMerged:
		return t.Palette.Merged
	}
	return t.Palette.Dim
}
