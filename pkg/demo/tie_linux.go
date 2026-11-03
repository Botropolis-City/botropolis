package demo

import (
	"os/exec"
	"syscall"
)

// tie has the kernel kill a shot's daemon when the filmer dies, so a
// crashed or interrupted film leaves no daemon behind.
func tie(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}

func tiedToParent(cmd *exec.Cmd) bool {
	return cmd.SysProcAttr != nil && cmd.SysProcAttr.Pdeathsig == syscall.SIGKILL
}
