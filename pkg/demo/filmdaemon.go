package demo

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// A shot whose city packs sessions up, as closing a group or the quiet
// sweep does, needs a daemon: the ledger of what is packed is its. A
// city pointed at a home with --home writes nothing, by design, so such
// a shot runs a daemon of its own and a city that is not pointed
// anywhere, both with HOME and the XDG directories in the shot's scratch
// directory, which makes the staged home their own (item 333). Every
// path they are given must resolve inside that directory, or the daemon
// does not start; and it dies with the filmer.

// daemonWait is how long a shot waits for its daemon's socket.
const daemonWait = 20 * time.Second

func daemonSocket(work string) string {
	return filepath.Join(work, "run", "botropolis.sock")
}

// withDaemon points the city at the shot's daemon, and at no home: HOME
// is the scratch home, so the city takes it for its own.
func withDaemon(sock string) cityOption {
	return func(args *[]string) {
		if at := slices.Index(*args, "--home"); at >= 0 {
			*args = slices.Delete(*args, at, at+2)
		}
		if at := slices.Index(*args, "--socket"); at >= 0 {
			(*args)[at+1] = sock
		}
	}
}

// filmEnv is the environment every process of a shot runs in: its
// scratch home and XDG directories, its clock, and nothing else of the
// machine's but how to find programs and speak.
func filmEnv(work, tz string) []string {
	env := []string{
		"HOME=" + filepath.Join(work, "home"),
		"XDG_STATE_HOME=" + filepath.Join(work, "state"),
		"XDG_CONFIG_HOME=" + filepath.Join(work, "config"),
		"XDG_CACHE_HOME=" + filepath.Join(work, "cache"),
		"XDG_RUNTIME_DIR=" + filepath.Join(work, "run"),
		"TZ=" + tz,
	}
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		switch key {
		case "PATH", "LANG", "LC_ALL", "TMPDIR":
			env = append(env, kv)
		}
	}
	return env
}

// contained refuses any home, XDG directory in env, or path that does
// not resolve inside work, following links and ".." as the kernel would.
func contained(work string, env []string, paths ...string) error {
	root, err := resolved(work)
	if err != nil {
		return err
	}
	check := slices.Clone(paths)
	for _, kv := range env {
		key, value, _ := strings.Cut(kv, "=")
		if key == "HOME" || strings.HasPrefix(key, "XDG_") {
			check = append(check, value)
		}
	}
	var errs []error
	for _, p := range check {
		real, err := resolved(p)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if real != root && !strings.HasPrefix(real, root+string(filepath.Separator)) {
			errs = append(errs, fmt.Errorf("%s is outside the shot's scratch directory %s", p, work))
		}
	}
	return errors.Join(errs...)
}

// resolved is p made absolute with every link in its existing part
// followed; the part that does not exist yet is joined on as written.
func resolved(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	rest := ""
	for dir := abs; ; dir = filepath.Dir(dir) {
		if real, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(real, rest), nil
		}
		if dir == filepath.Dir(dir) {
			return abs, nil
		}
		rest = filepath.Join(filepath.Base(dir), rest)
	}
}

// daemonPath is the daemon binary: as given, or beside the city's.
func (f Filmer) daemonPath() string {
	if f.Daemon != "" {
		return f.Daemon
	}
	return filepath.Join(filepath.Dir(f.Botropolis), "botropolisd")
}

func (f Filmer) daemonCommand(work, tz, sock string) *exec.Cmd {
	cmd := exec.Command(f.daemonPath(), "--socket", sock)
	cmd.Env = filmEnv(work, tz)
	tie(cmd)
	return cmd
}

// startDaemon starts the shot's daemon and waits for its socket; stop
// ends it.
func (f Filmer) startDaemon(work, tz string) (sock string, stop func(), err error) {
	sock = daemonSocket(work)
	env := filmEnv(work, tz)
	if err := contained(work, env, sock); err != nil {
		return "", nil, fmt.Errorf("not starting the shot's daemon: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(sock), 0o700); err != nil {
		return "", nil, err
	}
	cmd := f.daemonCommand(work, tz, sock)
	cmd.Stdout, cmd.Stderr = f.Log, f.Log
	if err := cmd.Start(); err != nil {
		return "", nil, err
	}
	stop = func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	deadline := time.Now().Add(daemonWait)
	for time.Now().Before(deadline) {
		if conn, err := net.Dial("unix", sock); err == nil {
			_ = conn.Close()
			return sock, stop, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	stop()
	return "", nil, fmt.Errorf("the shot's daemon did not open %s within %s", sock, daemonWait)
}
