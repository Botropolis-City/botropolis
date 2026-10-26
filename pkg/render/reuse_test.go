package render

import (
	"fmt"
	"image"
	"math"
	"reflect"
	"slices"
	"testing"
	"time"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
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
		for _, kind := range []recordKind{kindMonument, kindTree, kindBridge, kindFog} {
			t.Run("it should keep "+kind.String()+"'s records", func(t *testing.T) {
				assert.Positive(t, st.keptBy[kind])
			})
		}
		t.Run("it should record none of them afresh", func(t *testing.T) {
			assert.Zero(t, st.freshBy[kindMonument]+st.freshBy[kindTree]+st.freshBy[kindBridge]+st.freshBy[kindFog])
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

	t.Run("when a city like the real history has settled, its buildings quiet", func(t *testing.T) {
		d := fullZoom(t, DriveOptions{})
		d.game.verifyEvery = 1
		for range 3 {
			require.NoError(t, d.Tick())
		}
		require.NoError(t, d.Tick())
		st := d.game.still.stats

		t.Run("it should work out no building's frame again", func(t *testing.T) {
			assert.Zero(t, st.framesWorked)
		})
		t.Run("it should hold the record made afresh", func(t *testing.T) {
			assert.Zero(t, st.stale)
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
		rc.g.verifyEvery, rc.g.verifyBudget = 0, 0
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

	t.Run("when a city like the real history zooms all the way in, checked as a release checks, so records are kept between checks", func(t *testing.T) {
		now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.Local)
		d, err := NewDrive(DriveOptions{Home: helpers.ParkedWeek(t, 217, now, "Full zoom"), Now: now, Width: 1920, Height: 1200})
		require.NoError(t, err)
		d.game.verifyEvery = 0
		require.NoError(t, d.Run(30))
		d.game.scene.Wheel(city.Point{X: 960, Y: 600}, 40)
		stale := 0
		for range 120 {
			require.NoError(t, d.Tick())
			stale += d.game.still.stats.stale
		}

		t.Run("it should find no kept record that differs from the one made afresh", func(t *testing.T) {
			assert.Zero(t, stale)
		})
	})

	t.Run("when a city like the real history has settled at full zoom, many of its looks baked running off the frame", func(t *testing.T) {
		d := fullZoom(t, DriveOptions{})
		d.game.verifyEvery = 1
		for range 3 {
			require.NoError(t, d.Tick())
		}
		require.NoError(t, d.Tick())
		st := d.game.still.stats
		var off *lookUse
		for i := range d.game.jn.looks {
			u := &d.game.jn.looks[i]
			if u.inexact && u.box.Width() < 1800 && u.box.Height() < 1100 {
				off = u
				break
			}
		}
		require.NotNil(t, off, "precondition: a look baked running off the frame")

		t.Run("it should record no still building afresh", func(t *testing.T) {
			assert.Zero(t, st.freshBy[kindHouse])
		})
		t.Run("it should hold the record made afresh", func(t *testing.T) {
			assert.Zero(t, st.stale)
		})

		t.Run("and the camera pans by whole pixels until that look is wholly on the frame", func(t *testing.T) {
			key, box := off.key, off.box
			var pan city.Point
			switch {
			case box.Min.X < 0:
				pan.X = math.Ceil(-box.Min.X) + 40
			case box.Max.X > 1920:
				pan.X = -math.Ceil(box.Max.X-1920) - 40
			}
			switch {
			case box.Min.Y < 0:
				pan.Y = math.Ceil(-box.Min.Y) + 40
			case box.Max.Y > 1200:
				pan.Y = -math.Ceil(box.Max.Y-1200) - 40
			}
			d.game.verifyEvery, d.game.verifyBudget = 0, 0
			d.game.scene.Pan(pan)
			require.NoError(t, d.Tick())
			kept := false
			for _, e := range d.game.still.entries {
				for _, u := range d.game.jn.looks[e.jn.looks[0]:e.jn.looks[1]] {
					if u.key == key && e.kept {
						kept = true
					}
				}
			}

			// Drawn afresh, it is baked again exactly where the pages have
			// room, as a look baked off the frame always was.
			t.Run("it should draw it afresh, not keep it", func(t *testing.T) {
				assert.False(t, kept)
			})
		})
	})

	// A flag at the foot of its pole is not rising, so its building is
	// still; a city that draws only on a change can next draw it long
	// after, the flag at the top, with nothing else about it changed.
	t.Run("when a pull request opens, its flag at the foot of its pole, and the next frame comes long after", func(t *testing.T) {
		rc := settled(t)
		rc.snap(func(s *state.Snapshot) {
			s.At = rc.now
			sess := session(s, "s09")
			sess.PRs = append(slices.Clone(sess.PRs), claude.PR{Number: 7, URL: "https://example.com/s09/pr/7", State: claude.PROpen})
		})
		for range 3 {
			rc.frame()
		}
		rc.now = rc.now.Add(10 * time.Minute)
		st := rc.frame()

		t.Run("it should hold the record made afresh", func(t *testing.T) {
			assert.Zero(t, st.stale)
		})
	})

	t.Run("when records are kept unchecked for many frames, their meshes their own", func(t *testing.T) {
		d := fullZoom(t, DriveOptions{})
		d.game.verifyEvery, d.game.verifyBudget = 0, 0
		for range 10 {
			require.NoError(t, d.Tick())
		}
		kept := recordDigest(d.game)
		require.Positive(t, d.game.still.stats.keptBy[kindHouse], "precondition: buildings are kept")
		d.game.noReuse = true
		require.NoError(t, d.Tick())
		afresh := recordDigest(d.game)

		t.Run("it should draw what the frame made afresh draws", func(t *testing.T) {
			assert.Equal(t, afresh, kept)
		})
	})

	t.Run("when a search dims the quiet buildings it does not find", func(t *testing.T) {
		rc := settled(t)
		// s01 stands in a district of aged lots, which the search dims
		// but for its own.
		rc.scene.SetFilter("s01")
		stale := 0
		for range 3 {
			stale += rc.frame().stale
		}

		t.Run("it should hold the record made afresh", func(t *testing.T) {
			assert.Zero(t, stale)
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
			rc.g.verifyEvery = 0
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

// Item 212: what a settled frame of a city like the real history costs
// at full zoom, with its still drawables' records kept and recorded
// afresh, and how many of each kind it keeps and records afresh. It
// reports and never gates.
func BenchmarkAFullZoomFrameKeepingRecords(b *testing.B) {
	for _, c := range []struct {
		name  string
		fresh bool
	}{{"kept", false}, {"afresh", true}} {
		b.Run(c.name, func(b *testing.B) {
			d := fullZoom(b, DriveOptions{})
			d.game.noReuse = c.fresh
			for range 5 {
				require.NoError(b, d.Tick())
			}
			var kept, fresh [kindsKept]int
			frames := 0
			b.ReportAllocs()
			for b.Loop() {
				require.NoError(b, d.Tick())
				st := d.game.still.stats
				for k := range kindsKept {
					kept[k] += st.keptBy[k]
					fresh[k] += st.freshBy[k]
				}
				frames++
			}
			for k := recordKind(1); k < kindsKept; k++ {
				b.ReportMetric(float64(kept[k])/float64(frames), fmt.Sprintf("kept-%d/frame", k))
				b.ReportMetric(float64(fresh[k])/float64(frames), fmt.Sprintf("fresh-%d/frame", k))
			}
		})
	}
}

// Item 212: a release checks kept records on a fixed budget a frame
// (Aria, 2026-10-26: eight), going round them in turn, so a key that
// misses an input shows a stale record for at most a round.
func TestTheNetsBudget(t *testing.T) {
	t.Run("when a city like the real history has settled, checked as a release checks", func(t *testing.T) {
		d := fullZoom(t, DriveOptions{})
		require.Zero(t, d.game.verifyEvery, "precondition: a release checks on its budget")
		require.Equal(t, releaseVerifyBudget, d.game.verifyBudget, "precondition: a release checks on its budget")
		require.NoError(t, d.Tick())
		kept := d.game.still.stats.reused
		// Watched from the start of a round.
		clear(d.game.verifyDone)
		checked := map[uint64]bool{}
		d.game.verified = func(key uint64) { checked[key] = true }
		require.Positive(t, kept, "precondition: records are kept")
		most := 0
		for range kept/releaseVerifyBudget + 2 {
			require.NoError(t, d.Tick())
			most = max(most, d.game.still.stats.checked)
		}
		unchecked := 0
		for _, e := range d.game.still.entries {
			if e.kept && !checked[e.key] {
				unchecked++
			}
		}

		t.Run("it should check no more records a frame than its budget", func(t *testing.T) {
			assert.LessOrEqual(t, most, releaseVerifyBudget)
		})
		t.Run("and a round's frames pass, it should have checked every kept record", func(t *testing.T) {
			assert.Zero(t, unchecked)
		})
	})
}

// Item 212: a district's sign is laid out again only when the camera's
// zoom, heading or projection, or the faces, change, and a pan by whole
// pixels moves the layout it kept.
func TestASignLaidOut(t *testing.T) {
	for _, c := range []struct {
		name string
		pan  city.Point
	}{
		{"by whole pixels", city.Point{X: 17, Y: -9}},
		{"by part of a pixel", city.Point{X: 3.5, Y: 0.25}},
	} {
		t.Run("when the camera pans "+c.name+" after a sign was laid out", func(t *testing.T) {
			rc := newRecordCity(t)
			rc.scene.Camera().Zoom = 2
			rc.frame()
			rc.scene.Pan(c.pan)
			rc.frame()
			cam := rc.scene.Camera()
			apart, signs := 0.0, 0
			for _, d := range rc.scene.City().Districts {
				_, foot, board, laid, ok := rc.g.signOf(cam, d)
				wantFoot, wantBoard, wantLaid, wantOK := rc.g.signLaidOut(cam, d.Sign())
				if !ok || !wantOK {
					continue
				}
				signs++
				for _, p := range [][2]city.Point{{foot, wantFoot}, {board.Min, wantBoard.Min}, {board.Max, wantBoard.Max},
					{laid.Name.At, wantLaid.Name.At}, {laid.Move.At, wantLaid.Move.At}} {
					apart = max(apart, math.Abs(p[0].X-p[1].X), math.Abs(p[0].Y-p[1].Y))
				}
			}
			require.Positive(t, signs, "precondition: signs stand legible")

			t.Run("it should stand where laying it out afresh puts it", func(t *testing.T) {
				assert.LessOrEqual(t, apart, keepTolerance)
			})
		})
	}
}

// Item 212: a kept entry whose journal is quiet is not replayed, so a
// journal that noted anything at all must not read as quiet.
func TestAQuietJournal(t *testing.T) {
	typ := reflect.TypeFor[noteLog]()
	for i := range typ.NumField() {
		f := typ.Field(i)
		t.Run("when a journal noted only its "+f.Name, func(t *testing.T) {
			var j noteLog
			f := reflect.ValueOf(&j).Elem().Field(i)
			v := reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
			switch v.Kind() {
			case reflect.Int:
				v.SetInt(1)
			case reflect.Array:
				v.Index(1).SetInt(1)
			default:
				t.Fatalf("a %s the test cannot set", v.Kind())
			}

			t.Run("it should not be quiet", func(t *testing.T) {
				assert.False(t, j.quiet())
			})
		})
	}
}

// Item 212: keeping a record copies its commands, and a command that
// carried every kind of draw's options was 648 bytes, so keeping a
// building drawn in a hundred pieces cost what recording it did.
func TestARecordedCommand(t *testing.T) {
	t.Run("when a draw is recorded", func(t *testing.T) {
		t.Run("it should take no more than 160 bytes, its kind's options kept beside it", func(t *testing.T) {
			assert.LessOrEqual(t, unsafe.Sizeof(drawCmd{}), uintptr(160))
		})
	})
}

// Item 212: the same city settled at fit, where nearly all of it is on
// screen. It reports and never gates.
func BenchmarkAFitFrameKeepingRecords(b *testing.B) {
	now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.Local)
	d, err := NewDrive(DriveOptions{Home: helpers.ParkedWeek(b, 217, now, "Fit"), Now: now, Width: 1920, Height: 1200})
	require.NoError(b, err)
	require.NoError(b, d.Run(60))
	b.ReportAllocs()
	for b.Loop() {
		require.NoError(b, d.Tick())
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
	// One list, as a frame's: a command's options are kept in the list
	// that recorded it.
	l := newDrawList()
	record := func(img, from *ebiten.Image) drawCmd {
		l.begin()
		l.ImageOn(l.dry, img, from, &ebiten.DrawImageOptions{})
		l.end()
		return l.cmds[len(l.cmds)-1]
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
			assert.True(t, l.sameCmd(&was, &now))
		})
	})

	t.Run("when a sprite is cut from another page", func(t *testing.T) {
		elsewhere := record(other.SubImage(r).(*ebiten.Image), other)

		t.Run("it should hash otherwise", func(t *testing.T) {
			assert.NotEqual(t, was.hash, elsewhere.hash)
		})
	})
}

// recordDigest is what a frame's still drawables draw, command by
// command, positions to a 64th of a pixel.
func recordDigest(g *Game) []string {
	q := func(v float64) int64 { return int64(math.Round(v * 64)) }
	l := g.rec
	var out []string
	for _, e := range g.still.entries {
		if e.moves {
			continue
		}
		for _, c := range l.cmds[e.from:e.to] {
			line := fmt.Sprintf("kind %d box %v page %p", c.kind, c.box, c.page)
			if c.src != nil {
				line += fmt.Sprintf(" src %v", c.src.Bounds())
			}
			switch c.kind {
			case cmdImage:
				o := l.imgOps[c.op]
				line += fmt.Sprintf(" at %d,%d scale %v", q(o.GeoM.Element(0, 2)), q(o.GeoM.Element(1, 2)), o.ColorScale)
			case cmdMesh:
				for _, v := range c.vs {
					line += fmt.Sprintf(" %d,%d", q(float64(v.DstX)), q(float64(v.DstY)))
				}
				line += fmt.Sprint(" ", c.is)
			}
			out = append(out, line)
		}
	}
	return out
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
