//go:build !linux && !darwin

package proc

func startTime(int) (uint64, bool) {
	return 0, false
}
