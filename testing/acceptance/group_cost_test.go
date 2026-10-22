//go:build linux

package acceptance_test

import (
	"fmt"
	"runtime"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

func processCPU(t *testing.T) time.Duration {
	t.Helper()
	var u syscall.Rusage
	require.NoError(t, syscall.Getrusage(syscall.RUSAGE_SELF, &u))
	return time.Duration(u.Utime.Nano() + u.Stime.Nano())
}

// TestGroupingCostPerPrompt is item 158's measure: every prompt in any
// session appends to history.jsonl, and the daemon loads again.
//
// The limit comes from the daemon's budget (performance/budgets.md in
// the meta repository): at most 2% of one core, mean, with sessions
// working. Taking one prompt every 5 s across the machine, 2% of a core
// is 100 ms of CPU per prompt; half of that is kept for the rest of the
// load, so grouping after a prompt, with the load around it, gets 50 ms.
// Before item 158 one detection alone took 258 ms with 100 sessions.
//
// It is process CPU, not wall time, so a busy machine slows it without
// failing it, and the median of five prompts, so one collection landing
// in one window does not decide it.
func TestGroupingCostPerPrompt(t *testing.T) {
	const (
		sessions = 100
		limit    = 50 * time.Millisecond
	)

	t.Run("when 100 sessions are on the map, and one sends a prompt", func(t *testing.T) {
		now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
		home := helpers.NewHome(t)
		history := helpers.SyntheticHistory(158, sessions, 40, now.Add(-20*24*time.Hour))
		started := map[string]bool{}
		for _, p := range history {
			if !started[p.Session] {
				started[p.Session] = true
				ts := p.At.Format("2006-01-02T15:04:05.000Z")
				home.Transcript(p.Session, p.Dir, helpers.UserPrompt(p.Session, p.Dir, ts),
					helpers.AssistantReply(p.Session, "msg_"+p.Session[len(p.Session)-6:], ts))
			}
			home.Prompt(p.Session, p.Dir, p.Text, p.At)
		}
		loader := state.NewLoader(home.Path, state.Probes{Alive: func(int) bool { return false }, Attached: func(string) bool { return false }})
		snapshot, err := loader.Load(now)
		require.NoError(t, err)
		require.Len(t, snapshot.Sessions, sessions)

		var costs []time.Duration
		for i := 0; i < 5; i++ {
			p := history[len(history)-1-i*7]
			home.Prompt(p.Session, p.Dir, fmt.Sprintf("please zzqprompt%d", i), now.Add(time.Duration(i)*time.Second))
			runtime.GC()
			before := processCPU(t)
			_, err := loader.Load(now.Add(time.Duration(i+1) * time.Second))
			require.NoError(t, err)
			costs = append(costs, processCPU(t)-before)
		}
		slices.Sort(costs)
		t.Logf("CPU per prompt, sorted: %v", costs)

		t.Run("it should spend under 50 ms of CPU on the next load", func(t *testing.T) {
			assert.Less(t, costs[len(costs)/2], limit, "costs %v", costs)
		})
	})
}
