//go:build linux

package sighold

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func threadMask() unix.Sigset_t {
	var mask unix.Sigset_t
	_ = unix.PthreadSigmask(unix.SIG_BLOCK, nil, &mask)
	return mask
}

func blocked(mask unix.Sigset_t, sig unix.Signal) bool {
	var only unix.Sigset_t
	add(&only, sig)
	for i := range only.Val {
		if only.Val[i] != 0 {
			return mask.Val[i]&only.Val[i] != 0
		}
	}
	return false
}

func TestHold(t *testing.T) {
	t.Run("when the loop's thread holds signals", func(t *testing.T) {
		var found, holding, released unix.Sigset_t
		done := make(chan struct{})
		go func() {
			defer close(done)
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			found = threadMask()
			release := Hold()
			holding = threadMask()
			release()
			released = threadMask()
		}()
		<-done

		for _, sig := range []unix.Signal{unix.SIGURG, unix.SIGINT, unix.SIGTERM, unix.SIGCHLD, unix.SIGWINCH} {
			t.Run("it should block "+unix.SignalName(sig)+" on the calling thread", func(t *testing.T) {
				assert.True(t, blocked(holding, sig))
			})
		}

		t.Run("and the synchronous signals", func(t *testing.T) {
			for _, sig := range []unix.Signal{unix.SIGSEGV, unix.SIGBUS, unix.SIGFPE, unix.SIGILL, unix.SIGTRAP, unix.SIGSYS} {
				t.Run("it should leave "+unix.SignalName(sig)+" unblocked", func(t *testing.T) {
					assert.False(t, blocked(holding, sig))
				})
			}
		})

		t.Run("and it is released", func(t *testing.T) {
			t.Run("it should restore the mask it found", func(t *testing.T) {
				assert.Equal(t, found, released)
			})
		})
	})
}
