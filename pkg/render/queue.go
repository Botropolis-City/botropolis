package render

import (
	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
)

// queues is every customer queued at a stall, one per subagent in
// flight, for the depth sort: what a building's drones say, a stall's
// queue says, and it shows wherever the drones would (item 273).
func (g *Game) queues(screen **ebiten.Image, c *city.City, cam *city.Camera) []drawable {
	if !g.shows(city.NetworkCranes) {
		return nil
	}
	var out []drawable
	for _, b := range c.Buildings() {
		if !b.Stall() || b.Vacant || g.scene.Dimmed(b) {
			continue
		}
		for i, at := range g.scene.Queue(b) {
			name := kitCustomers[(len(b.Session.ID)+i)%len(kitCustomers)]
			at := at
			out = append(out, footprintAt(cam, city.RectAt(at.X-0.5, at.Y-0.5, 1, 1), func() {
				g.kitAt(*screen, cam, name, 0, cam.WorldToScreen(at), nil)
			}))
		}
	}
	return out
}
