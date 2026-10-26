// Package script is the choreography for a recorded clip: what the
// pointer and keyboard do, and when, counted in seconds of video. The
// demo stager writes it and the city plays it; both read this one shape.
package script

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// Cue is one thing done at At seconds into the clip. Exactly one of the
// verbs is set.
type Cue struct {
	At float64 `json:"at" yaml:"at"`

	// Key presses a key by its Ebitengine name, as --keys does.
	Key string `json:"key,omitempty" yaml:"key,omitempty"`
	// Hold holds a key down For seconds: an arrow pans smoothly.
	Hold string  `json:"hold,omitempty" yaml:"hold,omitempty"`
	For  float64 `json:"for,omitempty" yaml:"for,omitempty"`
	// Point glides the pointer to a window position over Over seconds;
	// PointAt glides it to something in the city -- a session by title
	// or id, "district:<name>", "car:<from>-><to>" for the first car on
	// the road between two districts, or "plant" -- and keeps it there as the
	// camera moves.
	Point   []float64 `json:"point,omitempty" yaml:"point,omitempty"`
	PointAt string    `json:"point_at,omitempty" yaml:"point_at,omitempty"`
	Over    float64   `json:"over,omitempty" yaml:"over,omitempty"`
	// PanTo glides the camera over Over seconds until a target -- named
	// as PointAt names one -- is in the middle of the window. Unlike an
	// arrow held down, it cannot carry the camera off the city. "home"
	// is wherever the camera stood when the clip began.
	PanTo string `json:"pan_to,omitempty" yaml:"pan_to,omitempty"`
	// Click clicks where the pointer is.
	Click bool `json:"click,omitempty" yaml:"click,omitempty"`
	// Wheel turns the scroll wheel at the pointer by this many notches,
	// spread over Over seconds: a smooth zoom toward what it points at.
	Wheel float64 `json:"wheel,omitempty" yaml:"wheel,omitempty"`
	// ShowStatsPrompt shows the statistics prompt, off or on, as the
	// city would show it at open, so a frame can show what a recording
	// is otherwise never asked. Item 110.
	ShowStatsPrompt string `json:"show_stats_prompt,omitempty" yaml:"show_stats_prompt,omitempty"`
	// ShowStoreNotice shows the notice for a statistics store the daemon
	// refused, newer or damaged, so a frame can show it. Item 190.
	ShowStoreNotice string `json:"show_store_notice,omitempty" yaml:"show_store_notice,omitempty"`
}

func (c Cue) verbs() int {
	n := 0
	for _, set := range []bool{c.Key != "", c.Hold != "", len(c.Point) > 0, c.PointAt != "", c.PanTo != "", c.Click, c.Wheel != 0, c.ShowStatsPrompt != "", c.ShowStoreNotice != ""} {
		if set {
			n++
		}
	}
	return n
}

// Validate refuses a cue that does nothing, or two things at once, or
// a point that is not x,y: a typo should fail the shot, not film a clip
// in which nothing happens.
func Validate(cues []Cue) error {
	for i, c := range cues {
		switch {
		case c.verbs() != 1:
			return fmt.Errorf("cue %d at %gs: want exactly one of key, hold, point, point_at, pan_to, click, wheel, show_stats_prompt, show_store_notice", i, c.At)
		case len(c.Point) != 0 && len(c.Point) != 2:
			return fmt.Errorf("cue %d at %gs: point wants [x, y]", i, c.At)
		case c.Hold != "" && c.For <= 0:
			return fmt.Errorf("cue %d at %gs: hold wants a duration in for", i, c.At)
		case c.ShowStatsPrompt != "" && c.ShowStatsPrompt != "off" && c.ShowStatsPrompt != "on":
			return fmt.Errorf("cue %d at %gs: show_stats_prompt wants off or on", i, c.At)
		case c.ShowStoreNotice != "" && c.ShowStoreNotice != "newer" && c.ShowStoreNotice != "damaged":
			return fmt.Errorf("cue %d at %gs: show_store_notice wants newer or damaged", i, c.At)
		case c.At < 0:
			return fmt.Errorf("cue %d: at %gs is before the clip starts", i, c.At)
		}
	}
	return nil
}

// Sorted is the cues in the order they happen.
func Sorted(cues []Cue) []Cue {
	out := append([]Cue(nil), cues...)
	sort.SliceStable(out, func(a, b int) bool { return out[a].At < out[b].At })
	return out
}

// Load reads a script file written by Save.
func Load(path string) ([]Cue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cues []Cue
	if err := json.Unmarshal(data, &cues); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cues, Validate(cues)
}

// Save writes cues for the city to Load.
func Save(path string, cues []Cue) error {
	if err := Validate(cues); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cues, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
