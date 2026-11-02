package render

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// trafficFrameSize is the size the traffic frames are drawn at.
var trafficFrameSize = image.Point{X: 1600, Y: 900}

// trafficHeading turns the city so the plant stands clear of the road.
var trafficHeading = 180

// TrafficFrames draws a road carrying all four kinds of its traffic — a
// car, a van, a box truck and a motorbike — at the fit view and at full
// zoom, by day and by night, and writes each to a PNG in dir. Item 255.
func TrafficFrames(dir string) ([]string, error) {
	var wrote []string
	err := runSheet(func(*Game) error {
		img := ebiten.NewImageWithOptions(image.Rectangle{Max: trafficFrameSize}, &ebiten.NewImageOptions{Unmanaged: true})
		defer img.Deallocate()
		for _, light := range []struct {
			name string
			hour int
		}{{"day", 14}, {"night", 22}} {
			home, err := os.MkdirTemp("", "botropolis-traffic-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(home) }()
			now := time.Now()
			at := time.Date(now.Year(), now.Month(), now.Day(), light.hour, 0, 0, 0, time.Local)
			d, err := NewDrive(DriveOptions{Home: home, Now: at, Width: trafficFrameSize.X, Height: trafficFrameSize.Y})
			if err != nil {
				return err
			}
			g := d.game
			g.scene.Camera().Heading = trafficHeading
			g.Offer(trafficCity(at, 0))
			for range liveTPS * 20 {
				if err := d.Tick(); err != nil {
					return err
				}
			}
			g.Offer(trafficCity(at, 1))
			for range liveTPS * 11 / 10 {
				if err := d.Tick(); err != nil {
					return err
				}
			}
			s := g.scene
			for _, zoom := range []struct {
				name string
				set  func()
			}{
				{"fit", s.Fit},
				{"full-zoom", func() {
					on := s.Traffic()
					if len(on) == 0 {
						return
					}
					s.Camera().Zoom = city.MaxZoom
					s.CenterOn(on[len(on)/2].At(s.Clock()))
				}},
			} {
				zoom.set()
				if err := g.Update(); err != nil {
					return err
				}
				path := filepath.Join(dir, fmt.Sprintf("item-255-%s-%s.png", zoom.name, light.name))
				dumpPNG(path, g.worldFrame(img, true))
				wrote = append(wrote, path)
			}
		}
		return nil
	})
	return wrote, err
}

// trafficCity is two working sessions in two projects, a session working
// across them, and n of each of the road's other three kinds of traffic.
func trafficCity(at time.Time, n int) state.Snapshot {
	working := func(id, cwd, title string) state.Session {
		return state.Session{ID: id, CWD: cwd, State: state.Working, Title: title, Model: "claude-opus-5",
			ContextPercent: 40, ContextTokens: 400_000, ContextWindow: 1_000_000,
			Usage:     claude.Usage{Input: 40_000, Output: 20_000, CacheRead: 900_000, Messages: 60},
			StartedAt: at.Add(-2 * time.Hour), LastActivity: at.Add(-time.Minute)}
	}
	return state.Snapshot{At: at,
		Sessions: []state.Session{
			working("a", "/home/demo/src/tidepool", "Ask kiln about its reader"),
			working("b", "/home/demo/src/kiln", "Kiln reader"),
			working("c", "/home/demo/src/tidepool", "Fix the table timezone"),
		},
		Roads: []state.Road{{From: "/home/demo/src/tidepool", To: "/home/demo/src/kiln",
			Messages: n, Files: n, Visits: n, Sessions: 2, Active: 1}},
	}
}
