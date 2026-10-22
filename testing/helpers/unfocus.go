package helpers

import (
	"testing"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"

	"botropolis.city/botropolis/pkg/xvfb"
)

// xconn connects to a virtual display, and closes the connection when
// the test ends.
func xconn(t testing.TB, s *xvfb.Server) (*xgb.Conn, xproto.Window) {
	t.Helper()
	t.Setenv("XAUTHORITY", s.Authority)
	conn, err := xgb.NewConnDisplay(s.Display)
	if err != nil {
		t.Fatalf("connecting to %s: %v", s.Display, err)
	}
	t.Cleanup(conn.Close)
	return conn, xproto.Setup(conn).DefaultScreen(conn).Root
}

// Unfocus gives a virtual display's keyboard focus to its root window,
// as clicking elsewhere would, so a city open on it is visible and
// unfocused. A display with no window manager otherwise leaves the
// window it opened focused (item 197).
func Unfocus(t testing.TB, s *xvfb.Server) {
	t.Helper()
	conn, root := xconn(t, s)
	if err := xproto.SetInputFocusChecked(conn, xproto.InputFocusNone, root, xproto.TimeCurrentTime).Check(); err != nil {
		t.Fatalf("moving the focus on %s: %v", s.Display, err)
	}
}

// Hide unmaps every window on a virtual display, as i3's scratchpad
// does to a window it holds (item 197).
func Hide(t testing.TB, s *xvfb.Server) {
	t.Helper()
	conn, root := xconn(t, s)
	tree, err := xproto.QueryTree(conn, root).Reply()
	if err != nil {
		t.Fatalf("listing the windows on %s: %v", s.Display, err)
	}
	for _, w := range tree.Children {
		if err := xproto.UnmapWindowChecked(conn, w).Check(); err != nil {
			t.Fatalf("unmapping a window on %s: %v", s.Display, err)
		}
	}
}
