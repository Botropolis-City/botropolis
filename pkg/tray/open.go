package tray

import "fmt"

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
	conn := sessionBus(getenv)
	var f found
	f.watcher, f.host = probeWatcher(conn)
	k, err := choose(f)
	if k == kindSNI {
		icon, err := startSNI(conn)
		if err == nil {
			return icon, nil
		}
		_ = conn.Close()
		return nil, fmt.Errorf("%w: %v", ErrNoTray, err)
	}
	if conn != nil {
		_ = conn.Close()
	}
	return nil, err
}
