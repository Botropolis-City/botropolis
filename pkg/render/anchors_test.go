package render

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Item 76: the website places a hotspot over each thing a still shows,
// by hand, until the film says where each one landed.
func TestAnchors(t *testing.T) {
	at := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	newGame := func(width, height float64) *Game {
		var sessions []state.Session
		for i := range 4 {
			cwd := fmt.Sprintf("/p/d%d", i)
			sessions = append(sessions, state.Session{ID: cwd, CWD: cwd, State: state.Working, StartedAt: at, LastActivity: at})
		}
		sessions = append(sessions, state.Session{ID: "packed", CWD: "/p/d0", State: state.Resting,
			StartedAt: at.Add(-48 * time.Hour), LastActivity: at.Add(-40 * time.Hour), Packed: true, PackedAt: at.Add(-time.Hour)})
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return at }, time.UTC)
		s.SetInstant(true)
		s.Resize(width, height)
		s.SetSnapshot(state.Snapshot{At: at, Sessions: sessions,
			Stats:   &claude.Stats{},
			Servers: []state.Server{{Name: "tracker", Calls: 3, Sessions: 1}, {Name: "weather", Calls: 1, Sessions: 1}}})
		return &Game{scene: s, scripted: -1, clicked: -1}
	}

	// names is what hit-testing a point finds, in the anchors' own words.
	names := func(h city.Hit) string {
		switch {
		case h.Tower != nil:
			return "tower:" + h.Tower.Server.Name
		case h.Landmark != city.LandmarkNone:
			return string(h.Landmark)
		case h.District != nil && h.District.Depot:
			return "depot"
		case h.District != nil:
			return "district:" + h.District.Name
		}
		return ""
	}

	t.Run("when a still is taken of the city", func(t *testing.T) {
		g := newGame(1600, 1000)
		anchors := g.anchors(1600, 1000)
		want := []string{"plant", "fountain", "library", "hall", "depot", "tower:tracker", "tower:weather"}
		for _, d := range g.scene.City().Districts {
			if !d.Depot {
				want = append(want, "district:"+d.Name)
			}
		}
		require.Greater(t, len(want), 7, "the scene has no districts to anchor")

		for _, name := range want {
			t.Run("it should name an anchor for "+name, func(t *testing.T) {
				assert.Contains(t, anchors, name)
			})

			t.Run("it should sit "+name+"'s anchor on it", func(t *testing.T) {
				p, ok := anchors[name]
				require.True(t, ok)
				assert.Equal(t, name, names(g.scene.City().At(g.scene.Camera().ScreenToWorld(p))))
			})
		}

		t.Run("and an object lies outside the frame", func(t *testing.T) {
			const width, height = 700, 420
			var off []string
			for name, p := range g.anchors(width, height) {
				if p.X < 0 || p.Y < 0 || p.X >= width || p.Y >= height {
					off = append(off, name)
				}
			}

			t.Run("it should leave that object out", func(t *testing.T) {
				assert.Empty(t, off)
			})

			t.Run("it should keep what is inside", func(t *testing.T) {
				assert.Contains(t, g.anchors(width, height), "tower:tracker")
			})
		})
	})

	t.Run("when the anchors are written", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "still.anchors.json")
		require.NoError(t, writeAnchors(path, map[string]city.Point{"plant": {X: 812.4, Y: 403.6}}))
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		var back map[string]struct{ X, Y float64 }
		require.NoError(t, json.Unmarshal(data, &back))

		t.Run("it should write JSON a reader can load back", func(t *testing.T) {
			assert.Equal(t, map[string]struct{ X, Y float64 }{"plant": {X: 812.4, Y: 403.6}}, back)
		})

		t.Run("it should name the fields x and y", func(t *testing.T) {
			assert.True(t, strings.Contains(string(data), `"x"`) && strings.Contains(string(data), `"y"`))
		})
	})
}
