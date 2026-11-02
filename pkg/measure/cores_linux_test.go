package measure

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// Item 330: the same tick took 14.9 ms of thread CPU on a P-core, 21.3 on
// an E-core and 38.6 on a low-power core, so the stress measures pin to
// the fastest cores, found by each core's maximum frequency.
func TestCoreClasses(t *testing.T) {
	// sysfs lays out a cpu directory per core, each with its maximum
	// frequency in kHz, or none when it has no cpufreq.
	sysfs := func(t *testing.T, khz ...int) string {
		dir := t.TempDir()
		for cpu, f := range khz {
			freq := filepath.Join(dir, "cpu"+strconv.Itoa(cpu), "cpufreq")
			require.NoError(t, os.MkdirAll(freq, 0o755))
			if f > 0 {
				require.NoError(t, os.WriteFile(filepath.Join(freq, "cpuinfo_max_freq"), []byte(strconv.Itoa(f)+"\n"), 0o644))
			}
		}
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "cpufreq"), 0o755))
		return dir
	}

	for _, c := range []struct {
		scenario string
		khz      []int
		classes  [][]int
	}{
		{
			scenario: "when the cores run at three speeds, as on the reference machine",
			khz:      []int{5400000, 5400000, 5400000, 5400000, 4300000, 4300000, 4300000, 4300000, 4300000, 4300000, 4300000, 4300000, 2400000, 2400000},
			classes:  [][]int{{0, 1, 2, 3}, {4, 5, 6, 7, 8, 9, 10, 11}, {12, 13}},
		},
		{
			scenario: "when the fastest cores are not numbered first",
			khz:      []int{4300000, 5400000, 4300000, 5400000},
			classes:  [][]int{{1, 3}, {0, 2}},
		},
		{
			scenario: "when every core runs at one speed",
			khz:      []int{3000000, 3000000, 3000000},
			classes:  [][]int{{0, 1, 2}},
		},
		{
			scenario: "when one core has no maximum frequency",
			khz:      []int{3000000, 0, 3000000},
			classes:  [][]int{{0, 2}},
		},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			classes, err := CoreClasses(sysfs(t, c.khz...))
			require.NoError(t, err)

			t.Run("it should group them fastest first", func(t *testing.T) {
				assert.Equal(t, c.classes, classes)
			})
		})
	}

	t.Run("when no core has a maximum frequency", func(t *testing.T) {
		_, err := CoreClasses(sysfs(t, 0, 0))

		t.Run("it should say it cannot tell the cores apart", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}

func TestPinned(t *testing.T) {
	affinity := func(t *testing.T) unix.CPUSet {
		var set unix.CPUSet
		require.NoError(t, unix.SchedGetaffinity(0, &set))
		return set
	}

	for _, core := range []int{0, runtime.NumCPU() - 1} {
		t.Run("when work runs pinned to core "+strconv.Itoa(core), func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			before := affinity(t)
			var ended int
			require.NoError(t, Pinned([]int{core}, func() { ended = Thread(func() {}).EndedOn }))
			after := affinity(t)

			t.Run("it should run there", func(t *testing.T) {
				assert.Equal(t, core, ended)
			})

			t.Run("it should give the thread its cores back", func(t *testing.T) {
				assert.Equal(t, before, after)
			})
		})
	}

	t.Run("when it is given no cores", func(t *testing.T) {
		ran := false
		err := Pinned(nil, func() { ran = true })

		t.Run("it should refuse", func(t *testing.T) {
			assert.Error(t, err)
		})

		t.Run("it should run nothing", func(t *testing.T) {
			assert.False(t, ran)
		})
	})
}
