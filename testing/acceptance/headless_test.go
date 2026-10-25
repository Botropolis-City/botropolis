package acceptance_test

import (
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/testing/helpers"
)

// Item 126: under xvfb-run -a, cities started together picked the same
// display, and before the fix 19 of 40 such starts lost their X server.
func TestHeadlessCitiesStartedTogether(t *testing.T) {
	helpers.NeedXvfb(t)

	t.Run("when eight headless recordings start at the same moment", func(t *testing.T) {
		const n = 8
		runs := make([]stateRun, n)
		for i := range runs {
			runs[i] = newStateRun(t)
			withSession(t, runs[i].home)
		}
		failed := map[int]string{}
		var mu sync.Mutex
		var wg sync.WaitGroup
		for i, r := range runs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				cmd := exec.Command(botropolis, "city", "--socket", filepath.Join(r.tmp, "none.sock"), "--headless",
					"--record", filepath.Join(r.tmp, "frames"), "--seconds", "1", "--fps", "5")
				cmd.Env = r.env()
				if out, err := cmd.CombinedOutput(); err != nil {
					mu.Lock()
					failed[i] = err.Error() + ": " + string(out)
					mu.Unlock()
				}
			}()
		}
		wg.Wait()

		t.Run("it should finish every one without losing its X server", func(t *testing.T) {
			assert.Empty(t, failed)
		})
	})
}

// Item 251: a headless city killed outright never reached Xvfb's Close,
// so its Xvfb, and the city it had re-executed, outlived it for good.
func TestKilledHeadlessCity(t *testing.T) {
	helpers.NeedXvfb(t)

	t.Run("when a headless city is killed with SIGKILL while recording", func(t *testing.T) {
		r := newStateRun(t)
		withSession(t, r.home)
		cmd := exec.Command(botropolis, "city", "--socket", filepath.Join(r.tmp, "none.sock"), "--headless",
			"--record", filepath.Join(r.tmp, "frames"), "--seconds", "60", "--fps", "5")
		cmd.Env = r.env()
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			for _, pid := range append(running("Xvfb", r.tmp), cities(r.tmp)...) {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		})
		started := poll(30*time.Second, func() bool {
			return len(running("Xvfb", r.tmp)) > 0 && len(cities(r.tmp)) >= 2
		})
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if !started {
			t.Fatal("the city never had both its Xvfb and its child running")
		}
		poll(10*time.Second, func() bool {
			return len(running("Xvfb", r.tmp)) == 0 && len(cities(r.tmp)) == 0
		})

		t.Run("it should leave no Xvfb naming its state directory", func(t *testing.T) {
			assert.Empty(t, running("Xvfb", r.tmp))
		})

		t.Run("it should leave no city child naming its state directory", func(t *testing.T) {
			assert.Empty(t, cities(r.tmp))
		})
	})
}

// poll asks done every 20 ms until it holds or limit passes, and says
// whether it held.
func poll(limit time.Duration, done func() bool) bool {
	deadline := time.Now().Add(limit)
	for !done() {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
	return true
}
