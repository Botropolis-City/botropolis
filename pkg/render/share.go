package render

import (
	"bufio"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

// Sharing mode (item 168) shows the city as it would be shared: the city
// wears neutral names (the veil), no panel or figure is drawn, and a
// guard at the two places text reaches the screen refuses anything that
// still names the person. It takes pictures of what it shows.

// shareBinding is the key list's row for sharing mode, which a click on
// the reference's key list enters too.
const shareBinding = "shift+s"

// shareMode is sharing mode while it is on.
type shareMode struct {
	guard *shareGuard
	// snap is a picture asked for, taken on the next draw; saved is the
	// folder the last capture went to, which o opens.
	snap  string
	saved string
	// own is set while the sharing bar draws its own words, which the
	// guard has no business refusing.
	own bool
	// rec is the recording under way, nil when none is; last is the one
	// that finished most recently, and encoding waits for any still
	// being written.
	rec      *shareRec
	last     *shareRec
	encoding sync.WaitGroup
	// clipStep is how many times l has stepped the clip's length.
	clipStep int
}

// shareRec is one recording of the shared city, on the city's own clock:
// while it runs the city updates once per drawn frame (item 113's
// lockstep), and every every'th draw is a frame of it.
type shareRec struct {
	kind  string
	path  string
	sink  frameSink
	every int
	// frames is how many it holds, written how many it has, and draws
	// the draws since it began; from and to are the city's ticks at its
	// first and last frame.
	frames, written, draws int
	from, to               uint64
}

// seconds is how much of the city's clock the recording spans.
func (r *shareRec) seconds() float64 {
	if r == nil || r.written == 0 {
		return 0
	}
	return float64(r.to-r.from+uint64(r.every)) / liveTPS
}

// recording is whether sharing mode is recording.
func (s *shareMode) recording() bool { return s != nil && s.rec != nil }

// startGIF begins a GIF of the shared city.
func (g *Game) startGIF() {
	path := filepath.Join(g.captureDir(picturesDir), captureName(timeNow(), "gif"))
	g.share.rec = &shareRec{
		kind: "GIF", path: path, sink: newGIFWriter(path, gifFPS, gifMaxWidth),
		every: liveTPS / gifFPS, frames: gifSeconds * gifFPS,
	}
}

// startClip begins a clip of the shared city, or says what is missing.
func (g *Game) startClip() {
	bin, err := g.ffmpegPath()
	if err != nil {
		g.SetStatus("a clip needs ffmpeg, which is not installed; install it and press c again")
		return
	}
	format, ok := pickClipFormat(clipEncoders(bin))
	if !ok {
		g.SetStatus("a clip needs ffmpeg with libx264 or libvpx-vp9, and this ffmpeg has neither")
		return
	}
	length := g.share.clipLength()
	path := filepath.Join(g.captureDir(videosDir), captureName(timeNow(), format.ext))
	g.share.rec = &shareRec{
		kind: "clip", path: path, sink: newClipWriter(bin, path, clipFPS, format),
		every: liveTPS / clipFPS, frames: length * clipFPS,
	}
}

// drop ends a recording before its last frame: it writes nothing, and
// encoding waits for its writer to have cleaned up.
func (s *shareMode) drop() {
	if s.rec == nil {
		return
	}
	done := s.rec.sink.abandon()
	s.rec = nil
	s.encoding.Add(1)
	go func() {
		defer s.encoding.Done()
		// An abandoned recording can only fail to clean up after itself,
		// and sharing is over, so there is no bar left to say so on.
		if err := <-done; err != nil {
			return
		}
	}()
}

// recordFrame gives the recording this draw, and ends it with its last
// frame.
func (g *Game) recordFrame(screen *ebiten.Image) {
	rec := g.share.rec
	if rec.draws%rec.every == 0 {
		if rec.written == 0 {
			rec.from = g.ticks
		}
		rec.to = g.ticks
		rec.sink.add(g.grabFrame(screen))
		rec.written++
	}
	rec.draws++
	if rec.written < rec.frames {
		return
	}
	share := g.share
	share.rec, share.last = nil, rec
	share.saved = filepath.Dir(rec.path)
	g.SetStatus("writing the " + rec.kind + "…")
	done := rec.sink.finish()
	share.encoding.Add(1)
	go func() {
		defer share.encoding.Done()
		if err := <-done; err != nil {
			g.SetStatus(err.Error())
			return
		}
		g.SetStatus("saved " + rec.path + "; o opens the folder")
	}()
}

// sharing is whether the city is being shared.
func (g *Game) sharing() bool { return g.share != nil }

// toggleSharing enters sharing mode, or leaves it.
func (g *Game) toggleSharing() {
	if g.share != nil {
		g.leaveSharing()
		return
	}
	guard := newShareGuard(city.Words(g.scene.Snapshot()))
	g.share = &shareMode{guard: guard}
	g.scene.SetVeil(city.NewVeil())
}

// leaveSharing takes the veil off; everything open underneath was left
// open, so there is nothing else to restore.
func (g *Game) leaveSharing() {
	g.share.drop()
	g.share = nil
	g.scene.SetVeil(nil)
}

// finishSharing is the window closing: a recording not yet finished is
// dropped, and one being written is waited for, so it is not lost.
func (g *Game) finishSharing() {
	if g.share == nil {
		return
	}
	g.share.drop()
	g.share.encoding.Wait()
}

// shareSnapshot is told of every snapshot while sharing, so a session
// that starts mid-share has its names refused too.
func (g *Game) shareSnapshot() {
	if g.share != nil {
		g.share.guard.add(city.Words(g.scene.RealSnapshot()))
	}
}

// shareKeys answers the keyboard while sharing: the captures, leaving,
// and the keys that frame the shot. Nothing else reaches the map.
func (g *Game) shareKeys() error {
	just := g.just
	switch {
	case just(ebiten.KeyEscape), just(ebiten.KeyS) && g.shifted():
		g.leaveSharing()
		return nil
	case just(ebiten.KeyP) && !g.shifted():
		g.share.snap = filepath.Join(g.captureDir(picturesDir), captureName(timeNow(), "png"))
	case just(ebiten.KeyG) && g.share.rec == nil:
		g.startGIF()
	case just(ebiten.KeyC) && g.share.rec == nil:
		g.startClip()
	case just(ebiten.KeyL) && g.share.rec == nil:
		g.share.clipStep++
		g.SetStatus(fmt.Sprintf("clips are %d s", g.share.clipLength()))
	case just(ebiten.KeyO):
		dir := g.share.saved
		if dir == "" {
			dir = g.captureDir(picturesDir)
		}
		if err := g.openDir(dir); err != nil {
			g.SetStatus(err.Error())
		}
	case just(ebiten.KeyR) && !g.shifted():
		g.SetStatus(fmt.Sprintf("heading %d°", g.scene.Turn()))
	case just(ebiten.KeyN) && !g.shifted():
		g.cycleLight()
	case just(ebiten.KeyF), just(ebiten.KeyDigit0), just(ebiten.KeyKP0):
		g.scene.Fit()
	}
	g.viewKeys()
	return nil
}

// captureName is a shared capture's file name, stamped to the second,
// and named apart from p's screenshots, which hide nothing.
func captureName(now time.Time, ext string) string {
	return fmt.Sprintf("botropolis-share-%s.%s", now.Format("2006-01-02-150405"), ext)
}

// captureDir is where a capture of a kind is saved.
func (g *Game) captureDir(kind userDirKind) string {
	if g.shareDir != nil {
		return g.shareDir(kind)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	return userDir(home, runtime.GOOS, config, kind)
}

// openDir shows a folder in the desktop's file manager.
func (g *Game) openDir(dir string) error {
	if g.openFolder != nil {
		return g.openFolder(dir)
	}
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	cmd := exec.Command(opener, dir)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not open %s: %w", dir, err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// grabFrame reads the frame being drawn.
func (g *Game) grabFrame(screen *ebiten.Image) image.Image {
	if g.grab != nil {
		return g.grab(screen)
	}
	return frame(screen)
}

// shareCapture takes what sharing mode asked for from the frame just
// drawn, before the sharing bar goes over it.
func (g *Game) shareCapture(screen *ebiten.Image) {
	if g.share == nil {
		return
	}
	if g.share.rec != nil {
		g.recordFrame(screen)
	}
	if g.share.snap == "" {
		return
	}
	path := g.share.snap
	g.share.snap = ""
	if err := writePNG(path, g.grabFrame(screen)); err != nil {
		g.SetStatus(err.Error())
		return
	}
	g.share.saved = filepath.Dir(path)
	g.SetStatus("saved " + path + "; o opens the folder")
}

// shareBar is what the sharing bar says.
func (s *shareMode) bar() string {
	return fmt.Sprintf("Sharing: names and figures hidden    p picture    g GIF    c clip, %d s    l length    o open folder    esc leave", s.clipLength())
}

// drawShareBar draws the sharing bar and the last notice over the frame,
// after any capture has read it.
func (g *Game) drawShareBar(screen *ebiten.Image) {
	if g.share == nil {
		return
	}
	th := g.theme
	bounds := screen.Bounds()
	width, height := float64(bounds.Dx()), float64(bounds.Dy())
	lines := []string{g.share.bar()}
	if note := g.notice.Text(timeNow()); note != "" {
		lines = append([]string{note}, lines...)
	}
	if rec := g.share.rec; rec != nil {
		lines = append([]string{fmt.Sprintf("● recording %s %.1f of %.0f s", rec.kind,
			float64(rec.draws)/liveTPS, float64(rec.frames*rec.every)/liveTPS)}, lines...)
	}
	pad := th.Px(8)
	var w, h float64
	for _, l := range lines {
		lw, lh := g.faces.Measure(l, ui.Body)
		w, h = max(w, lw), h+lh
	}
	box := city.Rect{
		Min: city.Point{X: (width-w)/2 - pad, Y: height - h - 3*pad},
		Max: city.Point{X: (width+w)/2 + pad, Y: height - pad},
	}
	g.roundPanel(screen, box)
	g.share.own = true
	defer func() { g.share.own = false }()
	y := box.Min.Y + pad
	for _, l := range lines {
		lw, lh := g.faces.Measure(l, ui.Body)
		g.text(screen, city.Point{X: (width - lw) / 2, Y: y}, l, ui.Body, th.Palette.Text)
		y += lh
	}
}

// shareAllows is whether a string may be drawn: anything while not
// sharing, and while sharing only what the guard allows.
func (g *Game) shareAllows(s string) bool {
	return g.share == nil || g.share.own || g.share.guard.allows(s)
}

// shareFigure is a spend, a token count or a usage percentage.
var shareFigure = regexp.MustCompile(`\$\s?\d|\d\s?(?i:usd)\b|\d(\.\d+)?\s?[kKMB]\b|\d\s?%|(?i:\btok(en)?s?\b)`)

// shareAlias is a name the veil gave.
var shareAlias = regexp.MustCompile(`^(District [A-Z]+|(Session|Tower|Skill|Group|Team|Agent|Repository) \d+)$`)

// aliasWords are the veil's own words, which a name may contain without
// the aliases built from them being refused.
var aliasWords = map[string]bool{
	"district": true, "session": true, "tower": true, "skill": true, "group": true,
	"team": true, "agent": true, "repository": true, "sharing": true,
}

// shareGuard refuses to draw a string that names the person. It is the
// last of three layers: the veil and the hidden panels should leave it
// nothing to refuse, and refused counts what they missed.
type shareGuard struct {
	names    []string
	texts    []string
	refused  int
	refusals []string
}

func newShareGuard(id city.Identifying) *shareGuard {
	g := &shareGuard{}
	g.add(id)
	return g
}

// minFragment is the shortest piece of a name the guard looks for, so a
// name's every word is caught and two-letter words are not.
const minFragment = 4

// add learns the names and texts in one more snapshot.
func (s *shareGuard) add(id city.Identifying) {
	seen := map[string]bool{}
	for _, n := range s.names {
		seen[n] = true
	}
	learn := func(w string) {
		w = strings.ToLower(w)
		if len(w) < minFragment || seen[w] || aliasWords[w] {
			return
		}
		seen[w] = true
		s.names = append(s.names, w)
	}
	for _, name := range id.Names {
		learn(name)
		for _, word := range splitWords(name) {
			learn(word)
		}
	}
	for _, text := range id.Texts {
		text = strings.ToLower(strings.TrimSpace(text))
		if len(text) >= minFragment && !seen["\x00"+text] {
			seen["\x00"+text] = true
			s.texts = append(s.texts, text)
		}
	}
}

func splitWords(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// allows is whether text may be drawn while the city is shared.
func (s *shareGuard) allows(text string) bool {
	if shareAlias.MatchString(text) {
		return true
	}
	if shareFigure.MatchString(text) {
		return s.refuse(text)
	}
	low := strings.ToLower(text)
	for _, n := range s.names {
		if strings.Contains(low, n) {
			return s.refuse(text)
		}
	}
	for _, t := range s.texts {
		if strings.Contains(low, t) {
			return s.refuse(text)
		}
	}
	// A label cut short with an ellipsis: its last word may be the start
	// of a name, and the whole of it the start of a text.
	if cut, ok := strings.CutSuffix(strings.TrimSpace(low), "…"); ok {
		words := splitWords(cut)
		if len(words) > 0 {
			last := words[len(words)-1]
			for _, n := range s.names {
				if len(last) >= minFragment && strings.HasPrefix(n, last) {
					return s.refuse(text)
				}
			}
		}
		cut = strings.TrimSpace(cut)
		for _, t := range s.texts {
			if len(cut) >= minFragment && strings.Contains(t, cut) {
				return s.refuse(text)
			}
		}
	}
	return true
}

func (s *shareGuard) refuse(text string) bool {
	s.refused++
	s.refusals = append(s.refusals, text)
	return false
}

// userDirKind is one of the desktop's own folders, as user-dirs.dirs
// names it.
type userDirKind string

const (
	picturesDir userDirKind = "PICTURES"
	videosDir   userDirKind = "VIDEOS"
)

// userDir is where captures of a kind go: the folder the desktop names
// for it in user-dirs.dirs on Linux, Pictures or Movies on macOS, and
// home when there is no such folder.
func userDir(home, goos, config string, kind userDirKind) string {
	fallback := map[userDirKind]string{picturesDir: "Pictures", videosDir: "Videos"}[kind]
	if goos == "darwin" {
		fallback = map[userDirKind]string{picturesDir: "Pictures", videosDir: "Movies"}[kind]
	} else if named, ok := xdgUserDir(home, config, kind); ok {
		if isDir(named) {
			return named
		}
		return home
	}
	if dir := filepath.Join(home, fallback); isDir(dir) {
		return dir
	}
	return home
}

// xdgUserDir reads a folder from user-dirs.dirs, the file xdg-user-dir
// reads.
func xdgUserDir(home, config string, kind userDirKind) (string, bool) {
	f, err := os.Open(filepath.Join(config, "user-dirs.dirs"))
	if err != nil {
		return "", false
	}
	defer func() { _ = f.Close() }()
	key := "XDG_" + string(kind) + "_DIR="
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		value, ok := strings.CutPrefix(line, key)
		if !ok {
			continue
		}
		value = strings.Trim(value, `"`)
		if rest, ok := strings.CutPrefix(value, "$HOME"); ok {
			value = home + rest
		}
		if !filepath.IsAbs(value) {
			return "", false
		}
		return filepath.Clean(value), true
	}
	return "", false
}

// shareReachable is whether shift+s can enter sharing now: from the map,
// or from the reference, but not from a panel that takes typing.
func (g *Game) shareReachable() bool {
	top := g.overlays.top()
	_, reference := top.(*reference)
	return top == nil || reference
}

// sharePointer is the pointer while sharing: the wheel zooms and a drag
// pans, to frame the shot, and a click does nothing, since everything it
// could press is hidden.
func (g *Game) sharePointer(cursor city.Point, wheel float64, pressed, down, released bool) {
	if wheel != 0 {
		g.scene.Wheel(cursor, wheel)
	}
	if pressed {
		g.dragging, g.dragFrom = true, cursor
	}
	if g.dragging && down {
		g.scene.Pan(cursor.Sub(g.dragFrom))
		g.dragFrom = cursor
	}
	if released {
		g.dragging = false
	}
}
