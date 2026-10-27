package tray

import (
	"fmt"
	"os"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

const (
	watcherName = "org.kde.StatusNotifierWatcher"
	watcherPath = dbus.ObjectPath("/StatusNotifierWatcher")
	itemIface   = "org.kde.StatusNotifierItem"
	itemPath    = dbus.ObjectPath("/StatusNotifierItem")
	menuIface   = "com.canonical.dbusmenu"
	menuPath    = dbus.ObjectPath("/MenuBar")
)

// entry is one line of the tray's menu. Quit… stays last, so whatever
// is added later goes above it.
type entry struct {
	id     int32
	label  string
	action Action
}

var entries = []entry{
	{1, "Open the city", Open},
	{2, "Quit…", Quit},
}

// sessionBus is the session bus DBUS_SESSION_BUS_ADDRESS names, or nil:
// with no address there is no session to put an icon in, and dbus's own
// autolaunch would start a bus nobody's panel is on.
func sessionBus(getenv func(string) string) *dbus.Conn {
	address := getenv("DBUS_SESSION_BUS_ADDRESS")
	if address == "" {
		return nil
	}
	conn, err := dbus.Connect(address)
	if err != nil {
		return nil
	}
	return conn
}

// probeWatcher says whether a StatusNotifierWatcher owns its name, and
// whether a host, the panel that draws the icons, has registered with it.
func probeWatcher(conn *dbus.Conn) (watcher, host bool) {
	if conn == nil {
		return false, false
	}
	if err := conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, watcherName).Store(&watcher); err != nil || !watcher {
		return false, false
	}
	v, err := conn.Object(watcherName, watcherPath).GetProperty(watcherName + ".IsStatusNotifierHostRegistered")
	if err != nil {
		return true, false
	}
	host, _ = v.Value().(bool)
	return true, host
}

// sni is the city's StatusNotifierItem, with its dbusmenu.
type sni struct {
	conn    *dbus.Conn
	name    string
	actions chan Action
}

func (s *sni) Actions() <-chan Action { return s.actions }

func (s *sni) Close() { _ = s.conn.Close() }

// send hands an action to the city without ever holding up the bus: a
// city that has not drained a click yet does not need a second one.
func (s *sni) send(a Action) {
	select {
	case s.actions <- a:
	default:
	}
}

func startSNI(conn *dbus.Conn) (Tray, error) {
	s := &sni{conn: conn, name: fmt.Sprintf("org.kde.StatusNotifierItem-%d-1", os.Getpid()), actions: make(chan Action, 4)}
	if err := s.export(); err != nil {
		return nil, err
	}
	reply, err := conn.RequestName(s.name, dbus.NameFlagDoNotQueue)
	if err != nil {
		return nil, err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return nil, fmt.Errorf("%s is taken on the session bus", s.name)
	}
	if err := s.register(); err != nil {
		return nil, err
	}
	s.follow()
	return s, nil
}

func (s *sni) register() error {
	return s.conn.Object(watcherName, watcherPath).Call(watcherName+".RegisterStatusNotifierItem", 0, s.name).Err
}

// follow registers again each time a watcher takes the name, so the icon
// comes back when the panel restarts.
func (s *sni) follow() {
	if err := s.conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, watcherName),
	); err != nil {
		return
	}
	signals := make(chan *dbus.Signal, 4)
	s.conn.Signal(signals)
	go func() {
		for sig := range signals {
			if sig.Name != "org.freedesktop.DBus.NameOwnerChanged" || len(sig.Body) < 3 {
				continue
			}
			if owner, _ := sig.Body[2].(string); owner != "" {
				_ = s.register()
			}
		}
	}()
}

type tooltip struct {
	Icon        string
	Pixmaps     []sniPixmap
	Title, Text string
}

func (s *sni) export() error {
	var pixmaps []sniPixmap
	for _, size := range iconSizes {
		pixmaps = append(pixmaps, pixmap(icon(size)))
	}
	if err := s.conn.Export(item{s}, itemPath, itemIface); err != nil {
		return err
	}
	str := func(v string) *prop.Prop { return &prop.Prop{Value: v} }
	if _, err := prop.Export(s.conn, itemPath, prop.Map{itemIface: {
		"Category":            str("ApplicationStatus"),
		"Id":                  str("botropolis"),
		"Title":               str("Botropolis"),
		"Status":              str("Active"),
		"WindowId":            {Value: int32(0)},
		"IconName":            str(""),
		"IconThemePath":       str(""),
		"IconPixmap":          {Value: pixmaps},
		"OverlayIconName":     str(""),
		"OverlayIconPixmap":   {Value: []sniPixmap{}},
		"AttentionIconName":   str(""),
		"AttentionIconPixmap": {Value: []sniPixmap{}},
		"AttentionMovieName":  str(""),
		"ToolTip":             {Value: tooltip{Pixmaps: []sniPixmap{}, Title: "Botropolis"}},
		"ItemIsMenu":          {Value: false},
		"Menu":                {Value: menuPath},
	}}); err != nil {
		return err
	}
	if err := s.conn.Export(menu{s}, menuPath, menuIface); err != nil {
		return err
	}
	_, err := prop.Export(s.conn, menuPath, prop.Map{menuIface: {
		"Version":       {Value: uint32(3)},
		"TextDirection": str("ltr"),
		"Status":        str("normal"),
		"IconThemePath": {Value: []string{}},
	}})
	return err
}

// item answers org.kde.StatusNotifierItem: a left click opens the city,
// and the host shows the menu itself on a right click.
type item struct{ s *sni }

func (i item) Activate(x, y int32) *dbus.Error {
	i.s.send(Open)
	return nil
}

func (i item) SecondaryActivate(x, y int32) *dbus.Error           { return nil }
func (i item) ContextMenu(x, y int32) *dbus.Error                 { return nil }
func (i item) Scroll(delta int32, orientation string) *dbus.Error { return nil }

// layout is a dbusmenu node: (ia{sv}av).
type layout struct {
	ID       int32
	Props    map[string]dbus.Variant
	Children []dbus.Variant
}

type groupProps struct {
	ID    int32
	Props map[string]dbus.Variant
}

type menuEvent struct {
	ID        int32
	EventID   string
	Data      dbus.Variant
	Timestamp uint32
}

// menu answers com.canonical.dbusmenu for a flat menu that never changes,
// so its revision is always 1.
type menu struct{ s *sni }

func props(e entry) map[string]dbus.Variant {
	return map[string]dbus.Variant{"label": dbus.MakeVariant(e.label), "enabled": dbus.MakeVariant(true), "visible": dbus.MakeVariant(true)}
}

func (m menu) GetLayout(parent, depth int32, names []string) (uint32, layout, *dbus.Error) {
	for _, e := range entries {
		if e.id == parent {
			return 1, layout{ID: e.id, Props: props(e), Children: []dbus.Variant{}}, nil
		}
	}
	root := layout{Props: map[string]dbus.Variant{"children-display": dbus.MakeVariant("submenu")}, Children: []dbus.Variant{}}
	if depth != 0 {
		for _, e := range entries {
			root.Children = append(root.Children, dbus.MakeVariant(layout{ID: e.id, Props: props(e), Children: []dbus.Variant{}}))
		}
	}
	return 1, root, nil
}

func (m menu) GetGroupProperties(ids []int32, names []string) ([]groupProps, *dbus.Error) {
	out := []groupProps{}
	for _, e := range entries {
		for _, id := range ids {
			if id == e.id {
				out = append(out, groupProps{e.id, props(e)})
			}
		}
	}
	return out, nil
}

func (m menu) GetProperty(id int32, name string) (dbus.Variant, *dbus.Error) {
	for _, e := range entries {
		if v, ok := props(e)[name]; ok && e.id == id {
			return v, nil
		}
	}
	return dbus.MakeVariant(""), nil
}

func (m menu) Event(id int32, eventID string, data dbus.Variant, timestamp uint32) *dbus.Error {
	if eventID != "clicked" {
		return nil
	}
	for _, e := range entries {
		if e.id == id {
			m.s.send(e.action)
		}
	}
	return nil
}

func (m menu) EventGroup(events []menuEvent) ([]int32, *dbus.Error) {
	for _, e := range events {
		_ = m.Event(e.ID, e.EventID, e.Data, e.Timestamp)
	}
	return []int32{}, nil
}

func (m menu) AboutToShow(id int32) (bool, *dbus.Error) { return false, nil }

func (m menu) AboutToShowGroup(ids []int32) ([]int32, []int32, *dbus.Error) {
	return []int32{}, []int32{}, nil
}
