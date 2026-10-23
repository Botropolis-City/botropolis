package measure

import (
	"io"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sink uint64

// work is a fixed piece of work: the same instructions every time.
func work() {
	x := uint64(1)
	for i := 0; i < 20_000_000; i++ {
		x = x*6364136223846793005 + 1442695040888963407
	}
	sink += x
}

// counting turns instruction counting on for one test, and skips it
// where the machine has no counters to read, saying why.
func counting(t *testing.T) {
	t.Helper()
	if err := Available(); err != nil {
		t.Skipf("no hardware instruction counters: %v", err)
	}
	was := CountInstructions.Swap(true)
	t.Cleanup(func() { CountInstructions.Store(was) })
}

// Item 209: CPU time on a hybrid laptop chip moves with its clocks and
// with which kind of core runs the work, so the cost gates count the
// instructions it retires, which neither moves.
func TestInstructions(t *testing.T) {
	t.Run("when instructions are counted", func(t *testing.T) {
		counting(t)

		t.Run("and a fixed piece of work is counted on a quiet machine, and again beside 20 spinning goroutines", func(t *testing.T) {
			quiet := Thread(work)
			stop := make(chan struct{})
			for range 20 {
				go func() {
					for {
						select {
						case <-stop:
							return
						default:
							runtime.Gosched()
						}
					}
				}()
			}
			busy := Thread(work)
			close(stop)

			t.Run("it should count both", func(t *testing.T) {
				assert.True(t, quiet.Counted && busy.Counted)
			})

			t.Run("it should count within 5% of the quiet count beside them", func(t *testing.T) {
				assert.InEpsilon(t, float64(quiet.Instructions), float64(busy.Instructions), 0.05)
			})
		})

		t.Run("and the work copies 256 MiB out of the kernel", func(t *testing.T) {
			zero, err := os.Open("/dev/zero")
			require.NoError(t, err)
			t.Cleanup(func() { _ = zero.Close() })
			buf := make([]byte, 1<<20)
			copied := Thread(func() {
				for range 256 {
					_, err := io.ReadFull(zero, buf)
					require.NoError(t, err)
				}
			})

			t.Run("it should count the thread's time in the kernel", func(t *testing.T) {
				assert.Greater(t, copied.Kernel, time.Duration(0))
			})
		})

		t.Run("and the work sleeps for 50 ms", func(t *testing.T) {
			slept := Thread(func() { time.Sleep(50 * time.Millisecond) })

			t.Run("it should count under a million instructions", func(t *testing.T) {
				assert.Less(t, slept.Instructions, uint64(1_000_000))
			})
		})

		t.Run("and a child process loops 50 million times once it is being counted", func(t *testing.T) {
			child := exec.Command("sh", "-c", `read go; awk 'BEGIN { for (i = 0; i < 50000000; i++) n += i; print "done" }'; sleep 600`)
			start, err := child.StdinPipe()
			require.NoError(t, err)
			out, err := child.StdoutPipe()
			require.NoError(t, err)
			require.NoError(t, child.Start())
			t.Cleanup(func() { _ = child.Process.Kill(); _ = child.Wait() })
			p, err := Process(child.Process.Pid)
			require.NoError(t, err)
			t.Cleanup(func() { _ = p.Close() })
			_, err = start.Write([]byte("go\n"))
			require.NoError(t, err)
			done := make([]byte, 5)
			_, err = io.ReadFull(out, done)
			require.NoError(t, err)
			spent := p.Instructions()

			t.Run("it should count the loop's work in the processes it starts, at least 20 ms of a core", func(t *testing.T) {
				assert.Greater(t, float64(spent), 0.02*CoreRate)
			})
		})
	})

	t.Run("when instructions are not counted", func(t *testing.T) {
		was := CountInstructions.Swap(false)
		t.Cleanup(func() { CountInstructions.Store(was) })
		spun := Thread(work)

		t.Run("it should say so", func(t *testing.T) {
			assert.False(t, spun.Counted)
		})

		t.Run("it should give its CPU as its cost", func(t *testing.T) {
			assert.Equal(t, spun.CPU, spun.Core())
		})
	})
}

func TestCoreRate(t *testing.T) {
	// Aria, 2026-10-23: a core is a core as bursty work gets it.
	t.Run("when the rates measured on sustained and on bursty work are compared", func(t *testing.T) {
		t.Run("it should take the bursty rate as a core", func(t *testing.T) {
			assert.Equal(t, BurstCoreRate, CoreRate)
		})

		t.Run("it should have measured bursts slower than either kind of core sustains", func(t *testing.T) {
			assert.Less(t, BurstCoreRate, min(PerformanceCoreRate, EfficiencyCoreRate))
		})
	})

	t.Run("when a counted cost is CoreRate instructions", func(t *testing.T) {
		c := Cost{Instructions: uint64(CoreRate), Counted: true}

		t.Run("it should cost a second of a core", func(t *testing.T) {
			assert.Equal(t, time.Second, c.Core())
		})

		// The counters see user space only, and the budget is all of the
		// CPU, so the kernel's time is added as it was spent.
		t.Run("and 10 ms in the kernel", func(t *testing.T) {
			c.Kernel = 10 * time.Millisecond

			t.Run("it should cost a second of a core and the 10 ms", func(t *testing.T) {
				assert.Equal(t, time.Second+10*time.Millisecond, c.Core())
			})
		})
	})
}

func TestTotal(t *testing.T) {
	t.Run("when two counted costs are added", func(t *testing.T) {
		var total Total
		total.Add(Cost{CPU: time.Millisecond, Instructions: 10, Counted: true})
		total.Add(Cost{CPU: 2 * time.Millisecond, Instructions: 20, Counted: true})

		t.Run("it should sum them, counted", func(t *testing.T) {
			assert.Equal(t, Cost{CPU: 3 * time.Millisecond, Instructions: 30, Counted: true}, total.Cost())
		})

		t.Run("and one that was not counted", func(t *testing.T) {
			total.Add(Cost{CPU: time.Millisecond})

			t.Run("it should say the total was not counted", func(t *testing.T) {
				assert.False(t, total.Cost().Counted)
			})
		})
	})
}
