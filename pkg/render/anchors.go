package render

import (
	"encoding/json"
	"os"

	"botropolis.city/botropolis/pkg/city"
)

// anchors is where each named thing in the city stands in a frame of
// this size, named as a script aims at it, for the website to place its
// hotspots by (item 76). Anything outside the frame is left out.
func (g *Game) anchors(width, height float64) map[string]city.Point {
	c := g.scene.City()
	names := []string{"plant", "fountain", "library", "hall", "depot"}
	for _, t := range c.Towers {
		names = append(names, "tower:"+t.Server.Name)
	}
	for _, d := range c.Districts {
		if !d.Depot {
			names = append(names, "district:"+d.Name)
		}
	}
	out := map[string]city.Point{}
	for _, name := range names {
		p, ok := g.resolveTarget(name)
		if ok && p.X >= 0 && p.Y >= 0 && p.X < width && p.Y < height {
			out[name] = p
		}
	}
	return out
}

type anchorPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func writeAnchors(path string, anchors map[string]city.Point) error {
	out := make(map[string]anchorPoint, len(anchors))
	for name, p := range anchors {
		out[name] = anchorPoint{X: p.X, Y: p.Y}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
