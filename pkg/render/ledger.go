package render

import "botropolis.city/botropolis/pkg/stats"

// setStatsOn turns statistics on or off for this window: on, the city
// reads its usage through a ledger over the statistics store, read-only,
// so the daemon stays its only writer; off, it has none and says so.
// Item 116.
func (g *Game) setStatsOn(on bool) {
	g.stats.on = on
	switch {
	case on && g.ledger == nil:
		g.ledger = stats.NewLedger(stats.Path())
		g.scene.SetLedger(g.ledger)
	case !on && g.ledger != nil:
		g.ledger.Close()
		g.ledger = nil
		g.scene.SetLedger(nil)
	}
}
