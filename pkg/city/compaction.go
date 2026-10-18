package city

import (
	"time"

	"botropolis.city/botropolis/pkg/state"
)

// How full a live session's context is shows as a skip at the kerb
// beside its building, and a compaction is the skip collected and a new
// course poured under the building: a skip truck hooks the full skip and
// drives off, and as it leaves a cement mixer pulls up, pours, and goes.
// Item 109.

// SkipLevel is how full a session's skip is: none below half its
// context window, then low, to the rim, heaped from 80%, and
// overflowing near the limit. A stopped session has none.
func SkipLevel(s state.Session) int {
	if !state.Live(s.State) {
		return 0
	}
	switch p := s.ContextPercent; {
	case p >= 95:
		return 4
	case p >= 80:
		return 3
	case p >= 65:
		return 2
	case p >= 50:
		return 1
	}
	return 0
}

// The job's timeline, from the moment the scene sees the compaction.
const (
	jobSkipArrive  = 400 * time.Millisecond
	jobSkipHooked  = 800 * time.Millisecond
	jobSkipGone    = 1800 * time.Millisecond
	jobMixerSet    = 900 * time.Millisecond
	jobMixerArrive = 1300 * time.Millisecond
	jobPoured      = 1900 * time.Millisecond
	CompactionFor  = 2400 * time.Millisecond
)

// Truck is one of a job's trucks: whether it is on the lot, and how far
// along the kerb it is, from 0 where it comes in through 1 where it
// stops to 2 where it leaves.
type Truck struct {
	Present bool
	Along   float64
	Loaded  bool
}

// Job is a compaction being carried out at a building.
type Job struct {
	SkipTruck Truck
	Mixer     Truck
	// SkipLevel is the skip as it stood before the compaction, shown at
	// the kerb until the truck hooks it.
	SkipLevel int
	// SkipAtKerb is whether that skip still stands at the kerb.
	SkipAtKerb bool
	// Pour is how far the new course has risen, 0 to 1.
	Pour float64
}

type compactionJob struct {
	since time.Time
	level int
}

// noteCompactions starts a job for every live session whose compactions
// rose since the last snapshot, one job however many it made, and
// remembers each session's skip for the next. The first snapshot only
// takes note, and at still nothing is started: the course is simply
// there and the skip shows its new fill.
func (s *Scene) noteCompactions(snapshot state.Snapshot, now time.Time) {
	first := s.compacted == nil
	if first {
		s.compacted = map[string]int{}
		s.skips = map[string]int{}
	}
	here := map[string]bool{}
	for _, sess := range snapshot.Sessions {
		here[sess.ID] = true
		was, known := s.compacted[sess.ID]
		level := s.skips[sess.ID]
		s.compacted[sess.ID], s.skips[sess.ID] = sess.Compactions, SkipLevel(sess)
		if first || !known || sess.Compactions <= was || s.reduced || !state.Live(sess.State) {
			continue
		}
		if s.jobs == nil {
			s.jobs = map[string]compactionJob{}
		}
		s.jobs[sess.ID] = compactionJob{since: now, level: level}
	}
	for id := range s.compacted {
		if !here[id] {
			delete(s.compacted, id)
			delete(s.skips, id)
			delete(s.jobs, id)
		}
	}
}

// CompactionJob is the job under way at a session's building, if any.
func (s *Scene) CompactionJob(id string) (Job, bool) {
	j, ok := s.jobs[id]
	if !ok {
		return Job{}, false
	}
	t := s.now().Sub(j.since)
	if t >= CompactionFor || t < 0 {
		delete(s.jobs, id)
		return Job{}, false
	}
	job := Job{SkipLevel: j.level, SkipAtKerb: t < jobSkipHooked}
	switch {
	case t < jobSkipArrive:
		job.SkipTruck = Truck{Present: true, Along: share(t, 0, jobSkipArrive)}
	case t < jobSkipHooked:
		job.SkipTruck = Truck{Present: true, Along: 1}
	case t < jobSkipGone:
		job.SkipTruck = Truck{Present: true, Along: 1 + share(t, jobSkipHooked, jobSkipGone), Loaded: true}
	}
	switch {
	case t < jobMixerSet:
	case t < jobMixerArrive:
		job.Mixer = Truck{Present: true, Along: share(t, jobMixerSet, jobMixerArrive)}
	case t < jobPoured:
		job.Mixer = Truck{Present: true, Along: 1}
		job.Pour = share(t, jobMixerArrive, jobPoured)
	default:
		job.Mixer = Truck{Present: true, Along: 1 + share(t, jobPoured, CompactionFor)}
		job.Pour = 1
	}
	if t < jobMixerArrive {
		job.Pour = 0
	}
	return job, true
}

// Compacting is whether any job is under way, so the map keeps drawing.
func (s *Scene) Compacting() bool {
	for id := range s.jobs {
		if _, ok := s.CompactionJob(id); ok {
			return true
		}
	}
	return false
}

func share(t, from, to time.Duration) float64 {
	return min(1, max(0, float64(t-from)/float64(to-from)))
}
