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
}

func NewMulti(harnesses ...Snapshotter) *Multi {
	return &Multi{harnesses: harnesses}
}

func (m *Multi) Name() string {
	return "multi"
}

func (m *Multi) Load(now time.Time) (state.Snapshot, error) {
	merged := state.Snapshot{At: now, Power: state.Power{Since: now.Add(-state.PowerWindow), ByModel: map[string]claude.Usage{}}}
	for _, h := range m.harnesses {
		snapshot, err := h.Load(now)
		if err != nil {
			return merged, err
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
		for model, usage := range snapshot.Power.ByModel {
			merged.Power.ByModel[model] = merged.Power.ByModel[model].Add(usage)
		}
		merged.Power.CostUSD += snapshot.Power.CostUSD
		merged.Power.CostKnown = merged.Power.CostKnown || snapshot.Power.CostKnown
		merged.Power.Fresh += snapshot.Power.Fresh
		merged.Power.Cached += snapshot.Power.Cached
	}
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
