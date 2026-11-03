//go:build !linux

package demo

import "os/exec"

// tie does nothing where the kernel cannot kill a child with its parent;
// the shot still stops its daemon when it ends.
func tie(*exec.Cmd) {}

func tiedToParent(*exec.Cmd) bool { return false }
