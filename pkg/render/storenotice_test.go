package render

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 190: a statistics store the daemon refused is said on the map,
// with the file and what to do, once a launch.
func TestStoreNotice(t *testing.T) {
	now := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	const path = "/home/someone/.local/state/botropolis/stats.db"
	refused := func(why string) *state.StatsStore {
		return &state.StatsStore{State: state.StoreRefused, Path: path, Why: why, Reason: "it could not be opened: disk I/O error"}
	}

	newGame := func(store *state.StatsStore) *Game {
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(state.Snapshot{At: now, Store: store})
		return &Game{scene: s, actor: &recordingActor{}, scripted: -1, clicked: -1}
	}
	press := func(t *testing.T, g *Game, key ebiten.Key) {
		t.Helper()
		g.scripted = key
		require.NoError(t, g.handleKeys())
		g.scripted = -1
	}

	for _, c := range []struct {
		name  string
		store *state.StatsStore
		want  storeWording
	}{
		{"the newer notice", refused(state.WhyNewer), storeWording{
			Title: "Statistics are not being kept",
			Lines: []string{
				"Statistics are on, but the daemon would not open its store: " + path + " was written by a newer Botropolis, which this one cannot read.",
				"Update Botropolis to keep statistics again. The file is left as it is.",
			},
			Keys: "esc  close",
		}},
		{"the damaged notice", refused(state.WhyDamaged), storeWording{
			Title: "Statistics are not being kept",
			Lines: []string{
				"Statistics are on, but the daemon would not open its store: " + path + " is damaged.",
				"Move it aside and restart the daemon, and Botropolis starts a new store. Nothing in the file was changed.",
			},
			Keys: "esc  close",
		}},
		{"the notice for another refusal", refused(""), storeWording{
			Title: "Statistics are not being kept",
			Lines: []string{
				"Statistics are on, but the daemon could not open its store, " + path + ": it could not be opened: disk I/O error.",
			},
			Keys: "esc  close",
		}},
	} {
		t.Run("when it is "+c.name, func(t *testing.T) {
			t.Run("it should say exactly its wording", func(t *testing.T) {
				assert.Equal(t, c.want, (&storeNotice{store: *c.store}).wording())
			})
		})
	}

	t.Run("when a snapshot says the store was refused", func(t *testing.T) {
		shown := func() *Game {
			g := newGame(refused(state.WhyDamaged))
			g.maybeStoreNotice()
			return g
		}

		t.Run("it should show the notice", func(t *testing.T) {
			assert.NotNil(t, shown().storeNotice())
		})

		t.Run("and esc is pressed", func(t *testing.T) {
			g := shown()
			press(t, g, ebiten.KeyEscape)

			t.Run("it should close", func(t *testing.T) {
				assert.Nil(t, g.storeNotice())
			})

			t.Run("and the next snapshot still says refused", func(t *testing.T) {
				g.scene.SetSnapshot(state.Snapshot{At: now.Add(time.Second), Store: refused(state.WhyDamaged)})
				g.maybeStoreNotice()

				t.Run("it should not show it again until the next launch", func(t *testing.T) {
					assert.Nil(t, g.storeNotice())
				})
			})
		})
	})

	for _, c := range []struct {
		name  string
		store *state.StatsStore
	}{
		{"a snapshot says the store is open", &state.StatsStore{State: state.StoreOpen, Path: path}},
		{"no snapshot has said anything of the store yet", nil},
	} {
		t.Run("when "+c.name, func(t *testing.T) {
			g := newGame(c.store)
			g.maybeStoreNotice()

			t.Run("it should show no notice", func(t *testing.T) {
				assert.Nil(t, g.storeNotice())
			})
		})
	}

	t.Run("when no snapshot has said anything of the store yet, and then one says it was refused", func(t *testing.T) {
		g := newGame(nil)
		g.maybeStoreNotice()
		g.scene.SetSnapshot(state.Snapshot{At: now.Add(time.Second), Store: refused(state.WhyNewer)})
		g.maybeStoreNotice()

		t.Run("it should show the notice", func(t *testing.T) {
			assert.NotNil(t, g.storeNotice())
		})
	})

	for _, run := range []struct {
		name string
		set  func(*Game)
	}{
		{"sharing", func(g *Game) { g.share = &shareMode{} }},
		{"a screenshot", func(g *Game) { g.screenshot = "shot.png" }},
		{"a recording", func(g *Game) { g.record = "frames" }},
		{"a driven run", func(g *Game) { g.driven = true }},
	} {
		t.Run("when the run is "+run.name+", and the store was refused", func(t *testing.T) {
			g := newGame(refused(state.WhyDamaged))
			run.set(g)
			g.maybeStoreNotice()

			t.Run("it should show no notice", func(t *testing.T) {
				assert.Nil(t, g.storeNotice())
			})
		})
	}

	for _, why := range []string{state.WhyNewer, state.WhyDamaged} {
		t.Run("when a recording's script shows the "+why+" notice", func(t *testing.T) {
			g := newGame(nil)
			g.record = "frames"
			g.playStoreNotice(why)

			t.Run("it should show it", func(t *testing.T) {
				n := g.storeNotice()
				require.NotNil(t, n)
				assert.Equal(t, why, n.store.Why)
			})
		})
	}
}
