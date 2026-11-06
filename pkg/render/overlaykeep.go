package render

import (
	"image"
	"image/color"
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

// lineKey is what a line's strokes are drawn from: the form it is drawn
// in, each stroke's ends on the screen and whether it is unlit or
// dashed, and its width, dash and colours (item 348). The faulty key
// leaves the colours out, for the net's test to find a line stale.
func (g *Game) lineKey(form city.LineForm, strokes []lineStroke, width, dash float64, ink, casing color.NRGBA) uint64 {
	key := keyMix(uint64(form)+1, math.Float64bits(width), math.Float64bits(dash))
	if !g.faultyKeys {
		key = keyMix(key, uint64(ink.R)<<24|uint64(ink.G)<<16|uint64(ink.B)<<8|uint64(ink.A),
			uint64(casing.R)<<24|uint64(casing.G)<<16|uint64(casing.B)<<8|uint64(casing.A))
	}
	for _, s := range strokes {
		key = keyMix(key, math.Float64bits(s.a.X), math.Float64bits(s.a.Y), math.Float64bits(s.b.X), math.Float64bits(s.b.Y), boolBits(s.unlit), boolBits(s.dashed))
	}
	return key
}
