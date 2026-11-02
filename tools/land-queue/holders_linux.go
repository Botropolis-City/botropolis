package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

// dieWithMe has the kernel stop the command when land-queue dies, since the
// command no longer holds the lock itself (item 326). The signal follows the
// thread that started the child, so the thread stays locked to this goroutine.
func dieWithMe() *syscall.SysProcAttr {
	runtime.LockOSThread()
	return &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}

// holders names every other process with a descriptor open on path.
func holders(path string) []string {
	want, err := os.Stat(path)
	if err != nil {
		return nil
	}
	procs, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	me := os.Getpid()
	var found []int
	for _, p := range procs {
		pid, err := strconv.Atoi(p.Name())
		if err != nil || pid == me {
			continue
		}
		dir := filepath.Join("/proc", p.Name(), "fd")
		fds, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			if got, err := os.Stat(filepath.Join(dir, fd.Name())); err == nil && os.SameFile(want, got) {
				found = append(found, pid)
				break
			}
		}
	}
	sort.Ints(found)
	var names []string
	for _, pid := range found {
		names = append(names, fmt.Sprintf("pid %d (%s)", pid, commandLine(pid)))
	}
	return names
}

func commandLine(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil || len(data) == 0 {
		return "?"
	}
	line := strings.TrimSpace(strings.ReplaceAll(string(data), "\x00", " "))
	if len(line) > 100 {
		line = line[:100] + "…"
	}
	return line
}
