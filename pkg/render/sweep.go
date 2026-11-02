package render

import (
	"fmt"
	"time"

	"botropolis.city/botropolis/pkg/state"
)

// quietIDs is what Pack up everything quiet would pack: the sessions
// the map shows in no group, quiet for at least quiet_days (item 333).
func (g *Game) quietIDs() []string {
	var sessions []state.Session
	for _, b := range g.scene.City().Buildings() {
		sessions = append(sessions, b.Session)
	}
	return state.Quiet(sessions, g.scene.Now(), g.quietDays())
}

func (g *Game) quietDays() time.Duration {
	if g.quietFor <= 0 {
		return state.DefaultThresholds.Shuttered
	}
	return g.quietFor
}

// quietLabel is City Hall's button, which says the count before the
// sweep.
func quietLabel(n int) string {
	switch n {
	case 0:
		return "Nothing quiet to pack up"
	case 1:
		return "Pack up 1 quiet session"
	}
	return fmt.Sprintf("Pack up %d quiet sessions", n)
}

// sweepQuiet packs up everything quiet in one move, beside the loop, and
// says how many went to the depot (item 333).
func (g *Game) sweepQuiet() {
	ids, quietFor := g.quietIDs(), g.quietDays()
	if len(ids) == 0 {
		g.SetStatus(fmt.Sprintf("nothing in no group has been quiet for %s", daysPhrase(quietFor)))
		return
	}
	if g.sweep == nil {
		g.SetStatus("packing up needs the daemon, which keeps the ledger of what is packed; start botropolisd")
		return
	}
	sweep := g.sweep
	g.SetStatus(fmt.Sprintf("packing up %s…", quietCount(len(ids))))
	g.acting.Add(1)
	go func() {
		defer g.acting.Done()
		packed, err := sweep(ids, quietFor)
		if err != nil {
			g.SetStatus(err.Error())
			return
		}
		g.SetStatus(fmt.Sprintf("packed up %s; unpack any from the depot", quietCount(packed)))
	}()
}

// sweptSince is how many sessions the quiet sweep packed up since a time
// that are still in the depot, for the journal's since strip.
func sweptSince(sessions []state.Session, since time.Time) int {
	n := 0
	for i := range sessions {
		if sessions[i].Packed && sessions[i].Swept && !sessions[i].PackedAt.Before(since) {
			n++
		}
	}
	return n
}

// withSwept adds the sweep's count to the since strip, when it did any.
func withSwept(strip string, n int) string {
	if n == 0 {
		return strip
	}
	return fmt.Sprintf("%s  ·  %d packed up as quiet", strip, n)
}

func quietCount(n int) string {
	if n == 1 {
		return "1 quiet session"
	}
	return fmt.Sprintf("%d quiet sessions", n)
}

func daysPhrase(d time.Duration) string {
	days := int(d / (24 * time.Hour))
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}
