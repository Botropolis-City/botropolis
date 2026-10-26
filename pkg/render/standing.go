package render

import (
	"hash/maphash"
	"math"
	"unsafe"

	"botropolis.city/botropolis/pkg/city"
)

// standingKey is what every still drawable reads alike, and each still
// thing's values that change without a snapshot, folded once a frame:
// while it holds from one frame to the next, nothing still on the map
// would be drawn otherwise (item 212). TestStillKeyReads holds every
// still drawable's key and draw to what this reads.
func (g *Game) standingKey(selected *city.Building) uint64 {
	s := g.scene
	if !g.frameBase.ok {
		g.frameBase = g.frameBaseKey()
	}
	c := s.City()
	now := s.Now()
	key := keyMix(g.frameBase.all, g.frameBase.house, g.houseWide, g.houseWideKey(),
		uint64(s.Generation()), uint64(s.GroundGeneration()), uint64(s.View()),
		boolBits(c.Night), boolBits(g.lightsOn), boolBits(g.labelsVisible()), boolBits(s.Scenery()),
		boolBits(s.Detailed()), math.Float64bits(s.PlantLights()), uint64(uintptr(unsafe.Pointer(selected))))
	// Moved by part of a pixel, nothing drawn lands where it did.
	if g.rec != nil {
		o := g.rec.origin
		frac := func(v float64) uint64 { return uint64(math.Round((v-math.Floor(v))*64)) % 64 }
		key = keyMix(key, frac(o.X), frac(o.Y))
	}
	var stirring uint64
	for id := range g.stirring {
		stirring ^= maphash.String(inkSeed, id)
	}
	key = keyMix(key, stirring)
	viewing := g.viewing()
	for _, b := range c.Buildings() {
		id := b.Session.ID
		v := keyMix(boolBits(s.Dimmed(b)), boolBits(g.unlit(b)), math.Float64bits(s.Scaffold(b)),
			boolBits(courseLit(&b.Session, now)), math.Float64bits(s.Rising(id)), math.Float64bits(s.Lights(id)),
			math.Float64bits(s.Mowing(id)))
		if site, ok := s.Construction(id); ok {
			v = keyMix(v, hashOf(&site))
		}
		if viewing {
			if tint, ok := g.viewTint(b); ok {
				v = keyMix(v, 1, uint64(tint.R)<<24|uint64(tint.G)<<16|uint64(tint.B)<<8|uint64(tint.A))
			}
		}
		for i := range min(b.Flags, 3) {
			v = keyMix(v, math.Float64bits(g.flagHoist(b, i)))
		}
		key = keyMix(key, v)
	}
	return key
}
