//go:build linux

package acceptance_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"botropolis.city/botropolis/pkg/xvfb"
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
				failures = append(failures, logRun(t, i, func() string {
					return screenshotRun(filepath.Join(root, fmt.Sprint(i)), signalMainThread)
				})...)
			}

			t.Run(fmt.Sprintf("it should start every one of %d times", signalledRuns), func(t *testing.T) {
				assert.Empty(t, failures)
			})
		})
	})

	t.Run("when the demo tool draws the building sheet while other processes hold the input devices busy", func(t *testing.T) {
		demo := filepath.Join(t.TempDir(), "botropolis-demo")
		build, err := exec.Command("go", "build", "-o", demo, "../../cmd/botropolis-demo").CombinedOutput()
		require.NoError(t, err, "%s", build)
		stopBusy := keepBusy(inputs, 4)
		defer stopBusy()

		t.Run("and its main thread is sent signals repeatedly", func(t *testing.T) {
			root := t.TempDir()
			var failures []string
			for i := range signalledRuns {
				failures = append(failures, logRun(t, i, func() string {
					return sheetRun(demo, filepath.Join(root, fmt.Sprint(i)), signalMainThread)
				})...)
			}

			t.Run(fmt.Sprintf("it should finish every one of %d times", signalledRuns), func(t *testing.T) {
				assert.Empty(t, failures)
			})
		})
	})
}

// logRun runs one soak run and logs how it went and how long it took as it
// finishes, so a soak that runs out of time still shows its runs (item 230).
func logRun(t *testing.T, i int, run func() string) []string {
	t.Helper()
	start := time.Now()
	failed := run()
	took := time.Since(start).Round(100 * time.Millisecond)
	if failed != "" {
		t.Logf("run %d failed after %s: %s", i, took, failed)
		return []string{fmt.Sprintf("run %d: %s", i, failed)}
	}
	t.Logf("run %d exited 0 after %s", i, took)
	return nil
}

// sheetRun draws the building sheet in the demo tool's drawing child, the
// process that runs Ebitengine (item 230), on a display of its own.
func sheetRun(demo, root string, during func(*os.Process) (stop func())) string {
	xdir := filepath.Join(root, "x")
	if err := os.MkdirAll(xdir, 0o700); err != nil {
		return err.Error()
	}
	x, err := xvfb.Start(xdir, 1100, 760)
	if err != nil {
		return "no display: " + err.Error()
	}
	defer func() { _ = x.Close() }()
	cmd := exec.Command(demo, "sheet", "--out", filepath.Join(root, "sheet.png"))
	cmd.Env = append(append(os.Environ(), x.Env()...), "BOTROPOLIS_SHEET_DISPLAY=1")
	return exitOf(cmd, during)
}
