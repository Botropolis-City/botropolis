package city

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"botropolis.city/botropolis/pkg/plan"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/statedir"
)

const (
	// layoutVersion is the shape this build writes, and
	// layoutReadableFrom the oldest version whose builds can still read
	// and write a layout at it: a change that only adds fields leaves it
	// alone, as stats.db's ReadableFrom does (item 149).
	layoutVersion      = 1
	layoutReadableFrom = 1
	layoutFile         = "layout.json"
)

type districtLayout struct {
	Slots map[string]int `json:"slots"`
	// Placed is the group each session's slot was chosen beside, so a
	// session Aria moved sits still once it is where she put it, and
	// moves again only when she changes its group (item 348).
	Placed map[string]string `json:"placed,omitempty"`
	// Type is the district's type, chosen the first time it appeared and
	// kept, or set by hand (item 109).
	Type DistrictType `json:"type,omitempty"`
	// Driveway is the side of its block the district's driveway is on,
	// and how many cells along that side, chosen when it first had a
	// road and kept (item 108).
	Driveway *drivewayMemory `json:"driveway,omitempty"`
}

type drivewayMemory struct {
	Side string `json:"side"`
	At   int    `json:"at"`
}

var sideNames = map[[2]int]string{{0, -1}: "N", {1, 0}: "E", {0, 1}: "S", {-1, 0}: "W"}

// keptDriveway is the side and place a district's driveway was given,
// if it has been given one.
func (l *Layout) keptDriveway(root string) (dc, dr, at int, ok bool) {
	d, found := l.Districts[root]
	if !found || d.Driveway == nil {
		return 0, 0, 0, false
	}
	for side, name := range sideNames {
		if name == d.Driveway.Side {
			return side[0], side[1], d.Driveway.At, true
		}
	}
	return 0, 0, 0, false
}

// keepDriveway remembers a district's driveway, for good.
func (l *Layout) keepDriveway(root string, dc, dr, at int) {
	l.district(root).Driveway = &drivewayMemory{Side: sideNames[[2]int{dc, dr}], At: at}
}

// Layout is what the map remembers between runs: each district's block on
// the plan and each live session's slot in its district, so the map you
// have learned does not move under you.
type Layout struct {
	Version int `json:"version"`
	// ReadableFrom is the oldest layout version whose builds can read
	// this file; a newer file keeps its own when this build saves it.
	ReadableFrom int                        `json:"readableFrom"`
	Districts    map[string]*districtLayout `json:"districts"`
	Plan         *plan.Memory               `json:"plan,omitempty"`
	// Bridge is the row the bridge lands on, by its place among the
	// plan's slot rows, chosen when the island first had one and kept
	// while it is a street (item 218).
	Bridge *plan.RowMark `json:"bridge,omitempty"`
	// Bridge2 is the row the second bridge lands on, chosen when the
	// island first held SecondBridgeAt districts and kept from then on
	// (item 220).
	Bridge2 *plan.RowMark `json:"bridge2,omitempty"`
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
	// moved is every session whose slot was changed by hand since the
	// scene last asked (item 348).
	moved []string
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
	return &Layout{Version: layoutVersion, ReadableFrom: layoutReadableFrom, Districts: map[string]*districtLayout{}, Plan: plan.NewMemory()}
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
	return filepath.Join(statedir.Dir(), layoutFile)
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
	if layout.Version > layoutVersion && (layout.ReadableFrom == 0 || layout.ReadableFrom > layoutVersion) {
		return nil, statedir.NewerError{Path: path, Kind: "layout", Version: layout.Version, Ours: layoutVersion}
	}
	if layout.Version < layoutVersion {
		layout.Version, layout.ReadableFrom = layoutVersion, layoutReadableFrom
	}
	if layout.Districts == nil {
		layout.Districts = map[string]*districtLayout{}
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
// A session with no slot yet takes the free one nearest its group's,
// and a session whose group Aria changed moves beside its new group, or
// out of the one it left; the ids of those moved are kept for the
// scene, which draws the move (item 348).
func (l *Layout) Slots(root string, sessions []state.Session) map[string]int {
	slots, moved := assign(l.district(root), sessions)
	l.moved = append(l.moved, moved...)
	return slots
}

// Moved is every session whose slot the layout changed by Aria's hand
// since it was last asked, and forgets them.
func (l *Layout) Moved() []string {
	moved := l.moved
	l.moved = nil
	return moved
}

// changedByHand is whether Aria changed a session's group since its slot
// was chosen: a move to a group other than the one it was placed beside,
// or a take-out of that group (item 348).
func changedByHand(d *districtLayout, s state.Session) bool {
	placed := d.Placed[s.ID]
	if s.GroupMoved && placed != s.Group {
		return true
	}
	return placed != "" && slices.Contains(s.GroupLeft, placed)
}

// assign gives sessions, in arrival order, their slots, and says which
// it moved by hand. The grid's width depends on the highest slot, so
// when placing raised it the pending ones are placed again at the new
// width.
func assign(d *districtLayout, sessions []state.Session) (map[string]int, []string) {
	sticky := d.Slots
	if d.Placed == nil {
		d.Placed = map[string]string{}
	}
	slots := map[string]int{}
	var pending []state.Session
	var moved []string
	for _, s := range sessions {
		slot, ok := sticky[s.ID]
		if ok && !changedByHand(d, s) {
			slots[s.ID] = slot
			continue
		}
		if ok {
			delete(sticky, s.ID)
			moved = append(moved, s.ID)
		}
		pending = append(pending, s)
	}
	if len(pending) == 0 {
		return slots, nil
	}
	groupOf := map[string]string{}
	for _, s := range sessions {
		groupOf[s.ID] = s.Group
	}
	width := buildingColumns(max(maxOf(slots)+1, len(sessions)))
	for range 4 {
		for _, s := range pending {
			var near, clear []int
			left := d.Placed[s.ID]
			leaving := left != "" && left != s.Group
			// A loose newcomer keeps room for every group, within the
			// lots the grid needs (Aria, 2026-11-06); one leaving a group
			// keeps clear of it whatever it takes.
			limit := -1
			if s.Group == "" && !leaving {
				limit = width * ((len(sessions) + width - 1) / width)
			}
			for id, slot := range slots {
				switch {
				case s.Group != "" && groupOf[id] == s.Group:
					near = append(near, slot)
				case leaving && groupOf[id] == left:
					clear = append(clear, slot)
				case limit >= 0 && groupOf[id] != "":
					clear = append(clear, slot)
				}
			}
			slot := chooseSlot(sticky, near, clear, width, limit)
			sticky[s.ID], slots[s.ID] = slot, slot
		}
		again := buildingColumns(maxOf(slots) + 1)
		if again == width {
			break
		}
		width = again
		for _, s := range pending {
			delete(sticky, s.ID)
			delete(slots, s.ID)
		}
	}
	for _, s := range pending {
		d.Placed[s.ID] = s.Group
	}
	return slots, moved
}

// chooseSlot is the free slot nearest the slots near, by lots walked in
// a grid of the width, the lowest on a tie; with nothing near, the
// lowest free slot not beside any of clear, below limit when it is not
// negative, and the lowest free slot when there is none below it.
func chooseSlot(sticky map[string]int, near, clear []int, width, limit int) int {
	taken := map[int]bool{}
	bound := width
	for _, slot := range sticky {
		taken[slot] = true
		bound = max(bound, slot+1)
	}
	best, bestDist := -1, math.MaxInt
	if len(near) > 0 {
		bestDist = math.MaxInt
	}
	for slot := 0; slot <= bound+2*width; slot++ {
		if taken[slot] {
			continue
		}
		if len(near) == 0 {
			if best < 0 {
				best = slot
			}
			if limit >= 0 && slot >= limit {
				return best
			}
			if gridDistance(slot, clear, width) > 1 {
				return slot
			}
			continue
		}
		if dist := gridDistance(slot, near, width); dist < bestDist {
			best, bestDist = slot, dist
		}
	}
	return best
}

// gridDistance is how many lots a slot is from the nearest of others in
// a grid of the width, or MaxInt with none.
func gridDistance(slot int, others []int, width int) int {
	dist := math.MaxInt
	for _, o := range others {
		dist = min(dist, abs(slot%width-o%width)+abs(slot/width-o/width))
	}
	return dist
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
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

// SetAside reports whether triage has put a session down: marked done
// with nothing new since, or snoozed until a time still to come.
func (l *Layout) SetAside(sess state.Session, now time.Time) bool {
	if l == nil {
		return false
	}
	if at, ok := l.Done[sess.ID]; ok && !sess.LastActivity.After(at) {
		return true
	}
	if until, ok := l.Snoozed[sess.ID]; ok && now.Before(until) {
		return true
	}
	return false
}
