package state

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

type priorityChange struct {
	darwinBG  bool
	qos       uint32
	diskIO    int
	lowersCPU bool
}

func (c priorityChange) from(before ThreadPriority) priorityChange {
	if c.qos == 0 {
		c.qos = before.QoS
	}
	if c.diskIO == 0 {
		c.diskIO = before.DiskIO
	}
	return c
}

var policyChanges = map[BackgroundPolicy]priorityChange{
	PolicyNone:               {},
	PolicyDarwinBG:           {darwinBG: true, lowersCPU: true},
	PolicyQoSUtility:         {qos: qosClassUtility, lowersCPU: true},
	PolicyQoSBackground:      {qos: qosClassBackground, lowersCPU: true},
	PolicyIOThrottle:         {diskIO: iopolThrottle},
	PolicyDarwinBGIOThrottle: {darwinBG: true, diskIO: iopolThrottle, lowersCPU: true},
}

type policyRun struct {
	before, after ThreadPriority
	own, ownAfter ThreadPriority
}

// onFreshThread gives a thread of its own background priority under
// the policy choose picks, and reads it before and after, and the
// caller's thread before and after.
// The fresh thread is thrown away afterwards, so no other test inherits
// what it did to it.
func onFreshThread(t *testing.T, choose func()) policyRun {
	t.Helper()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	own := ThreadPolicy()
	choose()
	got := make(chan policyRun)
	go func() {
		runtime.LockOSThread()
		var r policyRun
		r.before = ThreadPolicy()
		backgroundPriority()
		r.after = ThreadPolicy()
		got <- r
	}()
	r := <-got
	r.own, r.ownAfter = own, ThreadPolicy()
	return r
}

func assertChange(t *testing.T, want priorityChange, r policyRun) {
	t.Run("it should set the thread's PRIO_DARWIN_BG flag as the policy says", func(t *testing.T) {
		assert.Equal(t, want.darwinBG, r.after.DarwinBG)
	})

	t.Run("it should set the thread's QoS class as the policy says", func(t *testing.T) {
		assert.Equal(t, want.qos, r.after.QoS)
	})

	t.Run("it should set the thread's disk I/O policy as the policy says", func(t *testing.T) {
		assert.Equal(t, want.diskIO, r.after.DiskIO)
	})

	t.Run("it should lower the thread's base priority only if the policy lowers its CPU priority", func(t *testing.T) {
		assert.Equal(t, want.lowersCPU, r.after.BasePriority < r.before.BasePriority, "base priority %d before, %d after", r.before.BasePriority, r.after.BasePriority)
	})

	t.Run("it should leave the process's other threads at their QoS class", func(t *testing.T) {
		assert.Equal(t, r.own.QoS, r.ownAfter.QoS)
	})
}

// Item 200: on macOS the pass ran at the process's priority. Each policy
// item 200 measures has to change the pass's thread as it says, and
// nothing else, or the measurement is of something else.
func TestBackgroundPriority(t *testing.T) {
	for _, policy := range BackgroundPolicies {
		t.Run(fmt.Sprintf("when a thread is given background priority under %s", policy), func(t *testing.T) {
			t.Cleanup(func() { SetBackgroundPolicy(shippedPolicy) })
			r := onFreshThread(t, func() { SetBackgroundPolicy(policy) })
			assertChange(t, policyChanges[policy].from(r.before), r)
		})
	}

	t.Run("when no policy has been chosen", func(t *testing.T) {
		r := onFreshThread(t, func() {})
		t.Logf("the shipped policy is %s", shippedPolicy)
		assertChange(t, policyChanges[shippedPolicy].from(r.before), r)
	})
}
