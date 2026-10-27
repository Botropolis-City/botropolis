package control

import (
	"os"
	"strconv"
	"syscall"
)

// keepFromChildren marks every descriptor above stderr close-on-exec, so
// a tool the city runs gets stdin, stdout and stderr and nothing else.
// Go opens its own descriptors that way; the window library's C side
// does not, and xclip once held three of the city's /dev/input devices
// (item 284).
func keepFromChildren() {
	if closeRangeOnExec() {
		return
	}
	sweepOnExec()
}

func sweepOnExec() {
	dir, err := os.Open(fdDir)
	if err != nil {
		return
	}
	defer func() { _ = dir.Close() }()
	names, _ := dir.Readdirnames(-1)
	for _, name := range names {
		if fd, err := strconv.Atoi(name); err == nil && fd > 2 {
			syscall.CloseOnExec(fd)
		}
	}
}
