package state

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"botropolis.city/botropolis/pkg/claude"
)

// laterRead holds back from a loader's first load every read longer than
// a window, live transcripts read whole and stopped ones read from the
// plant's window on, and reads them afterwards in a paced pass, so the
// map draws from the windows first. Each load until a transcript's read
// has finished reads it through its window. Item 203: on Aria's history
// the first load read 708 MiB, 9 s at a core, before the map could draw.
type laterRead struct {
	done  func()
	pace  *pace
	gate  func()
	lane  chan struct{}
	mu    sync.Mutex
	jobs  []laterJob
	held  map[string]bool
	ready map[string]cachedTranscript
	// running is whether a pass is under way, and readBytes and
	// knownBytes are how far it has got.
	running               bool
	readBytes, knownBytes int64
	reads                 atomic.Int64
}

type laterJob struct {
	path  string
	size  int64
	live  bool
	since time.Time
}

func newLaterRead(done, gate func(), lane chan struct{}) *laterRead {
	return &laterRead{done: done, pace: newPace(), gate: gate, lane: lane, held: map[string]bool{}, ready: map[string]cachedTranscript{}}
}

// LaterReads is how many transcripts the later read has read.
func (l *Loader) LaterReads() int {
	if l.later == nil {
		return 0
	}
	return int(l.later.reads.Load())
}

// hold queues a transcript's long read for the pass.
func (r *laterRead) hold(job laterJob) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.held[job.path] {
		return
	}
	r.held[job.path] = true
	r.jobs = append(r.jobs, job)
}

// holding is whether a transcript's long read is still to come.
func (r *laterRead) holding(path string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.held[path]
}

// take hands over what the pass read of a transcript, once.
func (r *laterRead) take(path string) (cachedTranscript, bool) {
	if r == nil {
		return cachedTranscript{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.ready[path]
	delete(r.ready, path)
	return entry, ok
}

// start begins the pass over what was held, taking the lane before it
// returns, so no other background pass starts ahead of it.
func (r *laterRead) start() {
	r.mu.Lock()
	jobs := r.jobs
	r.jobs = nil
	if len(jobs) == 0 || r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	r.readBytes, r.knownBytes = 0, 0
	for _, job := range jobs {
		r.knownBytes += job.size
	}
	r.mu.Unlock()
	takeLane(r.lane)
	go r.pass(jobs)
}

func (r *laterRead) pass(jobs []laterJob) {
	runtime.LockOSThread()
	backgroundPriority()
	r.pace.start()
	var before int64
	for _, job := range jobs {
		yield := func() {
			r.mu.Lock()
			r.readBytes = min(r.readBytes+(1<<20), before+job.size)
			r.mu.Unlock()
			r.pace.yield(r.gate)
		}
		entry, ok := r.read(job, yield)
		r.reads.Add(1)
		before += job.size
		r.mu.Lock()
		if ok {
			r.ready[job.path] = entry
		}
		delete(r.held, job.path)
		r.readBytes = before
		r.mu.Unlock()
		r.pace.yield(r.gate)
	}
	r.mu.Lock()
	r.running = false
	r.mu.Unlock()
	freeLane(r.lane)
	if r.done != nil {
		r.done()
	}
}

func (r *laterRead) read(job laterJob, yield func()) (cachedTranscript, bool) {
	key, err := stat(job.path)
	if err != nil {
		return cachedTranscript{}, false
	}
	if job.live {
		reader := claude.NewTranscriptReader(job.path)
		t, err := reader.ReadPaced(yield)
		if err != nil {
			return cachedTranscript{}, false
		}
		return cachedTranscript{key: key, transcript: t, reader: reader}, true
	}
	t, err := claude.ReadTranscriptSincePaced(job.path, stoppedWindowBytes, job.since, yield)
	if err != nil {
		return cachedTranscript{}, false
	}
	return cachedTranscript{key: key, transcript: t}, true
}

func (r *laterRead) progress() *ScanProgress {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.running {
		return nil
	}
	return shown(r.readBytes, r.knownBytes)
}

// takeLane and freeLane keep the loader's background passes to one at a
// time, so together they stay at one pass's share of a core (item 203).
func takeLane(lane chan struct{}) {
	if lane != nil {
		lane <- struct{}{}
	}
}

func freeLane(lane chan struct{}) {
	if lane != nil {
		<-lane
	}
}
