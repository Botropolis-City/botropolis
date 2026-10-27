//go:build linux

package tray_test

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/tray"
	"botropolis.city/botropolis/pkg/xvfb"
)

// need fails under CI rather than skip, because the private server is
// the whole proof the tray works.
func need(t *testing.T, binary, why string) {
	t.Helper()
	if _, err := exec.LookPath(binary); err == nil {
		return
	}
	if os.Getenv("CI") != "" {
		t.Fatalf("%s is not installed, and CI must %s", binary, why)
	}
	t.Skipf("%s is not installed, so %s went untested", binary, why)
}

// manager is the XEmbed system tray i3bar or polybar runs: it owns the
// screen's selection, and maps each window that asks to dock where the
// test can click it.
type manager struct {
	display string
	conn    *xgb.Conn
	root    xproto.Window
	opcode  xproto.Atom
	sel     xproto.Atom
	docked  chan xproto.Window
	popups  chan xproto.Window
}

const iconAt, iconSide = 100, 32

func atom(t *testing.T, conn *xgb.Conn, name string) xproto.Atom {
	t.Helper()
	reply, err := xproto.InternAtom(conn, false, uint16(len(name)), name).Reply()
	require.NoError(t, err)
	return reply.Atom
}

func startManager(t *testing.T) *manager {
	t.Helper()
	need(t, "Xvfb", "test the XEmbed tray against an X server")
	x, err := xvfb.Start(t.TempDir(), 800, 600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = x.Close() })
	t.Setenv("XAUTHORITY", x.Authority)
	m := &manager{display: x.Display, docked: make(chan xproto.Window, 4), popups: make(chan xproto.Window, 4)}
	m.conn, err = xgb.NewConnDisplay(m.display)
	require.NoError(t, err)
	t.Cleanup(m.conn.Close)
	require.NoError(t, xtest.Init(m.conn))
	m.root = xproto.Setup(m.conn).DefaultScreen(m.conn).Root
	m.opcode = atom(t, m.conn, "_NET_SYSTEM_TRAY_OPCODE")
	m.sel = atom(t, m.conn, "_NET_SYSTEM_TRAY_S0")
	require.NoError(t, xproto.ChangeWindowAttributesChecked(m.conn, m.root, xproto.CwEventMask,
		[]uint32{xproto.EventMaskSubstructureNotify}).Check())
	m.own(t, m.conn)
	go m.listen()
	return m
}

// own takes the tray selection on conn with a window of its own, and
// announces it the way a tray that starts does.
func (m *manager) own(t *testing.T, conn *xgb.Conn) {
	t.Helper()
	w, err := xproto.NewWindowId(conn)
	require.NoError(t, err)
	require.NoError(t, xproto.CreateWindowChecked(conn, 0, w, m.root, 0, 0, 1, 1, 0, xproto.WindowClassInputOnly, 0, 0, nil).Check())
	require.NoError(t, xproto.SetSelectionOwnerChecked(conn, w, m.sel, xproto.TimeCurrentTime).Check())
	announce := xproto.ClientMessageEvent{Format: 32, Window: m.root, Type: atom(t, conn, "MANAGER"),
		Data: xproto.ClientMessageDataUnionData32New([]uint32{uint32(xproto.TimeCurrentTime), uint32(m.sel), uint32(w), 0, 0})}
	require.NoError(t, xproto.SendEventChecked(conn, false, m.root, xproto.EventMaskStructureNotify, string(announce.Bytes())).Check())
}

func (m *manager) listen() {
	for {
		ev, err := m.conn.WaitForEvent()
		if ev == nil && err == nil {
			return
		}
		switch e := ev.(type) {
		case xproto.ClientMessageEvent:
			if e.Type == m.opcode && e.Data.Data32[1] == 0 {
				w := xproto.Window(e.Data.Data32[2])
				_ = xproto.ConfigureWindowChecked(m.conn, w, xproto.ConfigWindowX|xproto.ConfigWindowY|xproto.ConfigWindowWidth|xproto.ConfigWindowHeight,
					[]uint32{iconAt, iconAt, iconSide, iconSide}).Check()
				_ = xproto.MapWindowChecked(m.conn, w).Check()
				m.docked <- w
			}
		case xproto.MapNotifyEvent:
			if e.OverrideRedirect {
				m.popups <- e.Window
			}
		}
	}
}

func (m *manager) click(t *testing.T, button byte, x, y int16) {
	t.Helper()
	require.NoError(t, xtest.FakeInputChecked(m.conn, xproto.MotionNotify, 0, 0, m.root, x, y, 0).Check())
	require.NoError(t, xtest.FakeInputChecked(m.conn, xproto.ButtonPress, button, 0, m.root, 0, 0, 0).Check())
	require.NoError(t, xtest.FakeInputChecked(m.conn, xproto.ButtonRelease, button, 0, m.root, 0, 0, 0).Check())
	m.conn.Sync()
}

func (m *manager) startTray(t *testing.T) (tray.Tray, error) {
	t.Helper()
	env := map[string]string{"DISPLAY": m.display}
	icon, err := tray.Start("linux", func(k string) string { return env[k] })
	if icon != nil {
		t.Cleanup(icon.Close)
	}
	return icon, err
}

func waitWindow(ch chan xproto.Window) (xproto.Window, bool) {
	select {
	case w := <-ch:
		return w, true
	case <-time.After(settleWithin):
		return 0, false
	}
}

// Item 293: the city's icon in the XEmbed tray of i3bar and polybar,
// where no StatusNotifierItem host answers.
func TestXEmbed(t *testing.T) {
	t.Run("when an XEmbed manager owns the selection, and there is no session bus", func(t *testing.T) {
		m := startManager(t)
		icon, err := m.startTray(t)
		require.NoError(t, err)
		_, docked := waitWindow(m.docked)

		t.Run("it should ask the manager to dock its window", func(t *testing.T) {
			assert.True(t, docked)
		})

		t.Run("and the icon is left-clicked", func(t *testing.T) {
			m.click(t, 1, iconAt+iconSide/2, iconAt+iconSide/2)
			got, ok := next(t, icon)

			t.Run("it should open the city", func(t *testing.T) {
				assert.Equal(t, [2]any{tray.Open, true}, [2]any{got, ok})
			})
		})

		for _, c := range []struct {
			name string
			row  func(g *xproto.GetGeometryReply) int16
			want tray.Action
		}{
			{"first entry", func(g *xproto.GetGeometryReply) int16 { return g.Y + 3 }, tray.Open},
			{"last entry", func(g *xproto.GetGeometryReply) int16 { return g.Y + int16(g.Height) - 3 }, tray.Quit},
		} {
			t.Run("and the icon is right-clicked", func(t *testing.T) {
				m.click(t, 3, iconAt+iconSide/2, iconAt+iconSide/2)
				popup, shown := waitWindow(m.popups)

				t.Run("it should open its menu", func(t *testing.T) {
					assert.True(t, shown)
				})
				if !shown {
					return
				}
				geometry, err := xproto.GetGeometry(m.conn, xproto.Drawable(popup)).Reply()
				require.NoError(t, err)

				t.Run("and the menu's "+c.name+" is clicked", func(t *testing.T) {
					m.click(t, 1, geometry.X+int16(geometry.Width)/2, c.row(geometry))
					got, ok := next(t, icon)

					t.Run("it should deliver it", func(t *testing.T) {
						assert.Equal(t, [2]any{c.want, true}, [2]any{got, ok})
					})
				})
			})
		}

		t.Run("and another manager takes the selection", func(t *testing.T) {
			next, err := xgb.NewConnDisplay(m.display)
			require.NoError(t, err)
			t.Cleanup(next.Close)
			asked := make(chan xproto.Window, 1)
			go func() {
				for {
					ev, err := next.WaitForEvent()
					if ev == nil && err == nil {
						return
					}
					if e, ok := ev.(xproto.ClientMessageEvent); ok && e.Type == m.opcode && e.Data.Data32[1] == 0 {
						asked <- xproto.Window(e.Data.Data32[2])
					}
				}
			}()
			m.own(t, next)
			_, again := waitWindow(asked)

			t.Run("it should ask to dock again", func(t *testing.T) {
				assert.True(t, again)
			})
		})
	})

	t.Run("when nothing owns the selection, and there is no session bus", func(t *testing.T) {
		need(t, "Xvfb", "test the XEmbed tray against an X server")
		x, err := xvfb.Start(t.TempDir(), 800, 600)
		require.NoError(t, err)
		t.Cleanup(func() { _ = x.Close() })
		t.Setenv("XAUTHORITY", x.Authority)
		_, err = tray.Start("linux", func(k string) string { return map[string]string{"DISPLAY": x.Display}[k] })

		t.Run("it should say there is no tray", func(t *testing.T) {
			assert.ErrorIs(t, err, tray.ErrNoTray)
		})
	})
}
