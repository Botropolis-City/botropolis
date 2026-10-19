package acceptance_test

import (
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

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
