package demo

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"botropolis.city/botropolis/pkg/script"
	"botropolis.city/botropolis/pkg/state"
)

// Scenario is one city to film: which recorded sessions stand where,
// in what state, and what the account around them looks like.
type Scenario struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	// Clock is the time of day the city is lit for, HH:MM; 14:00 when
	// unset, so a frame does not depend on when it was filmed.
	Clock    string      `yaml:"clock"`
	MCP      []string    `yaml:"mcp"`
	Sessions []Placement `yaml:"sessions"`
	// Groups are the groups the city is expected to find once the
	// scenario is staged, each named by its members' titles (item 223).
	Groups [][]string `yaml:"groups"`
	// Pace is how often a heavy user's session takes a turn: each session
	// with a started gets its hours before the recording filled with
	// turns this far apart, so its rates read as hours of work rather
	// than a few minutes spread thin (item 240). Unset adds none.
	Pace  Duration `yaml:"pace"`
	Shots []Shot   `yaml:"shots"`
	// Usage floats the boats; UsageChanges moves them during a clip.
	Usage        *Usage        `yaml:"usage"`
	UsageChanges []UsageChange `yaml:"usage_changes"`
}

// Placement puts one session in the city. Ref names a recorded session;
// an empty plot has no recording and names its Project instead. With
// both, the recording is cloned into that project as a session of its
// own, which is how a big city is filled from a small corpus.
type Placement struct {
	Ref     string      `yaml:"ref"`
	Project string      `yaml:"project"`
	State   state.State `yaml:"state"`
	// Ago is how long before now the session's last line sits.
	Ago Duration `yaml:"ago"`
	// Started is how long before now the session was opened, when that
	// is longer than its recording runs: staged as recorded, every
	// session would look opened minutes ago, and the city would group
	// them all as started together (item 223).
	Started Duration `yaml:"started"`

	// The rest is overlaid on the recording, for what a recording run
	// cannot cheaply produce: a title, pull requests in each state
	// (open, merged, closed), API errors, and a place in a team.
	Title  string   `yaml:"title"`
	PRs    []string `yaml:"prs"`
	Errors int      `yaml:"errors"`
	Team   string   `yaml:"team"`
	Agent  string   `yaml:"agent"`
	// Context is how full the session's context window is, "72%":
	// the building's height. Recorded sessions are short and efficient
	// and all stop under 15%, which would draw every building alike.
	Context string `yaml:"context"`

	// A clip that plays the timeline sees the session arrive Arrive
	// into it, leave at Leave, and change as Changes say. A still, and
	// a clip that does not, shows the city as it stands at the start.
	Arrive Duration `yaml:"arrive"`
	// Replay reveals the recording over part of the clip instead of
	// staging it whole, so the building is seen to grow.
	Replay  *Replay  `yaml:"replay"`
	Leave   Duration `yaml:"leave"`
	Changes []Change `yaml:"changes"`
}

// Replay is how a recording is played into a clip: its lines revealed
// at their own relative pacing, compressed into Over from the moment the
// session arrives, with its usage scaled so it ends at Context.
type Replay struct {
	Over    Duration `yaml:"over"`
	Context string   `yaml:"context"`
}

// Change is something that happens to a session partway through a clip:
// its state flips, its PRs move on (each entry is the new state of the
// PR at that position), or it hits errors.
type Change struct {
	At     Duration    `yaml:"at"`
	State  state.State `yaml:"state"`
	PRs    []string    `yaml:"prs"`
	Errors int         `yaml:"errors"`
}

// Validate refuses a scenario that would film nothing, or that names
// two shots alike and so writes one over the other.
func (s Scenario) Validate() error {
	if len(s.Sessions) == 0 {
		return fmt.Errorf("scenario %q places no sessions", s.Name)
	}
	if s.Pace < 0 {
		return fmt.Errorf("scenario %q: pace %s is below zero", s.Name, time.Duration(s.Pace))
	}
	seen := map[string]bool{}
	for i, shot := range s.Shots {
		if shot.Name == "" {
			return fmt.Errorf("scenario %q: shot %d has no name", s.Name, i)
		}
		if err := script.Validate(shot.Script); err != nil {
			return fmt.Errorf("scenario %q: shot %q: %w", s.Name, shot.Name, err)
		}
		if len(shot.Script) > 0 && shot.Record == nil {
			return fmt.Errorf("scenario %q: shot %q: a script plays only while recording", s.Name, shot.Name)
		}
		if shot.View != "" {
			if _, err := viewKeys(shot.View); err != nil {
				return fmt.Errorf("scenario %q: shot %q: %w", s.Name, shot.Name, err)
			}
		}
		if err := shot.validPosterAt(); err != nil {
			return fmt.Errorf("scenario %q: shot %q: %w", s.Name, shot.Name, err)
		}
		if seen[shot.Name] {
			return fmt.Errorf("scenario %q: two shots named %q", s.Name, shot.Name)
		}
		seen[shot.Name] = true
	}
	return nil
}

// Duration reads "90s", "3h" or "2d" from YAML: a scenario's parked
// sessions are days old, and Go's durations stop at hours.
type Duration time.Duration

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	raw, days := node.Value, 0
	if i := strings.IndexByte(raw, 'd'); i > 0 {
		n, err := strconv.Atoi(raw[:i])
		if err != nil {
			return fmt.Errorf("line %d: duration %q", node.Line, node.Value)
		}
		raw, days = raw[i+1:], n
	}
	var parsed time.Duration
	if raw != "" {
		var err error
		if parsed, err = time.ParseDuration(raw); err != nil {
			return fmt.Errorf("line %d: %w", node.Line, err)
		}
	}
	*d = Duration(time.Duration(days)*24*time.Hour + parsed)
	return nil
}

// LoadScenario reads a scenario file. Unknown keys are refused: a typo
// in a scenario films a different city without saying so.
func LoadScenario(path string) (Scenario, error) {
	f, err := os.Open(path)
	if err != nil {
		return Scenario{}, err
	}
	defer func() { _ = f.Close() }()
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	var s Scenario
	if err := dec.Decode(&s); err != nil {
		return Scenario{}, fmt.Errorf("%s: %w", path, err)
	}
	if err := s.Validate(); err != nil {
		return Scenario{}, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}

// validPosterAt refuses a poster_at no frame of the clip sits at.
func (shot Shot) validPosterAt() error {
	switch at := shot.PosterAt; {
	case at == nil:
		return nil
	case shot.Record == nil:
		return fmt.Errorf("poster_at: a still is its own poster")
	case *at < 0 || *at > shot.Record.Seconds:
		return fmt.Errorf("poster_at %gs: outside the clip's %gs", *at, shot.Record.Seconds)
	}
	return nil
}
