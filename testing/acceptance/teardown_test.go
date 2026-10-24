package acceptance_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/xvfb"
	"botropolis.city/botropolis/testing/helpers"
)

const teardownRuns = 120

// screenshotRun opens a --home city that takes one screenshot and exits,
// each on a display of its own (item 126), and returns how it exited.
// during, when not nil, starts with the city's process and is stopped
// once the city has exited.
func screenshotRun(root string, during func(*os.Process) (stop func())) string {
	r := stateRun{home: filepath.Join(root, "home"), realState: filepath.Join(root, "state"), tmp: filepath.Join(root, "tmp")}
	other, xdir := filepath.Join(root, "other"), filepath.Join(root, "x")
	for _, dir := range []string{r.home, r.realState, r.tmp, other, xdir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err.Error()
		}
	}
	x, err := xvfb.Start(xdir, 1100, 760)
	if err != nil {
		return "no display: " + err.Error()
	}
	defer func() { _ = x.Close() }()
	cmd := exec.Command(botropolis, "city", "--socket", filepath.Join(r.tmp, "none.sock"),
		"--home", other, "--screenshot", filepath.Join(r.tmp, "shot.png"))
	cmd.Env = append(r.env(), x.Env()...)
	return exitOf(cmd, during)
}

// exitOf runs cmd to its exit, with during beside it, and says how it
// exited: "" for status 0.
func exitOf(cmd *exec.Cmd, during func(*os.Process) (stop func())) string {
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		return err.Error()
	}
	if during != nil {
		stop := during(cmd.Process)
		defer stop()
	}
	err := cmd.Wait()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return ""
	case errors.As(err, &exit):
		return fmt.Sprintf("%s: %s", exit.ProcessState, lastLine(out.Bytes()))
	default:
		return err.Error()
	}
}

func lastLine(out []byte) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return lines[len(lines)-1]
}

// Item 125. The soak is opt-in, BOTROPOLIS_SOAK=1 or make soak-teardown:
// at 4 crashes in 60 unfixed, 120 clean runs tell a fix from luck
// (p ≈ 0.0003), and they take minutes.
func TestTeardownSoak(t *testing.T) {
	if os.Getenv("BOTROPOLIS_SOAK") == "" {
		t.Skip("set BOTROPOLIS_SOAK=1, or run make soak-teardown")
	}
	helpers.NeedXvfb(t)

	t.Run(fmt.Sprintf("when %d --home screenshot runs exit at load, each on its own display", teardownRuns), func(t *testing.T) {
		root := t.TempDir()
		var mu sync.Mutex
		var failures []string
		var wg sync.WaitGroup
		slots := make(chan struct{}, runtime.NumCPU())
		for i := range teardownRuns {
			wg.Add(1)
			slots <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-slots }()
				if failed := screenshotRun(filepath.Join(root, fmt.Sprint(i)), nil); failed != "" {
					mu.Lock()
					failures = append(failures, fmt.Sprintf("run %d: %s", i, failed))
					mu.Unlock()
				}
			}()
		}
		wg.Wait()

		t.Run("it should exit 0 every time", func(t *testing.T) {
			assert.Empty(t, failures)
		})
	})
}
