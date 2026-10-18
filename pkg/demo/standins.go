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
		f, err := os.OpenFile(filepath.Join(dir, standIns), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return 0, err
		}
		defer func() { _ = f.Close() }()
		_, err = fmt.Fprintln(f, pid)
		return pid, err
	}
}

// Unstage ends a staged home's stand-ins. A pid is only signalled while
// it is still a sleep: one that outlived its hold may have been handed
// to an unrelated process since.
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
		pid, err := strconv.Atoi(scan.Text())
		if err != nil || !isSleep(proc, pid) {
			continue
		}
		_ = kill(pid, syscall.SIGTERM)
	}
	if err := scan.Err(); err != nil {
		return err
	}
	return os.Remove(filepath.Join(dir, standIns))
}

// isSleep reads the process's name, not its command line: exec.Cmd.Start
// returns before the kernel has laid out the new program's arguments, so
// a stand-in just spawned can have an empty command line while its name
// is already sleep (item 104).
func isSleep(proc fs.FS, pid int) bool {
	comm, err := fs.ReadFile(proc, strconv.Itoa(pid)+"/comm")
	if err != nil {
		return false
	}
	return string(bytes.TrimSpace(comm)) == "sleep"
}
