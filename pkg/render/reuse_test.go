package render

import (
	"fmt"
	"image"
	"math"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 212: a still drawable whose key holds keeps last frame's record,
// moved by the frame's whole-pixel pan, and is recorded afresh when
// anything it is drawn from changes.
func TestRecordReuse(t *testing.T) {
	// settled is a record city drawn until it keeps everything it can,
	// each kept record checked against one made afresh.
	settled := func(t *testing.T) *recordCity {
		rc := newRecordCity(t)
		rc.g.verifyEvery = 1
		rc.scene.SetView(city.ViewAttention)
		rc.frame()
		// Past the flags' hoisting and the course's glow, which move.
		rc.now = rc.now.Add(10 * time.Minute)
		for range 3 {
			rc.frame()
		}
		require.Positive(t, rc.frame().reused, "precondition: the settled city keeps records")
		return rc
	}

	t.Run("when a city has settled, and 30 frames pass with nothing changing", func(t *testing.T) {
		rc := settled(t)
		var total stillStats
		for range 30 {
			st := rc.frame()
			total.fresh += st.fresh
			total.stale += st.stale
		}

		t.Run("it should record no still drawable afresh", func(t *testing.T) {
			assert.Zero(t, total.fresh)
		})
		t.Run("it should hold the record made afresh", func(t *testing.T) {
			assert.Zero(t, total.stale)
		})
	})

	t.Run("when a city has settled, its standing objects", func(t *testing.T) {
		rc := settled(t)
		st := rc.frame()
		for _, kind := range []recordKind{kindMonument, kindTree, kindBridge} {
			t.Run("it should keep "+kind.String()+"'s records", func(t *testing.T) {
				assert.Positive(t, st.keptBy[kind])
			})
		}
		t.Run("it should record none of them afresh", func(t *testing.T) {
			assert.Zero(t, st.freshBy[kindMonument]+st.freshBy[kindTree]+st.freshBy[kindBridge])
		})
	})

	t.Run("when a city has settled zoomed in, where its districts' signs stand legible", func(t *testing.T) {
		rc := newRecordCity(t)
		rc.g.verifyEvery = 1
		rc.scene.SetView(city.ViewAttention)
		rc.scene.Camera().Zoom = 2
		rc.frame()
		rc.now = rc.now.Add(10 * time.Minute)
		for range 4 {
			rc.frame()
		}
		st := rc.frame()

		t.Run("it should keep the signs' records", func(t *testing.T) {
			assert.Positive(t, st.keptBy[kindSign])
		})
		t.Run("it should record no sign afresh", func(t *testing.T) {
			assert.Zero(t, st.freshBy[kindSign])
		})
	})

	t.Run("when the camera pans by whole pixels", func(t *testing.T) {
		rc := settled(t)
		rc.scene.Pan(city.Point{X: 7, Y: -3})
		st := rc.frame()

		t.Run("it should record no still drawable afresh", func(t *testing.T) {
			assert.Zero(t, st.fresh)
		})
		t.Run("it should hold the record made afresh", func(t *testing.T) {
			assert.Zero(t, st.stale)
		})
	})

	for _, c := range []struct {
		name string
		move func(rc *recordCity)
	}{
		{"pans by part of a pixel", func(rc *recordCity) { rc.scene.Pan(city.Point{X: 0.5, Y: 0.25}) }},
		{"zooms", func(rc *recordCity) { rc.scene.Camera().Zoom *= 1.25 }},
	} {
		t.Run("when the camera "+c.name, func(t *testing.T) {
			rc := settled(t)
			c.move(rc)
			st := rc.frame()

			t.Run("it should keep no record", func(t *testing.T) {
				assert.Zero(t, st.reused)
			})
		})
	}

	t.Run("when one working session's activity moves in a snapshot", func(t *testing.T) {
		rc := settled(t)
		rc.now = rc.now.Add(time.Second)
		rc.snap(func(s *state.Snapshot) {
			session(s, "s00").LastActivity = s.At.Add(-time.Second)
		})
		st := rc.frame()

		t.Run("it should record afresh only that session's lot and building", func(t *testing.T) {
			assert.Equal(t, 2, st.fresh)
		})
		t.Run("it should hold the record made afresh", func(t *testing.T) {
			assert.Zero(t, st.stale)
		})
	})

	t.Run("when the camera has panned, and a frame is drawn straight between two that are not, as the frame check draws them", func(t *testing.T) {
		rc := settled(t)
		rc.scene.Pan(city.Point{X: 5, Y: 4})
		rc.frame()
		rc.g.noStill = true
		rc.frame()
		rc.g.noStill = false
		st := rc.frame()

		t.Run("it should record no still drawable afresh", func(t *testing.T) {
			assert.Zero(t, st.fresh)
		})
		t.Run("it should hold the record made afresh", func(t *testing.T) {
			assert.Zero(t, st.stale)
		})
	})

	t.Run("when a snapshot rebuilds the city and changes nothing a building is drawn from", func(t *testing.T) {
		rc := settled(t)
		rc.snap(func(*state.Snapshot) {})
		st := rc.frame()
		current := map[*city.Building]bool{}
		for _, b := range rc.scene.City().Buildings() {
			current[b] = true
		}
		stray := 0
		for _, h := range rc.g.hits {
			if h.hit.Building != nil && !current[h.hit.Building] {
				stray++
			}
		}

		t.Run("it should keep the buildings' records", func(t *testing.T) {
			assert.Zero(t, st.freshBy[kindHouse])
		})
		t.Run("it should note hits on this snapshot's buildings only", func(t *testing.T) {
			assert.Zero(t, stray)
		})
	})

	t.Run("when a city has settled at night", func(t *testing.T) {
		rc := newRecordCity(t)
		rc.g.verifyEvery = 1
		rc.scene.SetView(city.ViewAttention)
		for !rc.scene.City().Night {
			rc.scene.CycleLight()
		}
		for range 4 {
			rc.frame()
		}
		st := rc.frame()

		t.Run("it should record no still building afresh", func(t *testing.T) {
			assert.Zero(t, st.freshBy[kindHouse])
		})
		t.Run("it should keep some building's record", func(t *testing.T) {
			assert.Positive(t, st.keptBy[kindHouse])
		})
		t.Run("it should lay the lights laid afresh", func(t *testing.T) {
			assert.Zero(t, st.stale)
		})
	})

	t.Run("when a look is drawn straight because its key is new", func(t *testing.T) {
		rc := settled(t)
		bs := rc.scene.City().Buildings()
		rc.scene.Select(bs[len(bs)/2].Session.ID)
		bakes := 0
		for range 5 {
			rc.frame()
			bakes += rc.g.lookStats.bakes
		}

		t.Run("it should record it afresh until the look is baked", func(t *testing.T) {
			assert.Positive(t, bakes)
		})
	})

	t.Run("when the look pages have no room, so buildings are drawn straight", func(t *testing.T) {
		rc := newRecordCity(t)
		rc.g.verifyEvery = 1
		rc.scene.SetView(city.ViewAttention)
		// The first frame starts the look cache, and its pages with it.
		rc.frame()
		rc.g.lookPages = newLookPages(1, 160)
		for range 6 {
			rc.frame()
		}
		require.True(t, rc.g.lookPages.full || rc.g.lookStats.misses > 0, "precondition: some look finds no room")
		st := rc.frame()

		t.Run("it should keep their records", func(t *testing.T) {
			assert.Zero(t, st.freshBy[kindHouse])
		})

		t.Run("and a page starts again, so there is room", func(t *testing.T) {
			rc.g.lookPages.full = true
			rc.g.lookPages.next(rc.g.lookFrame+10, rc.g.looks.remove)
			st := rc.frame()

			t.Run("it should record them afresh", func(t *testing.T) {
				assert.Positive(t, st.freshBy[kindHouse])
			})
		})
	})

	t.Run("when the looks a kept record reads are evicted", func(t *testing.T) {
		rc := settled(t)
		require.Positive(t, rc.frame().keptBy[kindHouse], "precondition: buildings keep their records")
		for _, k := range rc.g.looks.keys() {
			rc.g.looks.remove(k)
		}
		st := rc.frame()

		t.Run("it should record those buildings afresh", func(t *testing.T) {
			assert.Positive(t, st.freshBy[kindHouse])
		})
		t.Run("it should hold the record made afresh", func(t *testing.T) {
			assert.Zero(t, st.stale)
		})
	})

	t.Run("when records are kept unchecked at night, the camera panned by whole pixels", func(t *testing.T) {
		rc := newRecordCity(t)
		rc.g.verifyEvery = 0
		rc.scene.SetView(city.ViewAttention)
		for !rc.scene.City().Night {
			rc.scene.CycleLight()
		}
		for range 4 {
			rc.frame()
		}
		rc.scene.Pan(city.Point{X: 6, Y: -5})
		rc.frame()
		st := rc.frame()
		kept := frameNotes(rc.g)
		rc.g.noReuse = true
		rc.frame()
		afresh := frameNotes(rc.g)

		t.Run("it should keep some building's record", func(t *testing.T) {
			require.Positive(t, st.keptBy[kindHouse])
		})
		t.Run("it should note what the frame made afresh notes", func(t *testing.T) {
			assert.Equal(t, afresh, kept)
		})
	})

	t.Run("when a working session's turn ends for you to answer, and its scaffold comes down with nothing else changing", func(t *testing.T) {
		rc := settled(t)
		rc.snap(func(s *state.Snapshot) {
			s.At = rc.now
			session(s, "s00").State = state.Unattended
		})
		// The new look is drawn straight, then baked, then kept.
		for range 3 {
			rc.frame()
		}
		rc.now = rc.now.Add(300 * time.Millisecond)
		st := rc.frame()

		t.Run("it should hold the record made afresh", func(t *testing.T) {
			assert.Zero(t, st.stale)
		})
	})

	t.Run("when reuse is off", func(t *testing.T) {
		rc := settled(t)
		rc.g.noReuse = true
		st := rc.frame()

		t.Run("it should keep no record", func(t *testing.T) {
			assert.Zero(t, st.reused)
		})
	})
}

// Item 212: what a settled frame of a city with aged lots costs with its
// still drawables' records kept, and recorded afresh. It reports and
// never gates.
func BenchmarkASettledFrameKeepingRecords(b *testing.B) {
	for _, c := range []struct {
		name  string
		fresh bool
	}{{"kept", false}, {"afresh", true}} {
		b.Run(c.name, func(b *testing.B) {
			rc := newRecordCity(b)
			rc.g.verifyEvery = releaseVerifyEvery
			rc.g.noReuse = c.fresh
			for range 3 {
				rc.frame()
			}
			b.ReportAllocs()
			for b.Loop() {
				rc.frame()
			}
		})
	}
}

// Item 212: Ebitengine lets a sub-image it cut go from its cache after
// 60 ticks unused, without disposing it, so a sprite cut again is a new
// image of the same pixels, while a kept record still holds the old one.
func TestASpriteCutAgain(t *testing.T) {
	page := ebiten.NewImage(64, 64)
	other := ebiten.NewImage(64, 64)
	t.Cleanup(page.Deallocate)
	t.Cleanup(other.Deallocate)
	r := image.Rect(8, 8, 24, 24)
	record := func(img, from *ebiten.Image) drawCmd {
		l := newDrawList()
		l.begin()
		l.ImageOn(l.dry, img, from, &ebiten.DrawImageOptions{})
		l.end()
		return l.cmds[0]
	}
	// Another image of the same rect, as Ebitengine hands back once it
	// has let the first go; Ebitengine's tick, which lets it go, does not
	// run in a test, so it is cut from another page and recorded as cut
	// from the first.
	was, now := record(page.SubImage(r).(*ebiten.Image), page), record(other.SubImage(r).(*ebiten.Image), page)

	t.Run("when a sprite is cut from its page again", func(t *testing.T) {
		t.Run("it should hash the same", func(t *testing.T) {
			assert.Equal(t, was.hash, now.hash)
		})
		t.Run("it should draw the same", func(t *testing.T) {
			assert.True(t, (&drawList{}).sameCmd(&was, &now))
		})
	})

	t.Run("when a sprite is cut from another page", func(t *testing.T) {
		elsewhere := record(other.SubImage(r).(*ebiten.Image), other)

		t.Run("it should hash otherwise", func(t *testing.T) {
			assert.NotEqual(t, was.hash, elsewhere.hash)
		})
	})
}

// frameNotes is what a frame noted besides drawing, positions to a
// 64th of a pixel: the night's light ops, their panes, points and
// vertices, the glows, the counts and the hits.
func frameNotes(g *Game) []string {
	q := func(v float64) int64 { return int64(math.Round(v * 64)) }
	var out []string
	add := func(format string, a ...any) { out = append(out, fmt.Sprintf(format, a...)) }
	for _, op := range g.lightOps {
		add("op punch=%v cut=%d warm=%v box=%v span=%d alpha=%v words=%q geo=%d,%d", op.punch, op.cut, op.warm, op.box, op.to-op.from, op.alpha, op.words,
			q(op.geo.Element(0, 2)), q(op.geo.Element(1, 2)))
		for _, v := range op.vertices {
			add(" v %d,%d", q(float64(v.DstX)), q(float64(v.DstY)))
		}
	}
	for _, r := range g.litRects {
		add("pane %v", r)
	}
	for _, p := range g.cutPoints {
		add("point %d,%d", q(p.X), q(p.Y))
	}
	for _, v := range g.cutVertices {
		add("cut vertex %d,%d", q(float64(v.DstX)), q(float64(v.DstY)))
	}
	for _, p := range g.glows {
		add("glow %d,%d", q(p.X), q(p.Y))
	}
	add("counts windows=%d landmarks=%d punches=%d cuts=%d buildings=%d world=%d",
		g.windowDraws, g.landmarkDraws, g.punches, g.cuts, g.buildingsDrawn, g.drawing.n)
	for _, h := range g.hits {
		add("hit %v %p %p", h.rect, h.hit.Building, h.hit.District)
	}
	return out
}

// keys is every key the cache holds.
func (c *lookCache[K, V]) keys() []K {
	out := make([]K, 0, len(c.entries))
	for k := range c.entries {
		out = append(out, k)
	}
	return out
}
