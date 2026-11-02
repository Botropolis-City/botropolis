package measure

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// CPUSysfs is where Linux lays out the cores.
const CPUSysfs = "/sys/devices/system/cpu"

var cpuDir = regexp.MustCompile(`^cpu(\d+)$`)

// CoreClasses groups the cores under sysfs by their maximum frequency,
// fastest first, each group in order (item 330). A thread's CPU for the
// same work moves with the kind of core it runs on, so a measure held to
// a bar runs on the first group.
func CoreClasses(sysfs string) ([][]int, error) {
	entries, err := os.ReadDir(sysfs)
	if err != nil {
		return nil, err
	}
	byFreq := map[int][]int{}
	for _, e := range entries {
		m := cpuDir.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		b, err := os.ReadFile(filepath.Join(sysfs, e.Name(), "cpufreq", "cpuinfo_max_freq"))
		if err != nil {
			continue
		}
		khz, err := strconv.Atoi(strings.TrimSpace(string(b)))
		if err != nil {
			continue
		}
		cpu, _ := strconv.Atoi(m[1])
		byFreq[khz] = append(byFreq[khz], cpu)
	}
	if len(byFreq) == 0 {
		return nil, fmt.Errorf("no core under %s says its maximum frequency, so the cores cannot be told apart", sysfs)
	}
	freqs := make([]int, 0, len(byFreq))
	for f := range byFreq {
		freqs = append(freqs, f)
	}
	slices.Sort(freqs)
	slices.Reverse(freqs)
	classes := make([][]int, 0, len(freqs))
	for _, f := range freqs {
		slices.Sort(byFreq[f])
		classes = append(classes, byFreq[f])
	}
	return classes, nil
}

// Pinned runs f on the calling goroutine's thread with that thread allowed
// only cores, and gives it back the cores it had.
func Pinned(cores []int, f func()) error {
	if len(cores) == 0 {
		return errors.New("measure: pinned to no cores")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var was, only unix.CPUSet
	if err := unix.SchedGetaffinity(0, &was); err != nil {
		return err
	}
	for _, c := range cores {
		only.Set(c)
	}
	if err := unix.SchedSetaffinity(0, &only); err != nil {
		return fmt.Errorf("measure: pinning to cores %v: %w", cores, err)
	}
	defer func() { _ = unix.SchedSetaffinity(0, &was) }()
	f()
	return nil
}
