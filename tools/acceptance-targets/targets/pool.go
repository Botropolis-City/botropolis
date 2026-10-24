package targets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
)

// Runnable is the tests the binary lists, less the performance tier, once
// the list agrees exactly with the Test functions the package declares, so
// that no test can be left out of a run without the run saying so.
func Runnable(listed []string, tiers map[string]Test) ([]string, error) {
	seen := map[string]bool{}
	var stray, run []string
	for _, name := range listed {
		seen[name] = true
		test, ok := tiers[name]
		switch {
		case !ok:
			stray = append(stray, name)
		case test.Tier != Performance:
			run = append(run, name)
		}
	}
	var missing []string
	for name := range tiers {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(stray)
	sort.Strings(missing)
	if len(stray)+len(missing) > 0 {
		return nil, fmt.Errorf("the test binary and the package disagree: listed but not declared %v, declared but not listed %v", stray, missing)
	}
	sort.Strings(run)
	return run, nil
}

type Outcome int

const (
	Passed Outcome = iota
	Skipped
	Failed
)

// Result is one test's run in a process of its own.
type Result struct {
	Test    string
	Outcome Outcome
	Reason  string
	Output  []byte
}

// Report reads one child's -test.v output and how it exited. A child that
// exits 0 must still say it passed or skipped the test it was asked for.
func Report(test string, output []byte, exitErr error, timedOut bool) Result {
	r := Result{Test: test, Outcome: Failed, Output: output}
	var passed, skipped, failed bool
	for _, line := range strings.Split(string(output), "\n") {
		switch {
		case strings.HasPrefix(line, "--- PASS: "+test+" "):
			passed = true
		case strings.HasPrefix(line, "--- SKIP: "+test+" "):
			skipped = true
		case strings.HasPrefix(line, "--- FAIL: "+test+" "):
			failed = true
		}
	}
	switch {
	case timedOut:
		r.Reason = "timed out"
	case failed:
		r.Reason = "failed"
	case exitErr != nil:
		r.Reason = "exited with " + exitErr.Error()
	case passed:
		r.Outcome = Passed
	case skipped:
		r.Outcome = Skipped
	default:
		r.Reason = "never reported " + test
	}
	return r
}

// Child runs one test in a process of its own and returns what it printed
// and how it exited. It must stop when ctx is done.
type Child func(ctx context.Context, test string) ([]byte, error)

// RunPool runs every test through child, parallel at once, each within
// timeout, and reports to w: each failure by name with its output, then a
// count. It returns non-zero when anything failed or nothing ran.
func RunPool(tests []string, parallel int, timeout time.Duration, child Child, w io.Writer) int {
	if len(tests) == 0 {
		fmt.Fprintln(w, "ran no tests")
		return 1
	}
	start := time.Now()
	results := make([]Result, len(tests))
	work := make(chan int)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range max(parallel, 1) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				began := time.Now()
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				out, err := child(ctx, tests[i])
				timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
				cancel()
				r := Report(tests[i], out, err, timedOut)
				results[i] = r
				took := time.Since(began).Round(100 * time.Millisecond)
				mu.Lock()
				switch r.Outcome {
				case Failed:
					fmt.Fprintf(w, "FAIL %s (%s) after %s\n%s\n", r.Test, r.Reason, took, bytes.TrimRight(r.Output, "\n"))
				case Skipped:
					fmt.Fprintf(w, "skip %s (%s)\n", r.Test, took)
				default:
					fmt.Fprintf(w, "ok   %s (%s)\n", r.Test, took)
				}
				mu.Unlock()
			}
		}()
	}
	for i := range tests {
		work <- i
	}
	close(work)
	wg.Wait()
	count := map[Outcome]int{}
	for _, r := range results {
		count[r.Outcome]++
	}
	fmt.Fprintf(w, "%d tests: %d passed, %d skipped, %d failed, %d at a time, in %s\n",
		len(tests), count[Passed], count[Skipped], count[Failed], max(parallel, 1), time.Since(start).Round(time.Second))
	if count[Failed] > 0 {
		return 1
	}
	return 0
}

// DefaultParallel is how many tests run at once: half the cores, at least
// one and at most six, since several sessions share a machine and each runs
// its gate inside the landing lock (item 186).
func DefaultParallel(cores int) int {
	return min(max(cores/2, 1), 6)
}
