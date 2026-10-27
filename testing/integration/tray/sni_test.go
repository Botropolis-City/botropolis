//go:build linux

package tray_test

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/tray"
)

const (
	watcherName  = "org.kde.StatusNotifierWatcher"
	watcherPath  = dbus.ObjectPath("/StatusNotifierWatcher")
	itemIface    = "org.kde.StatusNotifierItem"
	menuIface    = "com.canonical.dbusmenu"
	settleWithin = 5 * time.Second
)

// privateBus is a session bus of the test's own, gone when it ends, so
// the test reads nothing of the desktop it runs on.
func privateBus(t *testing.T) string {
	t.Helper()
	need(t, "dbus-daemon", "test the StatusNotifierItem against a session bus")
	// A socket's path is capped near 108 bytes, and t.TempDir's carries
	// the test's whole name.
	dir, err := os.MkdirTemp("", "tray")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	cmd := exec.Command("dbus-daemon", "--session", "--nofork", "--print-address=1", "--address=unix:dir="+dir)
	out, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	address, err := bufio.NewReader(out).ReadString('\n')
	require.NoError(t, err)
	return strings.TrimSpace(address)
}

// fakeWatcher is the StatusNotifierWatcher a panel runs: it keeps the
// items that register, and says a host is registered when told to.
type fakeWatcher struct{ registered chan string }

func (w *fakeWatcher) RegisterStatusNotifierItem(service string) *dbus.Error {
	w.registered <- service
	return nil
}

func (w *fakeWatcher) RegisterStatusNotifierHost(string) *dbus.Error { return nil }

func watch(t *testing.T, address string, host bool) (*dbus.Conn, *fakeWatcher) {
	t.Helper()
	conn, err := dbus.Connect(address)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	w := &fakeWatcher{registered: make(chan string, 1)}
	require.NoError(t, conn.Export(w, watcherPath, watcherName))
	_, err = prop.Export(conn, watcherPath, prop.Map{watcherName: {
		"IsStatusNotifierHostRegistered": {Value: host},
		"RegisteredStatusNotifierItems":  {Value: []string{}},
		"ProtocolVersion":                {Value: int32(0)},
	}})
	require.NoError(t, err)
	reply, err := conn.RequestName(watcherName, dbus.NameFlagDoNotQueue)
	require.NoError(t, err)
	require.Equal(t, dbus.RequestNameReplyPrimaryOwner, reply)
	return conn, w
}

func start(t *testing.T, address string) (tray.Tray, error) {
	t.Helper()
	env := map[string]string{"DBUS_SESSION_BUS_ADDRESS": address}
	icon, err := tray.Start("linux", func(k string) string { return env[k] })
	if icon != nil {
		t.Cleanup(icon.Close)
	}
	return icon, err
}

func next(t *testing.T, icon tray.Tray) (tray.Action, bool) {
	t.Helper()
	select {
	case a := <-icon.Actions():
		return a, true
	case <-time.After(settleWithin):
		return 0, false
	}
}

type menuLayout struct {
	ID       int32
	Props    map[string]dbus.Variant
	Children []dbus.Variant
}

// Item 293: the city's icon on a panel that speaks StatusNotifierItem.
func TestStatusNotifierItem(t *testing.T) {
	t.Run("when a watcher with a host is on the bus", func(t *testing.T) {
		address := privateBus(t)
		conn, w := watch(t, address, true)
		icon, err := start(t, address)
		require.NoError(t, err)
		var service string
		select {
		case service = <-w.registered:
		case <-time.After(settleWithin):
		}

		t.Run("it should register with the watcher", func(t *testing.T) {
			assert.NotEmpty(t, service)
		})

		item := conn.Object(service, "/StatusNotifierItem")
		menuPath := dbus.ObjectPath("/MenuBar")
		if v, err := item.GetProperty(itemIface + ".Menu"); err == nil {
			menuPath, _ = v.Value().(dbus.ObjectPath)
		}
		menu := conn.Object(service, menuPath)
		var revision uint32
		var root menuLayout
		layoutErr := menu.Call(menuIface+".GetLayout", 0, int32(0), int32(-1), []string{}).Store(&revision, &root)
		labels := map[string]int32{}
		var order []string
		for _, child := range root.Children {
			var entry menuLayout
			if fields, ok := child.Value().([]any); ok && len(fields) == 3 {
				entry.ID, _ = fields[0].(int32)
				entry.Props, _ = fields[1].(map[string]dbus.Variant)
			}
			label, _ := entry.Props["label"].Value().(string)
			labels[label] = entry.ID
			order = append(order, label)
		}

		t.Run("it should offer its menu", func(t *testing.T) {
			assert.NoError(t, layoutErr)
		})

		t.Run("it should put Quit… last in the menu", func(t *testing.T) {
			require.NotEmpty(t, order)
			assert.Equal(t, "Quit…", order[len(order)-1])
		})

		t.Run("it should show its icon as pixels", func(t *testing.T) {
			v, err := item.GetProperty(itemIface + ".IconPixmap")
			require.NoError(t, err)
			assert.NotEmpty(t, v.Value())
		})

		t.Run("and the icon is left-clicked", func(t *testing.T) {
			require.NoError(t, item.Call(itemIface+".Activate", 0, int32(0), int32(0)).Err)
			got, ok := next(t, icon)

			t.Run("it should open the city", func(t *testing.T) {
				assert.Equal(t, [2]any{tray.Open, true}, [2]any{got, ok})
			})
		})

		for _, c := range []struct {
			label string
			want  tray.Action
		}{{"Open the city", tray.Open}, {"Quit…", tray.Quit}} {
			t.Run("and the menu's "+c.label+" is clicked", func(t *testing.T) {
				id, found := labels[c.label]
				require.True(t, found, "no entry %q in %v", c.label, order)
				require.NoError(t, menu.Call(menuIface+".Event", 0, id, "clicked", dbus.MakeVariant(""), uint32(0)).Err)
				got, ok := next(t, icon)

				t.Run("it should deliver it", func(t *testing.T) {
					assert.Equal(t, [2]any{c.want, true}, [2]any{got, ok})
				})
			})
		}
	})

	t.Run("when a watcher is on the bus with no host, and there is no display", func(t *testing.T) {
		address := privateBus(t)
		watch(t, address, false)
		_, err := start(t, address)

		t.Run("it should say there is no tray", func(t *testing.T) {
			assert.ErrorIs(t, err, tray.ErrNoTray)
		})
	})

	t.Run("when nothing is on the bus, and there is no display", func(t *testing.T) {
		_, err := start(t, privateBus(t))

		t.Run("it should say there is no tray", func(t *testing.T) {
			assert.ErrorIs(t, err, tray.ErrNoTray)
		})
	})
}
