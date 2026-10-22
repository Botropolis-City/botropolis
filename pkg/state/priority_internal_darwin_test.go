package state

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

// priorityChange is what a policy does to a thread. PRIO_DARWIN_BG and a
// thread I/O policy each strip the thread's QoS class, so it reads as
// unspecified after either. A baseAtMost of zero leaves the base
// priority where it was; CI's runners already hold threads at 20.
type priorityChange struct {
	darwinBG   bool
	stripsQoS  bool
	qos        uint32
	diskIO     int
	baseAtMost int32
}

func (c priorityChange) from(before ThreadPriority) priorityChange {
	if c.qos == 0 && !c.stripsQoS {
		c.qos = before.QoS
	}
	if c.diskIO == 0 {
		c.diskIO = before.DiskIO
	}
	if c.baseAtMost == 0 {
		c.baseAtMost = before.BasePriority
	}
	return c
}

var policyChanges = map[BackgroundPolicy]priorityChange{
	PolicyNone:               {},
	PolicyDarwinBG:           {darwinBG: true, stripsQoS: true, baseAtMost: 4},
	PolicyQoSUtility:         {qos: qosClassUtility, baseAtMost: 20},
	PolicyQoSBackground:      {qos: qosClassBackground, baseAtMost: 4},
	PolicyIOThrottle:         {diskIO: iopolThrottle, stripsQoS: true},
	PolicyDarwinBGIOThrottle: {darwinBG: true, stripsQoS: true, diskIO: iopolThrottle, baseAtMost: 4},
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

	t.Run("it should hold the thread's base priority at most where the policy puts it", func(t *testing.T) {
		assert.LessOrEqual(t, r.after.BasePriority, want.baseAtMost, "base priority %d before", r.before.BasePriority)
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

		t.Run("it should throttle the thread's disk I/O", func(t *testing.T) {
			assert.Equal(t, iopolThrottle, r.after.DiskIO)
		})

		t.Run("it should leave the thread out of PRIO_DARWIN_BG", func(t *testing.T) {
			assert.False(t, r.after.DarwinBG)
		})

		t.Run("it should leave the thread's base priority where it was", func(t *testing.T) {
			assert.Equal(t, r.before.BasePriority, r.after.BasePriority)
		})
	})
}
