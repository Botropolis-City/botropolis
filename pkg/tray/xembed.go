package tray

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"log"
	"sync"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// xgb logs every connection and every server that goes away to stderr;
// what matters reaches the city as errors.
func init() { xgb.Logger = log.New(io.Discard, "", 0) }

const (
	systemTrayRequestDock = 0
	xembedMapped          = 1
	defaultIconSide       = 22
	menuPadX, menuPadY    = 12, 6
)

// The menu's colours are the logo's: its plate, its light face, and a
// row a little lighter than the plate under the pointer.
var (
	menuPlate = rgb(plateColor)
	menuText  = rgb(0xe6e9ef)
	menuHover = rgb(0x2a2e38)
)

// xembedAtoms are the names the protocol speaks in.
type xembedAtoms struct {
	sel, opcode, manager, info, visual xproto.Atom
}

func internAtoms(conn *xgb.Conn, screen int) (xembedAtoms, error) {
	var a xembedAtoms
	for _, n := range []struct {
		name string
		into *xproto.Atom
	}{
		{fmt.Sprintf("_NET_SYSTEM_TRAY_S%d", screen), &a.sel},
		{"_NET_SYSTEM_TRAY_OPCODE", &a.opcode},
		{"MANAGER", &a.manager},
		{"_XEMBED_INFO", &a.info},
		{"_NET_SYSTEM_TRAY_VISUAL", &a.visual},
	} {
		reply, err := xproto.InternAtom(conn, false, uint16(len(n.name)), n.name).Reply()
		if err != nil {
			return a, err
		}
		*n.into = reply.Atom
	}
	return a, nil
}

// probeXEmbed says whether a manager owns the default screen's system
// tray selection.
func probeXEmbed(conn *xgb.Conn) bool {
	if conn == nil {
		return false
	}
	a, err := internAtoms(conn, conn.DefaultScreen)
	if err != nil {
		return false
	}
	owner, err := xproto.GetSelectionOwner(conn, a.sel).Reply()
	return err == nil && owner.Owner != 0
}

// xembed is the city's icon docked in an XEmbed system tray, and the
// small menu a right click opens beside it, since the protocol has none.
type xembed struct {
	conn    *xgb.Conn
	screen  *xproto.ScreenInfo
	atoms   xembedAtoms
	win     xproto.Window
	gc      xproto.Gcontext
	depth   byte
	argb    bool
	w, h    uint16
	actions chan Action
	menu    *trayMenu
	once    sync.Once
}

func (x *xembed) Actions() <-chan Action { return x.actions }

func (x *xembed) Close() { x.once.Do(x.conn.Close) }

func (x *xembed) send(a Action) {
	select {
	case x.actions <- a:
	default:
	}
}

func startXEmbed(conn *xgb.Conn) (Tray, error) {
	x := &xembed{conn: conn, screen: xproto.Setup(conn).DefaultScreen(conn), actions: make(chan Action, 4), w: defaultIconSide, h: defaultIconSide}
	var err error
	if x.atoms, err = internAtoms(conn, conn.DefaultScreen); err != nil {
		return nil, err
	}
	if err := x.create(); err != nil {
		return nil, err
	}
	if err := xproto.ChangeWindowAttributesChecked(conn, x.screen.Root, xproto.CwEventMask,
		[]uint32{xproto.EventMaskStructureNotify}).Check(); err != nil {
		return nil, err
	}
	if err := x.dock(); err != nil {
		return nil, err
	}
	go x.listen()
	return x, nil
}

// trayVisual is the 32-bit visual the manager offers for icons with
// transparent corners, or none.
func (x *xembed) trayVisual() (xproto.Visualid, bool) {
	owner, err := xproto.GetSelectionOwner(x.conn, x.atoms.sel).Reply()
	if err != nil || owner.Owner == 0 {
		return 0, false
	}
	prop, err := xproto.GetProperty(x.conn, false, owner.Owner, x.atoms.visual, xproto.AtomVisualid, 0, 1).Reply()
	if err != nil || prop.Format != 32 || len(prop.Value) < 4 {
		return 0, false
	}
	id := xproto.Visualid(binary.LittleEndian.Uint32(prop.Value))
	for _, d := range x.screen.AllowedDepths {
		for _, v := range d.Visuals {
			if v.VisualId == id && d.Depth == 32 {
				return id, true
			}
		}
	}
	return 0, false
}

func (x *xembed) create() error {
	var err error
	if x.win, err = xproto.NewWindowId(x.conn); err != nil {
		return err
	}
	events := uint32(xproto.EventMaskExposure | xproto.EventMaskStructureNotify | xproto.EventMaskButtonPress)
	visual, depth := x.screen.RootVisual, x.screen.RootDepth
	mask, values := uint32(xproto.CwBackPixmap|xproto.CwEventMask), []uint32{xproto.BackPixmapParentRelative, events}
	if v, ok := x.trayVisual(); ok {
		cmap, err := xproto.NewColormapId(x.conn)
		if err != nil {
			return err
		}
		if err := xproto.CreateColormapChecked(x.conn, xproto.ColormapAllocNone, cmap, x.screen.Root, v).Check(); err != nil {
			return err
		}
		visual, depth, x.argb = v, 32, true
		mask, values = xproto.CwBackPixel|xproto.CwBorderPixel|xproto.CwEventMask|xproto.CwColormap, []uint32{0, 0, events, uint32(cmap)}
	}
	x.depth = depth
	if err := xproto.CreateWindowChecked(x.conn, depth, x.win, x.screen.Root, 0, 0, x.w, x.h, 0,
		xproto.WindowClassInputOutput, visual, mask, values).Check(); err != nil {
		return err
	}
	info := make([]byte, 8)
	binary.LittleEndian.PutUint32(info[4:], xembedMapped)
	if err := xproto.ChangePropertyChecked(x.conn, xproto.PropModeReplace, x.win, x.atoms.info, x.atoms.info, 32, 2, info).Check(); err != nil {
		return err
	}
	name := "Botropolis"
	_ = xproto.ChangePropertyChecked(x.conn, xproto.PropModeReplace, x.win, xproto.AtomWmName, xproto.AtomString, 8, uint32(len(name)), []byte(name)).Check()
	if x.gc, err = xproto.NewGcontextId(x.conn); err != nil {
		return err
	}
	return xproto.CreateGCChecked(x.conn, x.gc, xproto.Drawable(x.win), 0, nil).Check()
}

// dock asks whichever manager owns the selection now to take the icon.
func (x *xembed) dock() error {
	owner, err := xproto.GetSelectionOwner(x.conn, x.atoms.sel).Reply()
	if err != nil {
		return err
	}
	if owner.Owner == 0 {
		return fmt.Errorf("%w: nothing owns the display's XEmbed system tray", ErrNoTray)
	}
	ev := xproto.ClientMessageEvent{Format: 32, Window: owner.Owner, Type: x.atoms.opcode,
		Data: xproto.ClientMessageDataUnionData32New([]uint32{uint32(xproto.TimeCurrentTime), systemTrayRequestDock, uint32(x.win), 0, 0})}
	return xproto.SendEventChecked(x.conn, false, owner.Owner, xproto.EventMaskNoEvent, string(ev.Bytes())).Check()
}

func (x *xembed) listen() {
	for {
		ev, err := x.conn.WaitForEvent()
		if ev == nil && err == nil {
			return
		}
		switch e := ev.(type) {
		case xproto.ExposeEvent:
			if e.Window == x.win && e.Count == 0 {
				x.paint()
			} else if x.menu != nil && e.Window == x.menu.win && e.Count == 0 {
				x.menu.draw()
			}
		case xproto.ConfigureNotifyEvent:
			if e.Window == x.win {
				x.w, x.h = e.Width, e.Height
				x.paint()
			}
		case xproto.ButtonPressEvent:
			x.press(e)
		case xproto.MotionNotifyEvent:
			if x.menu != nil && e.Event == x.menu.win {
				x.menu.hover(e.EventY)
			}
		case xproto.ClientMessageEvent:
			if e.Type == x.atoms.manager && xproto.Atom(e.Data.Data32[1]) == x.atoms.sel {
				_ = x.dock()
			}
		}
	}
}

func (x *xembed) press(e xproto.ButtonPressEvent) {
	if x.menu != nil {
		m := x.menu
		x.menu = nil
		if e.Event == m.win {
			if a, ok := m.at(e.EventX, e.EventY); ok {
				x.send(a)
			}
		}
		m.close()
		return
	}
	if e.Event != x.win {
		return
	}
	switch e.Detail {
	case xproto.ButtonIndex1:
		x.send(Open)
	case xproto.ButtonIndex3:
		x.menu = openTrayMenu(x.conn, x.screen, e.RootX, e.RootY)
	}
}

// paint draws the logo in the middle of the icon window: on a 32-bit
// visual with its corners clear, and otherwise on its own plate colour.
func (x *xembed) paint() {
	side := int(min(x.w, x.h))
	if side <= 0 {
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, int(x.w), int(x.h)))
	if !x.argb {
		draw.Draw(img, img.Bounds(), image.NewUniform(menuPlate), image.Point{}, draw.Src)
	}
	at := image.Pt((int(x.w)-side)/2, (int(x.h)-side)/2)
	draw.Draw(img, image.Rectangle{Min: at, Max: at.Add(image.Pt(side, side))}, icon(side), image.Point{}, draw.Over)
	putImage(x.conn, xproto.Drawable(x.win), x.gc, x.depth, img)
}

// putImage sends img as 32-bit pixels in the server's little-endian
// order: blue, green, red, then alpha, premultiplied as X expects.
func putImage(conn *xgb.Conn, d xproto.Drawable, gc xproto.Gcontext, depth byte, img *image.RGBA) {
	b := img.Bounds()
	data := make([]byte, 0, 4*b.Dx()*b.Dy())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.RGBAAt(x, y)
			data = append(data, c.B, c.G, c.R, c.A)
		}
	}
	xproto.PutImage(conn, xproto.ImageFormatZPixmap, d, gc, uint16(b.Dx()), uint16(b.Dy()), 0, 0, 0, depth, data)
}

// trayMenu is the menu a right click opens: an override-redirect window
// beside the pointer, one row per entry, Quit… last.
type trayMenu struct {
	conn    *xgb.Conn
	win     xproto.Window
	gc      xproto.Gcontext
	depth   byte
	w, rowH int
	hovered int
}

func openTrayMenu(conn *xgb.Conn, screen *xproto.ScreenInfo, rootX, rootY int16) *trayMenu {
	face := basicfont.Face7x13
	m := &trayMenu{conn: conn, depth: screen.RootDepth, hovered: -1, rowH: face.Metrics().Height.Ceil() + 2*menuPadY}
	for _, e := range entries {
		m.w = max(m.w, font.MeasureString(face, e.label).Ceil()+2*menuPadX)
	}
	h := m.rowH * len(entries)
	x, y := int(rootX), int(rootY)
	if y+h > int(screen.HeightInPixels) {
		y -= h
	}
	if x+m.w > int(screen.WidthInPixels) {
		x -= m.w
	}
	var err error
	if m.win, err = xproto.NewWindowId(conn); err != nil {
		return nil
	}
	events := uint32(xproto.EventMaskExposure | xproto.EventMaskButtonPress | xproto.EventMaskPointerMotion | xproto.EventMaskLeaveWindow)
	if err := xproto.CreateWindowChecked(conn, screen.RootDepth, m.win, screen.Root, int16(x), int16(y), uint16(m.w), uint16(h), 0,
		xproto.WindowClassInputOutput, screen.RootVisual, xproto.CwBackPixel|xproto.CwOverrideRedirect|xproto.CwEventMask,
		[]uint32{screen.BlackPixel, 1, events}).Check(); err != nil {
		return nil
	}
	if m.gc, err = xproto.NewGcontextId(conn); err != nil {
		return nil
	}
	_ = xproto.CreateGCChecked(conn, m.gc, xproto.Drawable(m.win), 0, nil).Check()
	_ = xproto.MapWindowChecked(conn, m.win).Check()
	// A click anywhere else reaches the menu too, and closes it.
	_, _ = xproto.GrabPointer(conn, true, m.win, xproto.EventMaskButtonPress|xproto.EventMaskPointerMotion,
		xproto.GrabModeAsync, xproto.GrabModeAsync, 0, 0, xproto.TimeCurrentTime).Reply()
	return m
}

func (m *trayMenu) row(y int16) int {
	if y < 0 {
		return -1
	}
	if r := int(y) / m.rowH; r < len(entries) {
		return r
	}
	return -1
}

func (m *trayMenu) at(x, y int16) (Action, bool) {
	r := m.row(y)
	if x < 0 || int(x) >= m.w || r < 0 {
		return 0, false
	}
	return entries[r].action, true
}

func (m *trayMenu) hover(y int16) {
	if r := m.row(y); r != m.hovered {
		m.hovered = r
		m.draw()
	}
}

func (m *trayMenu) draw() {
	if m == nil {
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, m.w, m.rowH*len(entries)))
	draw.Draw(img, img.Bounds(), image.NewUniform(menuPlate), image.Point{}, draw.Src)
	face := basicfont.Face7x13
	for i, e := range entries {
		top := i * m.rowH
		if i == m.hovered {
			draw.Draw(img, image.Rect(0, top, m.w, top+m.rowH), image.NewUniform(menuHover), image.Point{}, draw.Src)
		}
		d := font.Drawer{Dst: img, Src: image.NewUniform(color.RGBA(menuText)), Face: face,
			Dot: fixed.P(menuPadX, top+menuPadY+face.Metrics().Ascent.Ceil())}
		d.DrawString(e.label)
	}
	putImage(m.conn, xproto.Drawable(m.win), m.gc, m.depth, img)
}

func (m *trayMenu) close() {
	if m == nil {
		return
	}
	_ = xproto.UngrabPointerChecked(m.conn, xproto.TimeCurrentTime).Check()
	_ = xproto.DestroyWindowChecked(m.conn, m.win).Check()
}
