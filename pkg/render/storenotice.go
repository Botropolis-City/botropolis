package render

import (
	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// exampleStorePath is the file a scripted notice names, since a
// recording has no daemon to say which.
const exampleStorePath = "~/.local/state/botropolis/stats.db"

// storeNotice says that statistics are on but the daemon refused its
// store, with the file and what to do about it. The file is never
// touched from here: what to do is the person's (item 190).
type storeNotice struct {
	store state.StatsStore
}

type storeWording = statsWording

func (n *storeNotice) wording() storeWording {
	s := n.store
	w := storeWording{Title: "Statistics are not being kept", Keys: "esc  close"}
	switch s.Why {
	case state.WhyNewer:
		w.Lines = []string{
			"Statistics are on, but the daemon would not open its store: " + s.Path + " was written by a newer Botropolis, which this one cannot read.",
			"Update Botropolis to keep statistics again. The file is left as it is.",
		}
	case state.WhyDamaged:
		w.Lines = []string{
			"Statistics are on, but the daemon would not open its store: " + s.Path + " is damaged.",
			"Move it aside and restart the daemon, and Botropolis starts a new store. Nothing in the file was changed.",
		}
	default:
		w.Lines = []string{"Statistics are on, but the daemon could not open its store, " + s.Path + ": " + s.Reason + "."}
	}
	return w
}

func (g *Game) storeNotice() *storeNotice {
	n, _ := g.overlays.top().(*storeNotice)
	return n
}

// maybeStoreNotice shows the notice once a launch, when the first
// snapshot to say anything of the store says it was refused. Sharing,
// a screenshot, a recording or a driven run is not shown it.
func (g *Game) maybeStoreNotice() {
	if g.storeNoticed || g.share != nil || g.screenshot != "" || g.record != "" || g.driven {
		return
	}
	store := g.scene.Snapshot().Store
	if store == nil {
		return
	}
	g.storeNoticed = true
	if store.State == state.StoreRefused {
		g.overlays.push(&storeNotice{store: *store})
	}
}

// playStoreNotice shows the notice a recording's script cues.
func (g *Game) playStoreNotice(why string) {
	g.overlays.push(&storeNotice{store: state.StatsStore{State: state.StoreRefused, Path: exampleStorePath, Why: why}})
}

func (n *storeNotice) keys(g *Game) bool { return !g.just(ebiten.KeyEscape) }

func (n *storeNotice) click(g *Game, at city.Point) bool { return true }

func (n *storeNotice) draw(g *Game, screen *ebiten.Image, width, height float64) {
	g.drawPrompt(screen, width, height, n.wording())
}
