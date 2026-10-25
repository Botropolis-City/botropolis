//go:build !linux

package xvfb

import "syscall"

func tie(*syscall.SysProcAttr) {}
