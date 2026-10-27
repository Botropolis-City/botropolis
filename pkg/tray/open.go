package tray

import "fmt"

// Start puts the city's icon in the desktop's tray, or says why there is
// none. StatusNotifierItem and the XEmbed fallback are the next slices of
// item 293; macOS's menu bar is item 296. Until then no desktop has one,
// and on_close = tray asks instead.
func Start(goos string, getenv func(string) string) (Tray, error) {
	return nil, fmt.Errorf("%w on this desktop yet", ErrNoTray)
}
