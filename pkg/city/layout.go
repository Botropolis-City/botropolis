package city

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	"botropolis.city/botropolis/pkg/plan"
	"botropolis.city/botropolis/pkg/state"
)

const (
	layoutVersion = 1
	appDir        = "botropolis"
	layoutFile    = "layout.json"
)

type districtLayout struct {
	Slots map[string]int `json:"slots"`
	// Type is the district's type, chosen the first time it appeared and
	// kept, or set by hand (item 109).
	Type DistrictType `json:"type,omitempty"`
}

// Layout is what the map remembers between runs: each district's block on
// the plan and each live session's slot in its district, so the map you
// have learned does not move under you.
type Layout struct {
	Version   int                        `json:"version"`
	Districts map[string]*districtLayout `json:"districts"`
	Plan      *plan.Memory               `json:"plan,omitempty"`
	// Hidden and Starred are map-only marks on projects, never written
	// anywhere near ~/.claude.
	Hidden  map[string]bool `json:"hidden,omitempty"`
	Starred map[string]bool `json:"starred,omitempty"`
	// Seen is the last moment the window was open and focused; the
	// away list on the next start covers everything after it.
	Seen time.Time `json:"seen,omitzero"`
	// LastLive is the ids of the sessions live when the map was last
	// seen, which Restore the desk reopens when you did not close up
	// (item 86). Ids only.
	LastLive []string `json:"lastLive,omitempty"`
	// View is the info view the window was last in, by name rather than
	// by number so reordering the views cannot silently change it.
	View string `json:"view,omitempty"`
	// Lenses counts how often each lens has been opened, by name, so
	// its panel shrinks to a chip once it has been seen (item 88).
	Lenses map[string]int `json:"lenses,omitempty"`
	// Sighted is every kind of thing the map has captioned the first
	// time it appeared on this machine, by name (item 88).
	Sighted map[string]bool `json:"sighted,omitempty"`
	// Done is each session triage marked done, at the activity it had
	// then, and Snoozed each one snoozed, until when (item 88).
	Done    map[string]time.Time `json:"done,omitempty"`
	Snoozed map[string]time.Time `json:"snoozed,omitempty"`
	// StreamHidden is whether the stream's column was hidden (item 88).
	StreamHidden bool `json:"streamHidden,omitempty"`
	// typing chooses a type for a district that has none yet; it is
	// the district_type setting, never written here (item 109).
	typing Typing
}

// SetTyping is how districts that have no type yet are given one.
func (l *Layout) SetTyping(t Typing) { l.typing = t }

// DistrictType is a district's type, chosen by the typing the first time
// it is asked for and kept from then on.
func (l *Layout) DistrictType(root string) DistrictType {
	d := l.district(root)
	if d.Type == "" {
		d.Type = l.typing.Choose(root)
	}
	return d.Type
}

// SetDistrictType sets a district's type by hand.
func (l *Layout) SetDistrictType(root string, t DistrictType) {
	l.district(root).Type = t
}

func NewLayout() *Layout {
	return &Layout{Version: layoutVersion, Districts: map[string]*districtLayout{}, Plan: plan.NewMemory()}
}

func (l *Layout) IsHidden(root string) bool  { return l.Hidden[root] }
func (l *Layout) IsStarred(root string) bool { return l.Starred[root] }

// SetHidden hides or shows a project on the map.
func (l *Layout) SetHidden(root string, hidden bool) {
	if l.Hidden == nil {
		l.Hidden = map[string]bool{}
	}
	if hidden {
		l.Hidden[root] = true
	} else {
		delete(l.Hidden, root)
	}
}

// SetStarred marks or unmarks a project.
func (l *Layout) SetStarred(root string, starred bool) {
	if l.Starred == nil {
		l.Starred = map[string]bool{}
	}
	if starred {
		l.Starred[root] = true
	} else {
		delete(l.Starred, root)
	}
}

// HiddenRoots is every hidden project, sorted.
func (l *Layout) HiddenRoots() []string {
	roots := make([]string, 0, len(l.Hidden))
	for root := range l.Hidden {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	return roots
}

func LayoutPath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(".", layoutFile)
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, appDir, layoutFile)
}

func LoadLayout(path string) (*Layout, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return NewLayout(), nil
	}
	if err != nil {
		return nil, err
	}
	var layout Layout
	if err := json.Unmarshal(data, &layout); err != nil {
		return nil, err
	}
	if layout.Version != layoutVersion || layout.Districts == nil {
		return NewLayout(), nil
	}
	for _, d := range layout.Districts {
		if d.Slots == nil {
			d.Slots = map[string]int{}
		}
	}
	if layout.Plan == nil || layout.Plan.Slots == nil {
		layout.Plan = plan.NewMemory()
	}
	return &layout, nil
}

func (l *Layout) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Slots gives every session a sticky slot in the district's grid. A
// session that stops keeps its slot: it rests on its own lot (item 86).
func (l *Layout) Slots(root string, sessions []state.Session) map[string]int {
	return assign(l.district(root).Slots, sessions)
}

func assign(sticky map[string]int, sessions []state.Session) map[string]int {
	slots := map[string]int{}
	for _, s := range sessions {
		slot, ok := sticky[s.ID]
		if !ok {
			slot = freeSlot(sticky)
			sticky[s.ID] = slot
		}
		slots[s.ID] = slot
	}
	return slots
}

func (l *Layout) district(root string) *districtLayout {
	if entry, ok := l.Districts[root]; ok {
		if entry.Slots == nil {
			entry.Slots = map[string]int{}
		}
		return entry
	}
	entry := &districtLayout{Slots: map[string]int{}}
	l.Districts[root] = entry
	return entry
}

func (l *Layout) memory() *plan.Memory {
	if l.Plan == nil || l.Plan.Slots == nil {
		l.Plan = plan.NewMemory()
	}
	return l.Plan
}

func freeSlot(slots map[string]int) int {
	taken := map[int]bool{}
	for _, slot := range slots {
		taken[slot] = true
	}
	for slot := 0; ; slot++ {
		if !taken[slot] {
			return slot
		}
	}
}
