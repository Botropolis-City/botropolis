package xvfb

import "syscall"

func tie(attr *syscall.SysProcAttr) { attr.Pdeathsig = syscall.SIGKILL }
