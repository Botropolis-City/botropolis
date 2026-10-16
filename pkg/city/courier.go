package city

import "time"

// The refresh courier. Item 56.
//
// Aria, on r265: "Still don't see the refresh boat (to be clear the
// usage boats are there just not the refresh boat when I press u)."
//
// She was looking for something bug 54 removed. Until then a refresh
// put a boat on the river, but only by accident: `claude -p` is a real
// Claude Code session while it runs, so the probe got a tug the way any
// session does — and that is exactly why it dragged the map around and
// had to go. The event deserved a boat; the accident was not the only
// way to get one.
//
// A Courier is not a session. It has no district, no dock, no entry in
// snapshot.Sessions, and nothing in it reaches the plan — so it cannot
// move the river, because it is not part of what sizes the map.
// Everything bugs 54 and 54a fixed stays fixed.
//
// It also does not track the subprocess. Aria: "Even if the command
// completes, we should still render the boat all the way across." The
// probe finishes in about a second, and a boat that vanished a second
// after launching would be the despawn she already rejected. It means
// "you asked for fresh numbers", not "a subprocess is running".
type Courier struct {
	StartedAt time.Time
	// From and To are refreshed by Couriers from the live RiverBand, as
	// voyages are. Written that way from the start rather than fixed
	// later: bug 54a was a boat holding points captured at launch while
	// the river moved out from under it.
	From, To Point
}

// CourierFor is how long the courier takes to run the river end to end.
//
// Shorter than SailFor over a longer distance, so it visibly outruns
// the tugs: the hull was picked for being low and slender against their
// tall and squat, and motion is the other half of telling them apart.
const CourierFor = 5 * time.Second

// Progress is how far along the river the courier is, 0 to 1.
func (c *Courier) Progress(now time.Time) float64 {
	p := float64(now.Sub(c.StartedAt)) / float64(CourierFor)
	return min(1, max(0, p))
}

// At is where the courier is now.
func (c *Courier) At(now time.Time) Point {
	p := c.Progress(now)
	return Point{X: c.From.X + (c.To.X-c.From.X)*p, Y: c.From.Y + (c.To.Y-c.From.Y)*p}
}

// Card is what the courier says when pointed at.
//
// It teaches the key rather than just naming itself. Aria had not
// realised the refresh existed at all, so the status line alone was not
// enough to tell her the key was there; the first time she points at
// one of these it should say what pressing u does.
func (c *Courier) Card() Card {
	return Card{Title: "fetching your usage", Lines: []string{
		"sent     because you pressed " + RefreshKey,
		"brings   fresh limits for the boats to show",
		"costs    about four seconds, and is rate limited",
	}}
}

// SendCourier puts one on the river. Called when a refresh actually
// goes out, not when the key is pressed: a press inside the debounce
// fetches nothing, so it launches nothing, and the status line says so.
func (s *Scene) SendCourier() {
	s.couriers = append(s.couriers, &Courier{StartedAt: s.now()})
}

// Couriers is every refresh boat still running, with its ends resolved
// against the river as it stands now.
func (s *Scene) Couriers() []*Courier {
	now := s.now()
	kept := s.couriers[:0]
	for _, c := range s.couriers {
		if c.Progress(now) >= 1 {
			continue
		}
		kept = append(kept, c)
	}
	s.couriers = kept
	// The courier runs the river's own length, head to foot, not out to
	// sea: it is a usage refresh, not an arrival.
	head, mouth, _, ok := s.city.RiverBand()
	if !ok {
		return nil
	}
	for _, c := range s.couriers {
		c.From, c.To = head, mouth
	}
	return append([]*Courier(nil), s.couriers...)
}
