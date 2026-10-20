package render

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/ui"
)

// bindings is every key the city answers to, in the order the help lists
// them; the footer shows the few that matter most.
var bindings = []ui.Key{
	{Key: "drag", Action: "pan"},
	{Key: "arrows", Action: "pan"},
	{Key: "wheel", Action: "zoom"},
	{Key: "+ / -", Action: "zoom"},
	{Key: "click", Action: "select"},
	{Key: "enter", Action: "attach the selection"},
	{Key: "i", Action: "the selection's details"},
	{Key: "o", Action: "read the selection's conversation"},
	{Key: "tab", Action: "next needs-you"},
	{Key: "w", Action: "triage: walk what waits on you, longest-waiting first"},
	{Key: "shift+w", Action: "the journal: where you were, and what is next"},
	{Key: "l", Action: "the stream: what is happening, newest first"},
	{Key: "v", Action: "the lenses, and the lens's panel in full"},
	{Key: "1-6", Action: "a lens; press again for its next view"},
	{Key: "c", Action: "new session here"},
	{Key: "d d", Action: "demolish the selection"},
	{Key: "f", Action: "Skyline"},
	{Key: "wheel out past Skyline", Action: "the City Plan: every district at a glance; wheel in to return"},
	{Key: planBinding, Action: "the City Plan: every district on one screen; click a badge to fly in"},
	{Key: labelsBinding, Action: "labels off and on"},
	{Key: "0", Action: "reset the view"},
	{Key: "r", Action: "turn the camera"},
	{Key: "n", Action: "night, day, auto"},
	{Key: "[ ]", Action: "scrub the clock an hour"},
	{Key: "s", Action: "settings"},
	{Key: "x", Action: "power breakdown (also click the plant)"},
	{Key: "u", Action: "fetch the plan's usage again; the launch carries it down the river"},
	{Key: "g", Action: "groups board (also click City Hall)"},
	{Key: "shift+n", Action: "New session, in any directory (also on the board)"},
	{Key: "t", Action: "timeline; enter jumps to the session"},
	{Key: "b", Action: "sidebar"},
	{Key: "pgup pgdn home end", Action: "scroll the sidebar"},
	{Key: "h", Action: "hide the UI"},
	{Key: "p", Action: "save a screenshot"},
	{Key: shareBinding, Action: "share: the city with names and figures hidden, as a picture"},
	{Key: "shift+p", Action: "pack the selected session up, or unpack a crate"},
	{Key: "shift+q", Action: "close up: note what is open and what to start with next time; pause only what you choose"},
	{Key: "/ or ctrl+k", Action: "find a session by what was said; the map dims the rest"},
	{Key: "?", Action: "the reference: what everything on the map is"},
	{Key: "hold e", Action: "caption everything on the map"},
	{Key: "q", Action: "quit (asks; y or enter confirms)"},
}

// helpKeys is bindings with every rebindable key as configured.
func (g *Game) helpKeys() []ui.Key {
	chords := g.findChords()
	var keys []ui.Key
	for _, k := range bindings {
		if r, ok := rebindable[k.Key]; ok {
			keys = append(keys, ui.Key{Key: r.prefix + r.chord(chords).String(), Action: k.Action})
			continue
		}
		if k.Key != findBinding {
			keys = append(keys, k)
			continue
		}
		keys = append(keys,
			ui.Key{Key: "/ or " + chords.Open.String(), Action: k.Action},
			ui.Key{Key: chords.Copy.String(), Action: "in the finder: copy how to resume the session"},
			ui.Key{Key: chords.Show.String(), Action: "in the finder: show the session on the map"})
	}
	return keys
}

// rebindable is each help row whose key can be configured, by the key
// bindings lists it under, and the chord helpKeys spells it with
// (items 102 and 172).
var rebindable = map[string]struct {
	prefix string
	chord  func(config.Chords) config.Chord
}{
	planBinding:   {"", func(c config.Chords) config.Chord { return c.Plan }},
	labelsBinding: {"", func(c config.Chords) config.Chord { return c.Labels }},
	"w":           {"", func(c config.Chords) config.Chord { return c.Triage }},
	"shift+w":     {"", func(c config.Chords) config.Chord { return c.Journal }},
	"l":           {"", func(c config.Chords) config.Chord { return c.Stream }},
	"shift+n":     {"", func(c config.Chords) config.Chord { return c.New }},
	"hold e":      {"hold ", func(c config.Chords) config.Chord { return c.Explain }},
}

// findBinding is the help's row for the finder, which helpKeys spells
// with the chords as configured.
const findBinding = "/ or ctrl+k"

// planBinding is the help's row for the City Plan, which helpKeys spells
// with the key as configured.
const planBinding = "m"

// labelsBinding is the help's row for turning the labels off and on,
// which helpKeys spells with the key as configured (item 102).
const labelsBinding = "z"

var footerKeys = []ui.Key{
	{Key: "drag", Action: "pan"},
	{Key: "wheel", Action: "zoom"},
	{Key: "click", Action: "select"},
	{Key: "tab", Action: "next needs-you"},
	{Key: "v", Action: "lenses"},
	{Key: "b", Action: "sidebar"},
	{Key: "f", Action: "Skyline"},
	{Key: "?", Action: "help"},
	{Key: "q", Action: "quit (asks; y or enter confirms)"},
}

const (
	keyPanStep = 12.0
	// scriptPanBeat is how many frames of holding one scripted arrow
	// press is worth. A hand holds the key down; --keys gets a single
	// frame per key, so without this the script could zoom and turn but
	// never walk the view off the plaza.
	scriptPanBeat = 10
	quitPrompt    = "quit? y or enter to quit; any other key stays; or close up first"
)

// panFor is how far the arrow keys move the view this frame.
func panFor(down func(ebiten.Key) bool, step float64) city.Point {
	var pan city.Point
	if down(ebiten.KeyArrowLeft) {
		pan.X += step
	}
	if down(ebiten.KeyArrowRight) {
		pan.X -= step
	}
	if down(ebiten.KeyArrowUp) {
		pan.Y += step
	}
	if down(ebiten.KeyArrowDown) {
		pan.Y -= step
	}
	return pan
}

// handleKeys answers the keyboard for one tick.
func (g *Game) handleKeys() error {
	defer g.showOpenPanels()
	just := g.just
	g.answerRequests()
	g.reserveDive()
	// Sharing takes every key, so nothing behind it opens unseen.
	if g.share != nil {
		return g.shareKeys()
	}
	// Recording the global shortcut takes every key, Ctrl+K among them.
	// Item 84.
	if g.settingsOpen && g.recordShortcutKeys() {
		return nil
	}
	// Ctrl+K reaches the finder from anywhere, and goes no further: j, k
	// and l move inside the settings and the timeline.
	if g.paletteKey() {
		g.openFinder()
		return nil
	}
	if just(ebiten.KeyS) && g.shifted() && g.shareReachable() {
		g.toggleSharing()
		return nil
	}
	if g.overlayKeys() {
		return nil
	}
	if g.settingsOpen {
		g.settingsOpen = g.settingsKeys()
		return nil
	}
	if g.breakdown {
		g.breakdown = g.breakdownKeys()
		return nil
	}
	if g.timeline {
		g.timeline = g.timelineKeys()
		return nil
	}
	if just(ebiten.KeySlash) && !g.shifted() {
		g.openFinder()
		return nil
	}
	if just(ebiten.KeyT) {
		g.timeline, g.away, g.cursor = true, nil, 0
		return nil
	}
	if just(ebiten.KeyX) {
		g.breakdown, g.window = true, city.LastDay
		return nil
	}
	// u asks the CLI for the real usage again. It costs about four
	// seconds, so it runs off the frame and the boats keep their last
	// reading until it lands. Item 49.
	if just(ebiten.KeyU) {
		// Say which of the two happened. A key that silently does
		// nothing reads as a broken key.
		if g.refreshUsage() {
			// The boat goes with the fetch, not with the keypress: a
			// press inside the debounce fetches nothing, so it launches
			// nothing and the status line says why. Item 56.
			g.scene.SendCourier()
			g.SetStatus("reading usage…")
		} else {
			// By hand, the boats are away until called, so a press
			// inside the debounce still calls them up with the figures
			// just fetched. Item 79.
			if g.usageEvery <= 0 {
				g.scene.VisitGauges()
			}
			g.SetStatus("just asked; the figures do not move that fast")
		}
	}
	if just(ebiten.KeyS) {
		g.settingsOpen = true
		return nil
	}
	// Shift+Q closes up: optional, and one step from quitting (item 86).
	if just(ebiten.KeyQ) && g.shifted() {
		g.openCloseUp()
		return nil
	}
	if g.quit.armed(timeNow()) {
		switch {
		case just(ebiten.KeyY), just(ebiten.KeyEnter), just(ebiten.KeyKPEnter):
			if g.quit.confirm(timeNow()) {
				return g.leave()
			}
		case len(inpututil.AppendJustPressedKeys(nil)) > 0:
			g.quit.cancel()
			g.clearPrompt()
			if just(ebiten.KeyEscape) || just(ebiten.KeyQ) {
				return nil
			}
		default:
			g.SetStatus(quitPrompt)
			return nil
		}
	} else {
		g.clearPrompt()
	}
	// The selection sits at the top of the ladder: it is the most modal
	// thing on screen and it carries the verbs. Bug 43a. Bug 43 built
	// this ladder from the keyboard's point of view and missed the card
	// because the card is the one surface the mouse opens — the list of
	// things Escape closes has to be the list of things that are open,
	// whatever opened them.
	// Hidden is the top rung: h hides the card, the sidebar and the view
	// key with everything else, so any other rung would be a change
	// nobody can see. Item 137.
	if just(ebiten.KeyEscape) && g.hidden {
		g.hidden = false
		return nil
	}
	if g.diveKeys() {
		return nil
	}
	if just(ebiten.KeyEscape) && g.scene.Selected() != nil {
		g.scene.Deselect()
		return nil
	}
	if just(ebiten.KeyEscape) && g.scene.FocusedGroup() != "" {
		g.scene.ClearFocus()
		g.scene.Fit()
		return nil
	}
	if just(ebiten.KeyEscape) && g.viewKey {
		g.viewKey = false
		return nil
	}
	if just(ebiten.KeyEscape) && g.scene.CityPlanOpen() {
		g.scene.CloseCityPlan()
		return nil
	}
	if just(ebiten.KeyEscape) && g.scene.View() != city.ViewAttention {
		// Escape is back before it is quit: leave the lens first.
		g.selectLens(city.LensAttention)
		return nil
	}
	if just(ebiten.KeyEscape) && g.sidebar {
		g.sidebar = false
		return nil
	}
	if just(ebiten.KeyEscape) {
		// Escape is back, and the last rung back is settings, not the
		// door. Bug 43: it used to arm the quit prompt here, so the key
		// people press to get out of something asked them to leave.
		// Quit is q, which is what the footer has always said.
		g.settingsOpen = true
		return nil
	}
	if just(ebiten.KeyQ) {
		g.quit.ask(timeNow())
		g.SetStatus(quitPrompt)
		return nil
	}
	if (just(ebiten.KeySlash) && g.shifted()) || just(ebiten.KeyF1) {
		g.openReference()
	}
	// v opens the key to the views; 1 to 9 jump to one; Escape leaves for
	// Attention before it offers to quit, which is handled above.
	//
	// v used to walk the views one at a time, which worked only for
	// someone who already knew the nine existed and what order they came
	// in. The key says what there is, marks where you are, and takes a
	// click — and the numbers still work whether it is open or not, so
	// nothing is slower once you know them.
	if just(ebiten.KeyV) {
		g.viewKey = !g.viewKey
		g.scene.ShowLensPanel()
	}
	for i, key := range []ebiten.Key{
		ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3, ebiten.KeyDigit4, ebiten.KeyDigit5,
		ebiten.KeyDigit6,
	} {
		if just(key) {
			g.selectLens(city.Lenses[i])
			g.viewKey = false
		}
	}
	if just(ebiten.KeyH) {
		g.hidden = !g.hidden
	}
	if just(ebiten.KeyG) {
		g.openBoard()
		return nil
	}
	if just(ebiten.KeyB) {
		g.sidebar = !g.sidebar
		g.mu.Lock()
		g.sidebarFollowed = ""
		g.mu.Unlock()
	}
	if g.pageSidebar() {
		return nil
	}
	// Shift+P packs the selected session up, or unpacks a crate (item
	// 86); P alone saves a frame.
	if just(ebiten.KeyP) && g.shifted() {
		if action := g.scene.TogglePack(); action.Kind != city.ActionNone {
			g.act(action)
		}
	} else if just(ebiten.KeyP) {
		g.snap = screenshotPath(time.Now())
	}
	if just(ebiten.KeyF) || just(ebiten.KeyDigit0) || just(ebiten.KeyKP0) {
		g.scene.Fit()
	}
	if just(ebiten.KeyTab) {
		g.jump(state.NeedsYou)
	}
	if just(ebiten.KeyEnter) {
		g.act(g.scene.Activate())
	}
	if just(ebiten.KeyO) {
		g.openReader()
	}
	if just(ebiten.KeyI) {
		if _, note := g.scene.ToggleDetails(); note != "" {
			g.SetStatus(note)
		}
	}
	if just(ebiten.KeyBracketLeft) || just(ebiten.KeyBracketRight) {
		by := time.Hour
		if just(ebiten.KeyBracketLeft) {
			by = -time.Hour
		}
		shift := g.scene.Scrub(by)
		g.SetStatus(fmt.Sprintf("clock %s (%+.0fh; n to go live)", g.scene.Clock().Format("15:04"), shift.Hours()))
	}
	if just(ebiten.KeyR) {
		g.SetStatus(fmt.Sprintf("heading %d°", g.scene.Turn()))
	}
	if just(ebiten.KeyC) {
		action, note := g.scene.NewHere()
		g.SetStatus(note)
		g.act(action)
	}
	if g.chordPressed(g.findChords().New) {
		g.openNewSession()
		return nil
	}
	if g.chordPressed(g.findChords().Journal) {
		g.openJournal()
		return nil
	}
	if g.chordPressed(g.findChords().Triage) {
		g.startTriage()
		return nil
	}
	if g.chordPressed(g.findChords().Stream) {
		g.toggleStream()
		return nil
	}
	if plan := g.findChords().Plan; g.chordPressed(plan) && g.onlyHeld(plan) {
		g.scene.ToggleCityPlan()
		return nil
	}
	if labels := g.findChords().Labels; g.chordPressed(labels) && g.onlyHeld(labels) {
		g.toggleLabels()
		return nil
	}
	if just(ebiten.KeyN) {
		g.cycleLight()
	}
	if just(ebiten.KeyD) || just(ebiten.KeyDelete) {
		action, note := g.scene.Demolish(time.Now())
		g.SetStatus(note)
		g.act(action)
	}
	g.viewKeys()
	return nil
}

// viewKeys pans with the arrows and zooms with + and -.
func (g *Game) viewKeys() {
	just := g.just
	held, holding := keyByName(g.play.hold)
	pan := panFor(func(k ebiten.Key) bool { return ebiten.IsKeyPressed(k) || (holding && g.play.hold != "" && k == held) }, keyPanStep).
		Add(panFor(func(k ebiten.Key) bool { return g.scripted == k }, keyPanStep*scriptPanBeat))
	if pan != (city.Point{}) {
		g.scene.Pan(pan.Scale(g.theme.Scale))
	}
	centre := g.scene.Size().Scale(0.5)
	if just(ebiten.KeyEqual) || just(ebiten.KeyKPAdd) {
		g.scene.Wheel(centre, 1)
	}
	if just(ebiten.KeyMinus) || just(ebiten.KeyKPSubtract) {
		g.scene.Wheel(centre, -1)
	}
}

// cycleLight steps the light through night, day and live, and says so.
func (g *Game) cycleLight() {
	switch g.scene.CycleLight() {
	case city.LightNight:
		g.SetStatus("light: night (n again for day)")
	case city.LightDay:
		g.SetStatus("light: day (n again to follow the clock)")
	default:
		g.scene.Scrub(0)
		g.SetStatus("light: live")
	}
}

// showOpenPanels shows the interface when a panel is open behind it: a
// panel's key, or a click on the plant, says "show me this", and a panel
// nobody can see would take the keys that follow. Item 142.
func (g *Game) showOpenPanels() {
	if g.hidden && (g.settingsOpen || g.timeline || g.breakdown) {
		g.hidden = false
	}
}

func (g *Game) leave() error {
	if g.saveState != nil {
		g.saveState(g.scene.Layout())
	}
	return ebiten.Termination
}

// act hands an action to the actor and reports a failure in the footer.
func (g *Game) act(action city.Action) {
	if action.Kind == city.ActionMoveToGroup {
		g.openPicker(action.SessionID)
		return
	}
	if action.Kind == city.ActionMuteWord {
		g.openMutePicker(action.SessionID)
		return
	}
	if action.Kind == city.ActionGenerate {
		g.generate(action.SessionID)
		return
	}
	if action.Kind == city.ActionRead {
		g.openReader()
		return
	}
	if action.Kind == city.ActionBreakdown {
		g.breakdown, g.window = true, city.LastDay
		return
	}
	if action.Kind == city.ActionNone || g.actor == nil {
		return
	}
	if err := g.actor.Do(action); err != nil {
		g.SetStatus(err.Error())
	}
}

// screenshotPath is where p saves a frame: the Pictures directory when
// there is one, else home, stamped to the second.
func screenshotPath(now time.Time) string {
	dir, err := os.UserHomeDir()
	if err != nil {
		dir = "."
	}
	if pictures := filepath.Join(dir, "Pictures"); isDir(pictures) {
		dir = pictures
	}
	return filepath.Join(dir, fmt.Sprintf("botropolis-%s.png", now.Format("2006-01-02-150405")))
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// clearPrompt takes the quit prompt off the footer once it is answered
// or has run out; any other notice stays.
func (g *Game) clearPrompt() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.notice.Text(timeNow()) == quitPrompt {
		g.notice.Clear()
	}
}
