package targets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunnable(t *testing.T) {
	tiers := map[string]Test{
		"TestA":    {Tier: Behaviour},
		"TestB":    {Tier: Behaviour},
		"TestCost": {Tier: Performance},
	}

	t.Run("when the binary lists exactly the package's Test functions", func(t *testing.T) {
		run, err := Runnable([]string{"TestB", "TestCost", "TestA"}, tiers)
		require.NoError(t, err)

		t.Run("it should run every one except the performance tier, sorted", func(t *testing.T) {
			assert.Equal(t, []string{"TestA", "TestB"}, run)
		})
	})

	t.Run("when the binary lists a test the package does not declare", func(t *testing.T) {
		_, err := Runnable([]string{"TestA", "TestB", "TestCost", "TestStray"}, tiers)

		t.Run("it should refuse, naming the test", func(t *testing.T) {
			assert.ErrorContains(t, err, "TestStray")
		})
	})

	t.Run("when the binary misses a test the package declares", func(t *testing.T) {
		_, err := Runnable([]string{"TestA", "TestCost"}, tiers)

		t.Run("it should refuse, naming the test", func(t *testing.T) {
			assert.ErrorContains(t, err, "TestB")
		})
	})
}

func TestOutsideTier(t *testing.T) {
	tiers := map[string]Test{
		"TestA":     {Tier: Behaviour},
		"TestB":     {Tier: Behaviour},
		"TestCost":  {Tier: Performance},
		"TestCost2": {Tier: Performance},
	}

	t.Run("when a selection holds behaviour and performance tests", func(t *testing.T) {
		run, performance := OutsideTier([]string{"TestCost", "TestB", "TestA"}, tiers)

		t.Run("it should run the behaviour tests, sorted", func(t *testing.T) {
			assert.Equal(t, []string{"TestA", "TestB"}, run)
		})

		t.Run("it should leave the performance tests to their tier", func(t *testing.T) {
			assert.Equal(t, []string{"TestCost"}, performance)
		})
	})

	t.Run("when a selection holds only performance tests", func(t *testing.T) {
		run, performance := OutsideTier([]string{"TestCost2", "TestCost"}, tiers)

		t.Run("it should run none", func(t *testing.T) {
			assert.Empty(t, run)
		})

		t.Run("it should leave them all to their tier", func(t *testing.T) {
			assert.Equal(t, []string{"TestCost", "TestCost2"}, performance)
		})
	})

	t.Run("when a selection names a test the package does not declare", func(t *testing.T) {
		run, _ := OutsideTier([]string{"TestStray"}, tiers)

		t.Run("it should run it, so the binary's own check refuses it", func(t *testing.T) {
			assert.Equal(t, []string{"TestStray"}, run)
		})
	})
}

func exitStatus(t *testing.T, code int) error {
	t.Helper()
	err := exec.Command("sh", "-c", fmt.Sprintf("exit %d", code)).Run()
	require.Error(t, err)
	return err
}

func TestReport(t *testing.T) {
	for _, c := range []struct {
		when    string
		output  string
		exit    int
		outcome Outcome
		reason  string
	}{
		{"when the child prints --- PASS for the test and exits 0", "=== RUN   TestA\n--- PASS: TestA (0.10s)\nPASS\n", 0, Passed, ""},
		{"when the child prints --- SKIP for the test and exits 0", "=== RUN   TestA\n--- SKIP: TestA (0.00s)\nPASS\n", 0, Skipped, ""},
		{"when the child prints --- FAIL for the test", "=== RUN   TestA\n--- FAIL: TestA (0.10s)\nFAIL\n", 1, Failed, "failed"},
		{"when the child exits non-zero with no FAIL line", "=== RUN   TestA\npanic: boom\n", 2, Failed, "exited with exit status 2"},
		{"when the child exits 0 but never reports the test", "testing: warning: no tests to run\nPASS\n", 0, Failed, "never reported"},
		{"when only a subtest of another name passes", "=== RUN   TestAB\n--- PASS: TestAB (0.10s)\nPASS\n", 0, Failed, "never reported"},
	} {
		t.Run(c.when, func(t *testing.T) {
			var err error
			if c.exit != 0 {
				err = exitStatus(t, c.exit)
			}
			r := Report("TestA", []byte(c.output), err, false)

			t.Run("it should read the outcome", func(t *testing.T) {
				assert.Equal(t, c.outcome, r.Outcome)
			})

			t.Run("it should say why", func(t *testing.T) {
				assert.Contains(t, r.Reason, c.reason)
			})
		})
	}

	t.Run("when the child was killed for its timeout", func(t *testing.T) {
		r := Report("TestA", []byte("=== RUN   TestA\n"), exitStatus(t, 1), true)

		t.Run("it should fail it as timed out", func(t *testing.T) {
			assert.Equal(t, Result{Test: "TestA", Outcome: Failed, Reason: "timed out", Output: []byte("=== RUN   TestA\n")}, r)
		})
	})
}

// fakeChild answers for each test as told: "pass", "fail", or "hang", which
// waits for its context and records that it was cancelled.
type fakeChild struct {
	answers   map[string]string
	running   atomic.Int32
	widest    atomic.Int32
	mu        sync.Mutex
	ran       []string
	cancelled []string
}

func (f *fakeChild) run(ctx context.Context, test string) ([]byte, error) {
	now := f.running.Add(1)
	defer f.running.Add(-1)
	for {
		widest := f.widest.Load()
		if now <= widest || f.widest.CompareAndSwap(widest, now) {
			break
		}
	}
	f.mu.Lock()
	f.ran = append(f.ran, test)
	f.mu.Unlock()
	time.Sleep(5 * time.Millisecond)
	switch f.answers[test] {
	case "fail":
		return []byte("=== RUN   " + test + "\n    a_test.go:9: wrong answer\n--- FAIL: " + test + " (0.00s)\nFAIL\n"), errors.New("exit status 1")
	case "hang":
		<-ctx.Done()
		f.mu.Lock()
		f.cancelled = append(f.cancelled, test)
		f.mu.Unlock()
		return []byte("=== RUN   " + test + "\n"), ctx.Err()
	}
	return []byte("=== RUN   " + test + "\n--- PASS: " + test + " (0.00s)\nPASS\n"), nil
}

func TestRunPool(t *testing.T) {
	tests := []string{"TestA", "TestB", "TestC", "TestD", "TestE", "TestF"}

	t.Run("when every test passes", func(t *testing.T) {
		var out bytes.Buffer
		code := RunPool(tests, 3, time.Minute, (&fakeChild{}).run, &out)

		t.Run("it should exit 0", func(t *testing.T) {
			assert.Equal(t, 0, code)
		})

		t.Run("it should say how many ran", func(t *testing.T) {
			assert.Contains(t, out.String(), "6 tests: 6 passed")
		})
	})

	t.Run("when one test fails", func(t *testing.T) {
		child := &fakeChild{answers: map[string]string{"TestC": "fail"}}
		var out bytes.Buffer
		code := RunPool(tests, 3, time.Minute, child.run, &out)

		t.Run("it should exit non-zero", func(t *testing.T) {
			assert.NotEqual(t, 0, code)
		})

		t.Run("it should name it", func(t *testing.T) {
			assert.Contains(t, out.String(), "FAIL TestC")
		})

		t.Run("it should print its output", func(t *testing.T) {
			assert.Contains(t, out.String(), "wrong answer")
		})

		t.Run("it should still run the rest", func(t *testing.T) {
			assert.ElementsMatch(t, tests, child.ran)
		})
	})

	t.Run("when a child outlives its timeout", func(t *testing.T) {
		child := &fakeChild{answers: map[string]string{"TestB": "hang"}}
		var out bytes.Buffer
		code := RunPool(tests, 3, 50*time.Millisecond, child.run, &out)

		t.Run("it should kill it", func(t *testing.T) {
			assert.Equal(t, []string{"TestB"}, child.cancelled)
		})

		t.Run("it should exit non-zero", func(t *testing.T) {
			assert.NotEqual(t, 0, code)
		})

		t.Run("it should name it as timed out", func(t *testing.T) {
			assert.Contains(t, out.String(), "FAIL TestB (timed out)")
		})
	})

	t.Run("when there are no tests to run", func(t *testing.T) {
		var out bytes.Buffer
		code := RunPool(nil, 3, time.Minute, (&fakeChild{}).run, &out)

		t.Run("it should exit non-zero", func(t *testing.T) {
			assert.NotEqual(t, 0, code)
		})

		t.Run("it should say it ran none", func(t *testing.T) {
			assert.Contains(t, out.String(), "ran no tests")
		})
	})

	t.Run("when the pool is 2 wide", func(t *testing.T) {
		child := &fakeChild{}
		RunPool(tests, 2, time.Minute, child.run, &bytes.Buffer{})

		t.Run("it should never run more than 2 at once", func(t *testing.T) {
			assert.LessOrEqual(t, child.widest.Load(), int32(2))
		})
	})
}

func TestDefaultParallel(t *testing.T) {
	for _, c := range []struct{ cores, want int }{{1, 1}, {2, 1}, {4, 2}, {14, 6}, {64, 6}} {
		t.Run(fmt.Sprintf("when the machine has %d cores", c.cores), func(t *testing.T) {
			t.Run(fmt.Sprintf("it should run %d at once", c.want), func(t *testing.T) {
				assert.Equal(t, c.want, DefaultParallel(c.cores))
			})
		})
	}
}
