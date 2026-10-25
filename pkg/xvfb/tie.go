package xvfb

import (
	"os/exec"
	"runtime"
	"sync"
	"syscall"
)

// startTied starts cmd so that, on Linux, the kernel kills it with
// SIGKILL when this process dies, however it dies (item 251). Elsewhere
// there is no such signal, and cmd outlives a process killed outright.
//
// The signal follows the thread that forked cmd, not the process, so cmd
// is started on a thread locked to one goroutine, which stays parked
// until release is called; otherwise another goroutine could later lock
// that thread and exit, and the thread's exit would kill cmd with it
// (golang/go#27505). Call release once cmd has exited.
func startTied(cmd *exec.Cmd) (release func(), err error) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	tie(cmd.SysProcAttr)
	started := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		err := cmd.Start()
		started <- err
		if err == nil {
			<-done
		}
	}()
	if err := <-started; err != nil {
		return nil, err
	}
	var once sync.Once
	return func() { once.Do(func() { close(done) }) }, nil
}
