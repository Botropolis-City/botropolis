package tray

import (
	"errors"
	"fmt"

	"github.com/jezek/xgb"
)

// Start puts the city's icon in the desktop's tray, or says why there is
// none: StatusNotifierItem where a panel hosts it, the XEmbed tray where
// only that is found. macOS's menu bar is item 296.
func Start(goos string, getenv func(string) string) (Tray, error) {
	switch goos {
	case "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
	case "darwin":
		return nil, fmt.Errorf("%w: the menu bar comes with item 296", ErrNoTray)
	default:
		return nil, fmt.Errorf("%w on %s", ErrNoTray, goos)
	}
	bus := sessionBus(getenv)
	var x *xgb.Conn
	if display := getenv("DISPLAY"); display != "" {
		x, _ = xgb.NewConnDisplay(display)
	}
	var f found
	f.watcher, f.host = probeWatcher(bus)
	f.manager = probeXEmbed(x)
	k, err := choose(f)
	var icon Tray
	switch k {
	case kindSNI:
		if icon, err = startSNI(bus); err == nil {
			bus = nil
		}
	case kindXEmbed:
		if icon, err = startXEmbed(x); err == nil {
			x = nil
		}
	}
	if bus != nil {
		_ = bus.Close()
	}
	if x != nil {
		x.Close()
	}
	switch {
	case err == nil:
		return icon, nil
	case errors.Is(err, ErrNoTray):
		return nil, err
	}
	return nil, fmt.Errorf("%w: %v", ErrNoTray, err)
}
