//go:build !linux

package sighold

// Hold does nothing where Ebitengine does not open evdev devices.
func Hold() (release func()) { return func() {} }
