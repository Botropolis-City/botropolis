package demo

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// standIns is where a staged home lists the processes its live records
// point at, so Unstage can end them.
const standIns = "stand-ins.pids"

// Sleepers spawns a detached `sleep` per live record that outlives the
// stager, for a recording made after it exits. Each one dies on its own
// after hold, so a home that is never unstaged does not leave processes
// behind for good.
func Sleepers(dir string, hold time.Duration) Spawner {
	return func() (int, error) {
		cmd := exec.Command("sleep", strconv.Itoa(int(hold.Seconds())))
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := cmd.Start(); err != nil {
			return 0, err
		}
		pid := cmd.Process.Pid
		go func() { _ = cmd.Wait() }()
		started, ok := startTime(os.DirFS("/proc"), pid)
		if !ok {
			return 0, fmt.Errorf("stand-in %d has no start time", pid)
		}
		f, err := os.OpenFile(filepath.Join(dir, standIns), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return 0, err
		}
		defer func() { _ = f.Close() }()
		_, err = fmt.Fprintln(f, pid, started)
		return pid, err
	}
}

// Unstage ends a staged home's stand-ins. Each is listed with the time
// it started, and a pid is only signalled while the process holding it
// started then: one that outlived its hold may have been handed to an
// unrelated process since. The start time is set at fork, so it is there
// the moment Start returns; the process's name is not, and a stand-in
// can still carry its parent's name when Unstage reads it (item 124).
func Unstage(dir string) error {
	return unstage(dir, os.DirFS("/proc"), syscall.Kill)
}

func unstage(dir string, proc fs.FS, kill func(int, syscall.Signal) error) error {
	f, err := os.Open(filepath.Join(dir, standIns))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		var pid int
		var listed uint64
		if n, _ := fmt.Sscan(scan.Text(), &pid, &listed); n != 2 {
			continue
		}
		if started, ok := startTime(proc, pid); !ok || started != listed {
			continue
		}
		_ = kill(pid, syscall.SIGTERM)
	}
	if err := scan.Err(); err != nil {
		return err
	}
	return os.Remove(filepath.Join(dir, standIns))
}

// startTime is field 22 of the process's stat, in clock ticks since
// boot. Fields are counted from after the name, which can hold spaces
// and parentheses.
func startTime(proc fs.FS, pid int) (uint64, bool) {
	stat, err := fs.ReadFile(proc, strconv.Itoa(pid)+"/stat")
	if err != nil {
		return 0, false
	}
	shut := bytes.LastIndexByte(stat, ')')
	if shut < 0 {
		return 0, false
	}
	fields := strings.Fields(string(stat[shut+1:]))
	if len(fields) < 20 {
		return 0, false
	}
	started, err := strconv.ParseUint(fields[19], 10, 64)
	return started, err == nil
}
