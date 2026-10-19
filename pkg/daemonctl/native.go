package daemonctl

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"botropolis.city/botropolis/pkg/proto"
)

// Native is this machine: its service managers, its processes, and
// this build's daemon.
func Native() System {
	return System{
		GOOS: runtime.GOOS,
		UID:  os.Getuid(),
		Run: func(ctx context.Context, name string, args ...string) (string, error) {
			out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
			return string(out), err
		},
		PeerPID: PeerPID,
		Cgroup:  cgroup,
		Args:    args,
		Signal:  syscall.Kill,
		Alive:   alive,
		Start:   start,
		Binary:  binary,
		Answer:  proto.Ask,
		Sleep:   time.Sleep,
		Now:     time.Now,
	}
}

// PeerPID is the process listening at the other end of a unix socket.
func PeerPID(sock string) (int, error) {
	conn, err := net.DialTimeout("unix", sock, time.Second)
	if err != nil {
		return 0, err
	}
	defer func() { _ = conn.Close() }()
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, errors.New("not a unix socket")
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return 0, err
	}
	var pid int
	var peerErr error
	if err := raw.Control(func(fd uintptr) { pid, peerErr = peerPID(int(fd)) }); err != nil {
		return 0, err
	}
	return pid, peerErr
}

func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// start starts a daemon in a session of its own, with nothing of this
// process's to write to, so it outlives this process and never takes a
// broken pipe when this one exits. Its exit is reaped here while this
// process lives, so a daemon that dies is not left a zombie that still
// looks alive.
func start(path string, args []string) error {
	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// binary is the daemon beside this executable, which is the one this
// build shipped with, or failing that the one on the PATH.
func binary() (string, error) {
	if exe, err := os.Executable(); err == nil {
		beside := filepath.Join(filepath.Dir(exe), "botropolisd")
		if info, err := os.Stat(beside); err == nil && !info.IsDir() {
			return beside, nil
		}
	}
	return exec.LookPath("botropolisd")
}
