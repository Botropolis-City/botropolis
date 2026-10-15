//go:build !linux && !darwin

package state

func UnixSocketConnected(string) bool {
	return false
}
