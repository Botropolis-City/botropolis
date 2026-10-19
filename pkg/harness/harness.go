package harness

import (
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

type Snapshotter interface {
	Name() string
	Load(now time.Time) (state.Snapshot, error)
	WatchDirs() []string
}

type Multi struct {
	harnesses []Snapshotter
	heights   *state.Heights
}

func NewMulti(harnesses ...Snapshotter) *Multi {
	return &Multi{harnesses: harnesses, heights: state.NewHeights()}
}

func (m *Multi) Name() string {
	return "multi"
}

func (m *Multi) Load(now time.Time) (state.Snapshot, error) {
	merged := state.Snapshot{At: now, Power: state.Power{Since: now.Add(-state.PowerWindow), ByModel: map[string]claude.Usage{}}}
	merged.HeftsRead = len(m.harnesses) > 0
	var sample []int64
	for _, h := range m.harnesses {
		snapshot, err := h.Load(now)
		if err != nil {
			return merged, err
		}
		if snapshot.ScaleHefts != nil {
			sample = append(sample, snapshot.ScaleHefts...)
			merged.ScaleHefts = append(merged.ScaleHefts, snapshot.ScaleHefts...)
		} else {
			sample = append(sample, state.SampleOf(snapshot.Sessions)...)
		}
		for _, s := range snapshot.Sessions {
			if s.Harness == "" {
				s.Harness = h.Name()
			}
			merged.Sessions = append(merged.Sessions, s)
		}
		merged.Servers = append(merged.Servers, snapshot.Servers...)
		merged.Skills = append(merged.Skills, snapshot.Skills...)
		merged.Roads = append(merged.Roads, snapshot.Roads...)
		merged.Teams = append(merged.Teams, snapshot.Teams...)
		merged.Groups = append(merged.Groups, snapshot.Groups...)
		merged.GroupsReviewed = merged.GroupsReviewed || snapshot.GroupsReviewed
		merged.Skipped = append(merged.Skipped, snapshot.Skipped...)
		if merged.Stats == nil {
			merged.Stats = snapshot.Stats
		}
		if merged.Notice == "" {
			merged.Notice = snapshot.Notice
		}
		for model, usage := range snapshot.Power.ByModel {
			merged.Power.ByModel[model] = merged.Power.ByModel[model].Add(usage)
		}
		merged.Power.Fresh += snapshot.Power.Fresh
		merged.Power.Cached += snapshot.Power.Cached
		merged.Power.Output += snapshot.Power.Output
		merged.HeftsRead = merged.HeftsRead && snapshot.HeftsRead
	}
	m.heights.ApplyOn(merged.Sessions, sample, merged.HeftsRead, now)
	return merged, nil
}

func (m *Multi) WatchDirs() []string {
	var dirs []string
	for _, h := range m.harnesses {
		dirs = append(dirs, h.WatchDirs()...)
	}
	return dirs
}

type Claude struct {
	*state.Loader
}

func NewClaude(loader *state.Loader) Claude {
	return Claude{Loader: loader}
}

func (Claude) Name() string {
	return "claude"
}

// Packer is a harness that can pack a session up or unpack it (item 86).
type Packer interface {
	Pack(id string, packed bool, at time.Time) error
}

// Pack asks every harness that can pack to record it; the Claude
// loader's ledger holds it for every session.
func (m *Multi) Pack(id string, packed bool, at time.Time) error {
	err := state.ErrNoLedger
	for _, h := range m.harnesses {
		if p, ok := h.(Packer); ok {
			if err = p.Pack(id, packed, at); err == nil {
				return nil
			}
		}
	}
	return err
}

// Forgetter is a harness that can forget a demolished session (item 144).
type Forgetter interface {
	Forget(id string) error
}

// Forget asks every harness that can forget a demolished session to.
func (m *Multi) Forget(id string) error {
	for _, h := range m.harnesses {
		if f, ok := h.(Forgetter); ok {
			if err := f.Forget(id); err != nil {
				return err
			}
		}
	}
	return nil
}

// CloseUpper is a harness that can record a close-up (item 86).
type CloseUpper interface {
	CloseUp(open, pinned []string, at time.Time) error
}

// CloseUp asks the first harness that can record a close-up to.
func (m *Multi) CloseUp(open, pinned []string, at time.Time) error {
	err := state.ErrNoCloseUp
	for _, h := range m.harnesses {
		if c, ok := h.(CloseUpper); ok {
			if err = c.CloseUp(open, pinned, at); err == nil {
				return nil
			}
		}
	}
	return err
}
