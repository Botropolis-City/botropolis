package render

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// panesLit is how many of a building's panes are lit at p of a
// transition: its lights go out window by window as it rests, and come
// back on as it wakes (item 86).
func panesLit(kind city.Settle, p float64, panes int) int {
	if kind == city.SettleWake {
		return int(math.Round(p * float64(panes)))
	}
	return int(math.Round((1 - p) * float64(panes)))
}

// outFirst is the order a building's panes go dark in: the top pane
// first, the lowest last, as indexes into panes.
func outFirst(panes [][4]city.Point) []int {
	order := make([]int, len(panes))
	for i := range order {
		order[i] = i
	}
	top := func(q [4]city.Point) float64 { return math.Min(q[0].Y, q[1].Y) }
	sort.SliceStable(order, func(i, j int) bool { return top(panes[order[i]]) < top(panes[order[j]]) })
	return order
}

// beaconFade is how strongly the beacon shows at p of a transition.
func beaconFade(kind city.Settle, p float64) float64 {
	if kind == city.SettleWake {
		return p
	}
	return 1 - p
}

// settleTint is the walls' light at p of a transition, from lit to the
// blinds-down tint of a stopped building, or back.
func settleTint(kind city.Settle, p float64) *ebiten.ColorScale {
	lit := float32(beaconFade(kind, p))
	tint := &ebiten.ColorScale{}
	tint.SetR(0.7 + 0.3*lit)
	tint.SetG(0.7 + 0.3*lit)
	tint.SetB(0.72 + 0.28*lit)
	return tint
}

// putOut darkens the panes that are out at p of a transition, the top
// ones first; under reduced motion every pane fades together instead.
func (g *Game) putOut(screen *ebiten.Image, cam *city.Camera, b *city.Building, name string, kind city.Settle, p float64) {
	panes := g.facadePanes(cam, name, b.Rect.Center())
	if g.reduced {
		glass := colorDarkGlass
		glass.A = uint8(float64(glass.A) * (1 - beaconFade(kind, p)))
		for _, pane := range panes {
			fillQuad(screen, pane, glass)
		}
		return
	}
	out := len(panes) - panesLit(kind, p, len(panes))
	for _, i := range outFirst(panes)[:out] {
		fillQuad(screen, panes[i], colorDarkGlass)
	}
}
