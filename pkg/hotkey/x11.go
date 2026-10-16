package hotkey

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"

	"botropolis.city/botropolis/pkg/config"
)

type xprotoAccess = xproto.AccessError

// x11 grabs on the root window through a pure-Go X11 client, and raises
// with _NET_ACTIVE_WINDOW where a window manager answers it.
type x11 struct {
	conn  *xgb.Conn
	root  xproto.Window
	fired chan uint32
	pid   uint32

	mu      sync.Mutex
	code    xproto.Keycode
	mods    uint16
	locks   locks
	grabbed []uint16
}

// xgb logs to stderr on every connection and on every server that goes
// away; what matters reaches the city as errors.
func init() { xgb.Logger = log.New(io.Discard, "", 0) }

func openX11(display string) (Desktop, error) {
	conn, err := xgb.NewConnDisplay(display)
	if err != nil {
		return nil, fmt.Errorf("%w (%v); %s", ErrUnsupported, err, Fallback)
	}
	d := &x11{
		conn:  conn,
		root:  xproto.Setup(conn).DefaultScreen(conn).Root,
		fired: make(chan uint32, 1),
		pid:   uint32(os.Getpid()),
	}
	go d.listen()
	return d, nil
}

func (d *x11) Fired() <-chan uint32 { return d.fired }

func (d *x11) Close() { d.conn.Close() }

func (d *x11) Grab(c config.Chord) error {
	keyboard, err := d.keyboard()
	if err != nil {
		return err
	}
	code, err := keyboard.keycode(c.Key)
	if err != nil {
		return fmt.Errorf("%s: %w", c, err)
	}
	found, err := d.lockMasks(keyboard)
	if err != nil {
		return err
	}
	if err := d.Release(); err != nil {
		return err
	}
	masks := grabMasks(c, found)
	for i, m := range masks {
		err := xproto.GrabKeyChecked(d.conn, true, d.root, m, code, xproto.GrabModeAsync, xproto.GrabModeAsync).Check()
		if err != nil {
			for _, done := range masks[:i] {
				_ = xproto.UngrabKeyChecked(d.conn, code, d.root, done).Check()
			}
			return grabError(c, err)
		}
	}
	d.mu.Lock()
	d.code, d.mods, d.locks, d.grabbed = code, modMask(c), found, masks
	d.mu.Unlock()
	return nil
}

func (d *x11) Release() error {
	d.mu.Lock()
	code, grabbed := d.code, d.grabbed
	d.grabbed = nil
	d.mu.Unlock()
	for _, m := range grabbed {
		if err := xproto.UngrabKeyChecked(d.conn, code, d.root, m).Check(); err != nil {
			return err
		}
	}
	return nil
}

func (d *x11) listen() {
	for {
		ev, err := d.conn.WaitForEvent()
		if ev == nil && err == nil {
			return
		}
		press, ok := ev.(xproto.KeyPressEvent)
		if !ok {
			continue
		}
		d.mu.Lock()
		hit := len(d.grabbed) > 0 && press.Detail == d.code && press.State&^d.locks.all() == d.mods
		d.mu.Unlock()
		if hit {
			select {
			case d.fired <- uint32(press.Time):
			default:
			}
		}
	}
}

// Raise asks the window manager to activate this process's window, as a
// direct user action, with the press's time; with no window manager
// answering, it raises and focuses the window itself.
func (d *x11) Raise(time uint32) bool {
	win, ok := d.window(d.root, 3)
	if !ok {
		return false
	}
	if d.managed() {
		active, err := d.atom("_NET_ACTIVE_WINDOW")
		if err != nil {
			return false
		}
		ev := xproto.ClientMessageEvent{
			Format: 32,
			Window: win,
			Type:   active,
			Data:   xproto.ClientMessageDataUnionData32New([]uint32{2, time, 0, 0, 0}),
		}
		mask := uint32(xproto.EventMaskSubstructureRedirect | xproto.EventMaskSubstructureNotify)
		return xproto.SendEventChecked(d.conn, false, d.root, mask, string(ev.Bytes())).Check() == nil
	}
	if err := xproto.ConfigureWindowChecked(d.conn, win, xproto.ConfigWindowStackMode, []uint32{xproto.StackModeAbove}).Check(); err != nil {
		return false
	}
	return xproto.SetInputFocusChecked(d.conn, xproto.InputFocusParent, win, xproto.Timestamp(time)).Check() == nil
}

// managed is whether an EWMH window manager is running, which is the
// one to ask for activation.
func (d *x11) managed() bool {
	check, err := d.atom("_NET_SUPPORTING_WM_CHECK")
	if err != nil {
		return false
	}
	reply, err := xproto.GetProperty(d.conn, false, d.root, check, xproto.AtomWindow, 0, 1).Reply()
	return err == nil && reply.ValueLen > 0
}

// window finds the mapped window this process owns, by _NET_WM_PID,
// below a window manager's frames, which i3 unmaps on a workspace that
// is not showing.
func (d *x11) window(under xproto.Window, depth int) (xproto.Window, bool) {
	if depth == 0 {
		return 0, false
	}
	tree, err := xproto.QueryTree(d.conn, under).Reply()
	if err != nil {
		return 0, false
	}
	pidAtom, err := d.atom("_NET_WM_PID")
	if err != nil {
		return 0, false
	}
	for i := len(tree.Children) - 1; i >= 0; i-- {
		w := tree.Children[i]
		attrs, err := xproto.GetWindowAttributes(d.conn, w).Reply()
		if err != nil {
			continue
		}
		if attrs.MapState != xproto.MapStateUnmapped {
			reply, err := xproto.GetProperty(d.conn, false, w, pidAtom, xproto.AtomCardinal, 0, 1).Reply()
			if err == nil && reply.ValueLen == 1 && xgb.Get32(reply.Value) == d.pid {
				return w, true
			}
		}
		if found, ok := d.window(w, depth-1); ok {
			return found, true
		}
	}
	return 0, false
}

func (d *x11) atom(name string) (xproto.Atom, error) {
	reply, err := xproto.InternAtom(d.conn, false, uint16(len(name)), name).Reply()
	if err != nil {
		return 0, err
	}
	return reply.Atom, nil
}

func (d *x11) keyboard() (keyboardMap, error) {
	setup := xproto.Setup(d.conn)
	count := byte(setup.MaxKeycode - setup.MinKeycode + 1)
	reply, err := xproto.GetKeyboardMapping(d.conn, setup.MinKeycode, count).Reply()
	if err != nil {
		return keyboardMap{}, err
	}
	return keyboardMap{first: setup.MinKeycode, perCode: int(reply.KeysymsPerKeycode), syms: reply.Keysyms}, nil
}

// lockMasks finds which modifiers Num Lock and Scroll Lock are on.
func (d *x11) lockMasks(keyboard keyboardMap) (locks, error) {
	reply, err := xproto.GetModifierMapping(d.conn).Reply()
	if err != nil {
		return locks{}, err
	}
	found := locks{caps: xproto.ModMaskLock}
	per := int(reply.KeycodesPerModifier)
	for mod := 0; mod < 8 && per > 0; mod++ {
		for _, code := range reply.Keycodes[mod*per : (mod+1)*per] {
			switch keyboard.sym(code) {
			case keysymNumLock:
				found.num = 1 << mod
			case keysymScrollLock:
				found.scroll = 1 << mod
			}
		}
	}
	return found, nil
}

const (
	keysymNumLock    xproto.Keysym = 0xff7f
	keysymScrollLock xproto.Keysym = 0xff14
)

// locks are the modifiers that lock keys set, which X11 grabs match
// exactly, so a grab is made for every combination of them.
type locks struct{ caps, num, scroll uint16 }

func (l locks) all() uint16 { return l.caps | l.num | l.scroll }

func modMask(c config.Chord) uint16 {
	var m uint16
	if c.Ctrl {
		m |= xproto.ModMaskControl
	}
	if c.Shift {
		m |= xproto.ModMaskShift
	}
	if c.Alt {
		m |= xproto.ModMask1
	}
	if c.Cmd {
		m |= xproto.ModMask4
	}
	return m
}

func grabMasks(c config.Chord, l locks) []uint16 {
	base := modMask(c)
	seen := map[uint16]bool{}
	var masks []uint16
	for i := 0; i < 8; i++ {
		m := base
		if i&1 != 0 {
			m |= l.caps
		}
		if i&2 != 0 {
			m |= l.num
		}
		if i&4 != 0 {
			m |= l.scroll
		}
		if !seen[m] {
			seen[m] = true
			masks = append(masks, m)
		}
	}
	return masks
}

// keyboardMap is the server's keycode-to-keysym table.
type keyboardMap struct {
	first   xproto.Keycode
	perCode int
	syms    []xproto.Keysym
}

func (k keyboardMap) sym(code xproto.Keycode) xproto.Keysym {
	at := int(code-k.first) * k.perCode
	if code < k.first || k.perCode == 0 || at >= len(k.syms) {
		return 0
	}
	return k.syms[at]
}

// keycode finds the key a chord names on this keyboard.
func (k keyboardMap) keycode(name string) (xproto.Keycode, error) {
	want, ok := keysym(name)
	if !ok {
		return 0, fmt.Errorf("%s is not a key a global shortcut can use", name)
	}
	for i, s := range k.syms {
		if s == want && k.perCode > 0 {
			return k.first + xproto.Keycode(i/k.perCode), nil
		}
	}
	return 0, fmt.Errorf("this keyboard has no %s key", name)
}

var namedKeysyms = map[string]xproto.Keysym{
	"space": 0x20, "quote": 0x27, "comma": 0x2c, "minus": 0x2d, "period": 0x2e, "slash": 0x2f,
	"semicolon": 0x3b, "equal": 0x3d, "bracketleft": 0x5b, "backslash": 0x5c, "bracketright": 0x5d, "backquote": 0x60,
	"backspace": 0xff08, "tab": 0xff09, "enter": 0xff0d, "pause": 0xff13, "escape": 0xff1b,
	"home": 0xff50, "arrowleft": 0xff51, "arrowup": 0xff52, "arrowright": 0xff53, "arrowdown": 0xff54,
	"pageup": 0xff55, "pagedown": 0xff56, "end": 0xff57, "printscreen": 0xff61, "insert": 0xff63, "delete": 0xffff,
	"numpadenter": 0xff8d, "numpadmultiply": 0xffaa, "numpadadd": 0xffab, "numpadsubtract": 0xffad,
	"numpaddecimal": 0xffae, "numpaddivide": 0xffaf,
}

// keysym is the X11 keysym for a key as a chord names it.
func keysym(name string) (xproto.Keysym, bool) {
	name = strings.ToLower(name)
	if s, ok := namedKeysyms[name]; ok {
		return s, true
	}
	if len(name) == 1 && name[0] >= 'a' && name[0] <= 'z' {
		return xproto.Keysym(name[0]), true
	}
	for prefix, base := range map[string]xproto.Keysym{"digit": 0x30, "numpad": 0xffb0} {
		if rest, ok := strings.CutPrefix(name, prefix); ok {
			if n, err := strconv.Atoi(rest); err == nil && n >= 0 && n <= 9 {
				return base + xproto.Keysym(n), true
			}
		}
	}
	if rest, ok := strings.CutPrefix(name, "f"); ok {
		if n, err := strconv.Atoi(rest); err == nil && n >= 1 && n <= 24 {
			return 0xffbe + xproto.Keysym(n-1), true
		}
	}
	return 0, false
}
