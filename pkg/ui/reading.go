package ui

import (
	"fmt"
	"time"

	"botropolis.city/botropolis/pkg/state"
)

// rateSettles is how long a scan's rate is watched before the band
// estimates the time left, and rateWindow how far back it looks: the
// paced read starts and stops, so a few seconds say little (item 159).
const (
	rateSettles = 20 * time.Second
	rateWindow  = time.Minute
)

// ReadingLine is the reading band's line while the history is read: how far,
// how long is left once the rate has settled, and what fills in.
func ReadingLine(scan state.ScanProgress, left time.Duration, settled bool) string {
	percent := int64(0)
	if scan.Known > 0 {
		percent = min(scan.Done*100/scan.Known, 99)
	}
	line := fmt.Sprintf("Reading your history · %d%%", percent)
	if settled {
		if left < time.Minute {
			line += " · under a minute left"
		} else {
			line += fmt.Sprintf(" · about %d min left", int((left+time.Minute-1)/time.Minute))
		}
	}
	return line + " · statistics fill in as it reads"
}

// ScanRate watches a scan's bytes read over time.
type ScanRate struct {
	samples []rateSample
}

type rateSample struct {
	at   time.Time
	done int64
}

// Observe notes that done bytes had been read at at; fewer than last
// time is a new scan, watched from the start.
func (r *ScanRate) Observe(at time.Time, done int64) {
	if n := len(r.samples); n > 0 {
		switch last := r.samples[n-1].done; {
		case done == last:
			return
		case done < last:
			r.samples = nil
		}
	}
	r.samples = append(r.samples, rateSample{at: at, done: done})
	for len(r.samples) > 2 && at.Sub(r.samples[1].at) >= rateWindow {
		r.samples = r.samples[1:]
	}
}

// Remaining is how long the rest of known will take at the rate
// watched, and whether that rate has settled enough to say.
func (r *ScanRate) Remaining(done, known int64) (time.Duration, bool) {
	if len(r.samples) < 2 {
		return 0, false
	}
	first, last := r.samples[0], r.samples[len(r.samples)-1]
	span := last.at.Sub(first.at)
	read := last.done - first.done
	if span < rateSettles || read <= 0 {
		return 0, false
	}
	return time.Duration(float64(max(known-done, 0)) / float64(read) * float64(span)), true
}
