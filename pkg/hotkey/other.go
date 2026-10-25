//go:build !darwin

package hotkey

func openMacOS() (Desktop, error) {
	return nil, unsupportedHere()
}
