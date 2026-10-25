//go:build darwin

package hotkey_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/hotkey"
)

// AppKit and Carbon deliver to the main thread, so the tests run beside
// an NSApplication that owns it, with one window, the way the city's
// does under Ebitengine. Item 95.
func init() { runtime.LockOSThread() }

type rect struct{ x, y, w, h float64 }

func TestMain(m *testing.M) {
	if _, err := purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		panic(err)
	}
	sel := objc.RegisterName
	app := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication"))
	app.Send(sel("setActivationPolicy:"), 0)
	app.Send(sel("finishLaunching"))
	window := objc.ID(objc.GetClass("NSWindow")).Send(sel("alloc")).Send(sel("initWithContentRect:styleMask:backing:defer:"),
		rect{200, 200, 480, 320}, uint(15), uint(2), false)
	title := objc.ID(objc.GetClass("NSString")).Send(sel("stringWithUTF8String:"), "botropolis hotkey test")
	window.Send(sel("setTitle:"), title)
	window.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
	go func() { os.Exit(m.Run()) }()
	app.Send(sel("run"))
}

func osascript(t *testing.T, script string) string {
	t.Helper()
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	require.NoError(t, err, "osascript %q: %s", script, out)
	return strings.TrimSpace(string(out))
}

func frontmost(t *testing.T) int {
	t.Helper()
	pid, err := strconv.Atoi(osascript(t, `tell application "System Events" to get unix id of first application process whose frontmost is true`))
	require.NoError(t, err)
	return pid
}

// finderInFront plays the user working in another application.
func finderInFront(t *testing.T) {
	t.Helper()
	osascript(t, `tell application "Finder" to activate`)
	require.True(t, eventually(func() bool { return frontmost(t) != os.Getpid() }), "Finder never came in front")
}

func pressCtrlAltF(t *testing.T) {
	t.Helper()
	osascript(t, `tell application "System Events" to keystroke "f" using {control down, option down}`)
}

// frame keeps what the screen shows, for Aria to judge, where the
// workflow collects it.
func frame(t *testing.T, name string) {
	t.Helper()
	dir := os.Getenv("BOTROPOLIS_FRAMES")
	if dir == "" {
		return
	}
	require.NoError(t, os.MkdirAll(dir, 0o755))
	out, err := exec.Command("screencapture", "-x", filepath.Join(dir, name+".png")).CombinedOutput()
	assert.NoError(t, err, "screencapture: %s", out)
}

func firedWithin(d hotkey.Desktop, wait time.Duration) bool {
	select {
	case <-d.Fired():
		return true
	case <-time.After(wait):
		return false
	}
}

func TestGlobalShortcutOnMacOS(t *testing.T) {
	d, err := hotkey.OpenMacOS(hotkey.Activate)
	require.NoError(t, err)
	t.Cleanup(d.Close)

	t.Run("when ctrl+alt+f is registered from a thread other than the main one, as the city's Update does", func(t *testing.T) {
		t.Run("it should be accepted", func(t *testing.T) {
			assert.NoError(t, d.Grab(ctrlAltF))
		})
	})

	t.Run("when ctrl+alt+f is registered and pressed through System Events with Finder in front", func(t *testing.T) {
		require.NoError(t, d.Grab(ctrlAltF), "Carbon refused a registration off the main thread")
		finderInFront(t)
		pressCtrlAltF(t)

		t.Run("it should fire", func(t *testing.T) {
			assert.True(t, firedWithin(d, 5*time.Second))
		})
	})

	t.Run("when it is released and pressed", func(t *testing.T) {
		require.NoError(t, d.Release())
		pressCtrlAltF(t)

		t.Run("it should not fire", func(t *testing.T) {
			assert.False(t, firedWithin(d, 2*time.Second))
		})
	})
}

func TestRaiseOnMacOS(t *testing.T) {
	for _, how := range []hotkey.Activation{hotkey.Activate, hotkey.ActivateIgnoringOtherApps, hotkey.SetFrontProcess} {
		t.Run("when Finder is in front and the shortcut is pressed and the window is raised with "+how.String(), func(t *testing.T) {
			d, err := hotkey.OpenMacOS(how)
			require.NoError(t, err)
			defer d.Close()
			require.NoError(t, d.Grab(ctrlAltF), "Carbon refused a registration off the main thread")
			finderInFront(t)
			pressCtrlAltF(t)
			require.True(t, firedWithin(d, 5*time.Second), "the hot key never fired from a System Events keystroke")
			d.Raise(0)
			forward := eventually(func() bool { return frontmost(t) == os.Getpid() })
			frame(t, "raise-"+how.String())

			t.Run("it should make this process the frontmost application", func(t *testing.T) {
				assert.True(t, forward)
			})
		})
	}
}
