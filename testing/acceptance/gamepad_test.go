//go:build linux

package acceptance_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"

	"botropolis.city/botropolis/testing/helpers"
)

const signalledRuns = 40

// openableInputs is every evdev device this user can open, which is what
// Ebitengine opens as the city starts.
func openableInputs() (paths, refused []string) {
	all, _ := filepath.Glob("/dev/input/event*")
	for _, p := range all {
		fd, err := unix.Open(p, unix.O_RDONLY|unix.O_NONBLOCK, 0)
		if err != nil {
			refused = append(refused, p)
			continue
		}
		_ = unix.Close(fd)
		paths = append(paths, p)
	}
	return paths, refused
}

// keepBusy opens and closes paths from workers goroutines until stop is
// called, so the city's opens wait on each device's mutex.
func keepBusy(paths []string, workers int) (stop func()) {
	done := make(chan struct{})
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				for _, p := range paths {
					select {
					case <-done:
						return
					default:
					}
					if fd, err := unix.Open(p, unix.O_RDONLY|unix.O_NONBLOCK, 0); err == nil {
						_ = unix.Close(fd)
					}
				}
			}
		}()
	}
	return func() { close(done); wg.Wait() }
}

// signalMainThread sends SIGURG, Go's own preemption signal, to the
// process's main thread, whose id is its pid, until stop is called.
func signalMainThread(p *os.Process) (stop func()) {
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for {
			select {
			case <-done:
				return
			default:
			}
			if unix.Tgkill(p.Pid, p.Pid, unix.SIGURG) != nil {
				return
			}
		}
	}()
	return func() { close(done); <-finished }
}

// Item 128. Opt-in beside the teardown soak, BOTROPOLIS_SOAK=1 or make
// soak-gamepad: unfixed, a start failed about one time in seven here, so
// 40 clean starts tell a fix from luck, and the unit test in pkg/sighold
// guards the mask on every run.
func TestStartUnderSignals(t *testing.T) {
	if os.Getenv("BOTROPOLIS_SOAK") == "" {
		t.Skip("set BOTROPOLIS_SOAK=1, or run make soak-gamepad")
	}
	helpers.NeedXvfb(t)
	inputs, refused := openableInputs()
	if len(inputs) == 0 {
		t.Skipf("no /dev/input/event* device can be opened here (refused: %v), so nothing can interrupt the city's open", refused)
	}

	t.Run("when a --home screenshot city starts while other processes hold the input devices busy", func(t *testing.T) {
		stopBusy := keepBusy(inputs, 4)
		defer stopBusy()

		t.Run("and its main thread is sent signals repeatedly", func(t *testing.T) {
			root := t.TempDir()
			var failures []string
			for i := range signalledRuns {
				if failed := screenshotRun(filepath.Join(root, fmt.Sprint(i)), signalMainThread); failed != "" {
					failures = append(failures, fmt.Sprintf("run %d: %s", i, failed))
				}
			}

			t.Run(fmt.Sprintf("it should start every one of %d times", signalledRuns), func(t *testing.T) {
				assert.Empty(t, failures)
			})
		})
	})
}
