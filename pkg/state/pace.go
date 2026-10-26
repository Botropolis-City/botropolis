package state

import "time"

// backgroundShare is the fraction of the wall clock a background pass
// may work for. Half a core leaves the other half of one for the
// collector and the loads, so a first launch takes at most one core in
// all whatever GOMAXPROCS is (item 159).
const backgroundShare = 0.5

// pace keeps a background pass to a fraction of the wall clock: at each
// yield it sleeps in proportion to the work done since the last one.
// Time spent waiting, at the load gate or asleep, is not work.
type pace struct {
	fraction float64
	now      func() time.Time
	sleep    func(time.Duration)
	since    time.Time
}

func newPace() *pace {
	return &pace{fraction: backgroundShare, now: time.Now, sleep: time.Sleep}
}

// lower gives the calling thread background priority, unless the pass
// may work for the whole clock: that is a capture's, which waits on it,
// and in the idle I/O class its every commit waited behind the rest of
// a busy disk (item 254).
func (p *pace) lower() {
	if p.fraction < 1 {
		backgroundPriority()
	}
}

func (p *pace) start() {
	p.since = p.now()
}

func (p *pace) yield(wait func()) {
	if work := p.now().Sub(p.since); work > 0 && p.fraction < 1 {
		p.sleep(time.Duration(float64(work) * (1 - p.fraction) / p.fraction))
	}
	wait()
	p.start()
}
