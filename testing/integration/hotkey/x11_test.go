package hotkey_test

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/hotkey"
)

// Xvfb is the whole proof that the X11 part works, so under CI a missing
// one fails the run rather than reading green by skipping. Item 84.
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

// server is a private X server for one test, and a client of it that
// plays the user: pressing keys through XTEST and opening windows.
type server struct {
	display string
	conn    *xgb.Conn
	root    xproto.Window
}

func startServer(t *testing.T) *server {
	t.Helper()
	need(t, "Xvfb", "test the global shortcut against an X server")
	read, write, err := os.Pipe()
	require.NoError(t, err)
	cmd := exec.Command("Xvfb", "-displayfd", "3", "-screen", "0", "800x600x24", "-nolisten", "tcp")
	cmd.ExtraFiles = []*os.File{write}
	require.NoError(t, cmd.Start())
	_ = write.Close()
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	line, err := bufio.NewReader(read).ReadString('\n')
	require.NoError(t, err)
	_ = read.Close()
	s := &server{display: ":" + strings.TrimSpace(line)}
	s.conn, err = xgb.NewConnDisplay(s.display)
	require.NoError(t, err)
	t.Cleanup(s.conn.Close)
	require.NoError(t, xtest.Init(s.conn))
	s.root = xproto.Setup(s.conn).DefaultScreen(s.conn).Root
	return s
}

func (s *server) desktop(t *testing.T) hotkey.Desktop {
	t.Helper()
	d, err := hotkey.Open("linux", func(k string) string {
		if k == "DISPLAY" {
			return s.display
		}
		return ""
	})
	require.NoError(t, err)
	t.Cleanup(d.Close)
	return d
}

func (s *server) keycode(t *testing.T, sym xproto.Keysym) byte {
	t.Helper()
	setup := xproto.Setup(s.conn)
	reply, err := xproto.GetKeyboardMapping(s.conn, setup.MinKeycode, byte(setup.MaxKeycode-setup.MinKeycode+1)).Reply()
	require.NoError(t, err)
	for i, got := range reply.Keysyms {
		if got == sym {
			return byte(setup.MinKeycode) + byte(i/int(reply.KeysymsPerKeycode))
		}
	}
	t.Fatalf("the server's keyboard has no keysym %#x", sym)
	return 0
}

// press holds each key down in order and lets them go in reverse.
func (s *server) press(t *testing.T, syms ...xproto.Keysym) {
	t.Helper()
	codes := make([]byte, len(syms))
	for i, sym := range syms {
		codes[i] = s.keycode(t, sym)
		require.NoError(t, xtest.FakeInputChecked(s.conn, xproto.KeyPress, codes[i], 0, s.root, 0, 0, 0).Check())
	}
	for i := len(codes) - 1; i >= 0; i-- {
		require.NoError(t, xtest.FakeInputChecked(s.conn, xproto.KeyRelease, codes[i], 0, s.root, 0, 0, 0).Check())
	}
}

const (
	symControl xproto.Keysym = 0xffe3
	symAlt     xproto.Keysym = 0xffe9
	symNumLock xproto.Keysym = 0xff7f
	symF       xproto.Keysym = 0x66
)

var ctrlAltF = config.Chord{Ctrl: true, Alt: true, Key: "f"}

func (s *server) pressCtrlAltF(t *testing.T) { s.press(t, symControl, symAlt, symF) }

// fired waits a moment for a press to arrive.
func fired(d hotkey.Desktop) (uint32, bool) {
	select {
	case at := <-d.Fired():
		return at, true
	case <-time.After(2 * time.Second):
		return 0, false
	}
}

func TestGlobalShortcutOnX11(t *testing.T) {
	t.Run("when a combination is grabbed and pressed through XTEST", func(t *testing.T) {
		s := startServer(t)
		d := s.desktop(t)
		require.NoError(t, d.Grab(ctrlAltF))
		s.pressCtrlAltF(t)
		at, ok := fired(d)

		t.Run("it should fire with the key event's time", func(t *testing.T) {
			assert.Equal(t, [2]bool{true, true}, [2]bool{ok, at != 0})
		})

		t.Run("and Num Lock is on", func(t *testing.T) {
			s.press(t, symNumLock)
			t.Cleanup(func() { s.press(t, symNumLock) })
			s.pressCtrlAltF(t)
			_, ok := fired(d)

			t.Run("it should still fire", func(t *testing.T) {
				assert.True(t, ok)
			})
		})
	})

	t.Run("when the combination is released and pressed", func(t *testing.T) {
		s := startServer(t)
		d := s.desktop(t)
		require.NoError(t, d.Grab(ctrlAltF))
		require.NoError(t, d.Release())
		s.pressCtrlAltF(t)
		_, ok := fired(d)

		t.Run("it should not fire", func(t *testing.T) {
			assert.False(t, ok)
		})
	})

	t.Run("when another client already holds it", func(t *testing.T) {
		s := startServer(t)
		other := s.desktop(t)
		require.NoError(t, other.Grab(ctrlAltF))
		err := s.desktop(t).Grab(ctrlAltF)

		t.Run("it should report it held", func(t *testing.T) {
			assert.True(t, errors.Is(err, hotkey.ErrHeld), "got %v", err)
		})
	})
}

// window opens a top-level window, marked as this process's or another's.
func (s *server) window(t *testing.T, pid uint32) xproto.Window {
	t.Helper()
	id, err := xproto.NewWindowId(s.conn)
	require.NoError(t, err)
	screen := xproto.Setup(s.conn).DefaultScreen(s.conn)
	require.NoError(t, xproto.CreateWindowChecked(s.conn, screen.RootDepth, id, s.root, 0, 0, 200, 200, 0,
		xproto.WindowClassInputOutput, screen.RootVisual, 0, nil).Check())
	pidAtom := s.atom(t, "_NET_WM_PID")
	value := make([]byte, 4)
	xgb.Put32(value, pid)
	require.NoError(t, xproto.ChangePropertyChecked(s.conn, xproto.PropModeReplace, id, pidAtom, xproto.AtomCardinal, 32, 1, value).Check())
	class := "botropolis\x00Botropolis\x00"
	require.NoError(t, xproto.ChangePropertyChecked(s.conn, xproto.PropModeReplace, id, xproto.AtomWmClass, xproto.AtomString, 8, uint32(len(class)), []byte(class)).Check())
	require.NoError(t, xproto.MapWindowChecked(s.conn, id).Check())
	return id
}

func (s *server) atom(t *testing.T, name string) xproto.Atom {
	t.Helper()
	reply, err := xproto.InternAtom(s.conn, false, uint16(len(name)), name).Reply()
	require.NoError(t, err)
	return reply.Atom
}

func (s *server) focus(t *testing.T) xproto.Window {
	t.Helper()
	reply, err := xproto.GetInputFocus(s.conn).Reply()
	require.NoError(t, err)
	return reply.Focus
}

// eventually polls until cond holds or a few seconds pass.
func eventually(cond func() bool) bool {
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if cond() {
			return true
		}
	}
	return cond()
}

func TestRaiseOnX11(t *testing.T) {
	ours := uint32(os.Getpid())

	t.Run("when the city's window is raised on a bare server with no window manager", func(t *testing.T) {
		s := startServer(t)
		win := s.window(t, ours)
		other := s.window(t, 1)
		require.NoError(t, xproto.SetInputFocusChecked(s.conn, xproto.InputFocusParent, other, xproto.TimeCurrentTime).Check())
		raised := s.desktop(t).Raise(0)
		tree, err := xproto.QueryTree(s.conn, s.root).Reply()
		require.NoError(t, err)

		t.Run("it should say it raised it", func(t *testing.T) {
			assert.True(t, raised)
		})

		t.Run("it should put the window on top", func(t *testing.T) {
			assert.Equal(t, win, tree.Children[len(tree.Children)-1])
		})

		t.Run("it should give the window focus", func(t *testing.T) {
			assert.Equal(t, win, s.focus(t))
		})
	})

	t.Run("when the city's window is raised under i3", func(t *testing.T) {
		s := startServer(t)
		i3 := startI3(t, s)
		win := s.window(t, ours)
		require.True(t, eventually(func() bool { return s.focus(t) == win }), "i3 should focus the new window")

		t.Run("and it is on a workspace that is not showing", func(t *testing.T) {
			i3.msg(t, "workspace 2")
			other := s.window(t, 1)
			require.True(t, eventually(func() bool { return s.focus(t) == other }), "i3 should focus the window on workspace 2")
			raised := s.desktop(t).Raise(0)

			t.Run("it should say it asked", func(t *testing.T) {
				assert.True(t, raised)
			})

			t.Run("it should be focused", func(t *testing.T) {
				assert.True(t, eventually(func() bool { return s.focus(t) == win }))
			})
		})
	})
}

// i3 is a window manager of the test's own, whose IPC socket is kept in
// the test's directory so it never meets the desktop's i3.
type i3 struct{ display, socket string }

func startI3(t *testing.T, s *server) i3 {
	t.Helper()
	need(t, "i3", "test raising the window under i3")
	dir := t.TempDir()
	conf := filepath.Join(dir, "config")
	socket := filepath.Join(dir, "ipc")
	require.NoError(t, os.WriteFile(conf, []byte("font pango:monospace 8\nipc-socket "+socket+"\n"), 0o600))
	cmd := exec.Command("i3", "-c", conf)
	cmd.Env = append(os.Environ(), "DISPLAY="+s.display, "XDG_RUNTIME_DIR="+dir)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	check := s.atom(t, "_NET_SUPPORTING_WM_CHECK")
	require.True(t, eventually(func() bool {
		reply, err := xproto.GetProperty(s.conn, false, s.root, check, xproto.AtomWindow, 0, 1).Reply()
		return err == nil && reply.ValueLen > 0
	}), "i3 should start: %s", &out)
	return i3{display: s.display, socket: socket}
}

func (w i3) msg(t *testing.T, command string) {
	t.Helper()
	cmd := exec.Command("i3-msg", "-s", w.socket, command)
	cmd.Env = append(os.Environ(), "DISPLAY="+w.display)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, fmt.Sprintf("i3-msg %s: %s", command, out))
}
