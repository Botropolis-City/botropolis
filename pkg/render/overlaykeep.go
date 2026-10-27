package render

import (
	"image"
	"math"

	"botropolis.city/botropolis/pkg/city"
)

// keepOverlay draws part of the overlay, the layer over everything else,
// as noteKeyed notes a still thing: while key holds from the last frame,
// its commands are copied from the last frame's record rather than drawn
// again (item 212). The net checks it as it checks a kept record. Only
// while the overlay is recorded, and not while the ground recedes under a
// hovered car, since it is drawn into fade groups then.
func (g *Game) keepOverlay(kind recordKind, key uint64, draw func()) {
	s := &g.still
	if !g.overlaying || g.noReuse || g.noStill || g.lit.RecedesGround() {
		draw()
		return
	}
	l, k := g.rec, &s.kept
	key = keyMix(key, uint64(kind))
	r, found := s.lastLines[key]
	found = found && r.to <= len(k.cmds)
	if found && !g.checks(key) {
		from := len(l.cmds)
		g.copyCmds(k, r.from, r.to, image.Point{})
		l.adopt(from)
		s.lines[key] = inked{from: from, to: len(l.cmds)}
		s.stats.linesKept++
		return
	}
	from := len(l.cmds)
	draw()
	fresh := inked{from: from, to: len(l.cmds)}
	s.lines[key] = fresh
	s.stats.linesDrawn++
	if !found {
		return
	}
	// Checked: the copy is made beside what was drawn, compared, and let go.
	s.stats.checked++
	mark := l.mark()
	at := len(l.cmds)
	g.copyCmds(k, r.from, r.to, image.Point{})
	same := len(l.cmds)-at == fresh.to-fresh.from
	for j := 0; same && j < fresh.to-fresh.from; j++ {
		same = l.sameCmd(&l.cmds[at+j], &l.cmds[fresh.from+j])
	}
	l.truncate(mark)
	if !same {
		s.stats.stale++
		g.staleKey(kind)
	}
}

// lineKey is what a line's strokes are drawn from: its stations' and its
// office's points on the screen, which of its legs are dashed, and its
// width, dash and colours. The faulty key leaves the colours out, for the
// net's test to find a line stale.
func (g *Game) lineKey(l *city.Line, points []city.Point, office city.Point, width, dash float64, ink, casing [4]uint8) uint64 {
	key := keyMix(math.Float64bits(width), math.Float64bits(dash), math.Float64bits(office.X), math.Float64bits(office.Y), boolBits(l.Office != nil))
	if !g.faultyKeys {
		key = keyMix(key, uint64(ink[0])<<24|uint64(ink[1])<<16|uint64(ink[2])<<8|uint64(ink[3]),
			uint64(casing[0])<<24|uint64(casing[1])<<16|uint64(casing[2])<<8|uint64(casing[3]))
	}
	for i, p := range points {
		key = keyMix(key, math.Float64bits(p.X), math.Float64bits(p.Y), boolBits(l.Stations[i].Dashed))
	}
	return key
}
