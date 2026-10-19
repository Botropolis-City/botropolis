package sighold

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// held is every asynchronous standard signal. The synchronous ones, which
// the kernel raises on the faulting thread itself, are left alone, and so
// are the real-time signals, which a C library may need on every thread.
var held = []unix.Signal{
	unix.SIGHUP, unix.SIGINT, unix.SIGQUIT, unix.SIGPIPE, unix.SIGALRM,
	unix.SIGTERM, unix.SIGUSR1, unix.SIGUSR2, unix.SIGCHLD, unix.SIGCONT,
	unix.SIGTSTP, unix.SIGTTIN, unix.SIGTTOU, unix.SIGURG, unix.SIGXCPU,
	unix.SIGXFSZ, unix.SIGVTALRM, unix.SIGPROF, unix.SIGWINCH, unix.SIGIO,
	unix.SIGPWR, unix.SIGSTKFLT,
}

func add(set *unix.Sigset_t, sig unix.Signal) {
	width := uint(unsafe.Sizeof(set.Val[0])) * 8
	bit := uint(sig) - 1
	set.Val[bit/width] |= 1 << (bit % width)
}

// Hold blocks the held signals on the calling thread, which must be locked
// to its goroutine, and returns a function that restores the mask it found.
func Hold() (release func()) {
	var set, found unix.Sigset_t
	for _, sig := range held {
		add(&set, sig)
	}
	if err := unix.PthreadSigmask(unix.SIG_BLOCK, &set, &found); err != nil {
		return func() {}
	}
	return func() { _ = unix.PthreadSigmask(unix.SIG_SETMASK, &found, nil) }
}
