// Command land-queue serves the landing lock in arrival order (item 295).
//
// flock hands a released lock to any waiter, so with eight landings queued a
// landing could wait behind later arrivals for most of an hour. Each arrival
// takes a numbered ticket in the queue directory and holds a flock on it for
// as long as it lives, so a ticket whose lock can be taken belongs to a waiter
// that died. The lowest live ticket takes the landing lock itself, which stays
// the mutual exclusion beneath the queue, so anything still taking the lock
// with flock alone is excluded too, just not ordered.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const usage = `land-queue — serve the landing lock in arrival order

Usage:
  land-queue run  --lock <file> --queue <dir> --name <target> -- <command...>
  land-queue list --lock <file> --queue <dir>
  land-queue --help

run takes a ticket in <dir>, waits until every live ticket ahead of it is
gone, takes the flock on <file>, runs the command holding both, and exits with
the command's code. A waiter that dies leaves the queue: its ticket's flock is
released with it, and the next waiter to look clears the ticket.

list prints who holds the lock and who waits, since when, for which target,
from which worktree and at which commit. It takes no lock beyond non-blocking
probes and changes nothing.
`

const poll = 250 * time.Millisecond

type ticket struct {
	PID      int       `json:"pid"`
	Since    time.Time `json:"since"`
	Target   string    `json:"target"`
	Worktree string    `json:"worktree"`
	Head     string    `json:"head"`
}

func main() {
	if len(os.Args) < 2 || os.Args[1] == "--help" || os.Args[1] == "-h" {
		fmt.Print(usage)
		if len(os.Args) < 2 {
			os.Exit(2)
		}
		return
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	lock := fs.String("lock", "", "the landing lock")
	queue := fs.String("queue", "", "the queue directory")
	name := fs.String("name", "", "the target being run")
	_ = fs.Parse(os.Args[2:])
	if *lock == "" || *queue == "" {
		fail(2, "land-queue %s needs --lock and --queue", os.Args[1])
	}
	switch os.Args[1] {
	case "run":
		if fs.NArg() == 0 || *name == "" {
			fail(2, "land-queue run needs --name and a command after --")
		}
		os.Exit(run(*lock, *queue, *name, fs.Args()))
	case "list":
		if err := list(os.Stdout, *lock, *queue, time.Now()); err != nil {
			fail(1, "land-queue list: %v", err)
		}
	default:
		fail(2, "land-queue: unknown command %q\n\n%s", os.Args[1], usage)
	}
}

func fail(code int, format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(code)
}

func run(lockPath, queue, name string, command []string) int {
	mine, ticketFile, err := take(queue, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "land-queue: taking a ticket: %v\n", err)
		return 1
	}
	ticketPath := filepath.Join(queue, mine)
	defer func() { _ = ticketFile.Close() }()
	defer func() { _ = os.Remove(ticketPath) }()

	told := false
	for {
		ahead, err := clearAhead(queue, mine)
		if err != nil {
			fmt.Fprintf(os.Stderr, "land-queue: reading the queue: %v\n", err)
			return 1
		}
		if ahead == 0 {
			break
		}
		if !told {
			fmt.Printf("Waiting for the landing in progress to finish (%s); %d ahead in the queue\n", lockPath, ahead)
			told = true
		}
		time.Sleep(poll)
	}

	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "land-queue: opening %s: %v\n", lockPath, err)
		return 1
	}
	defer func() { _ = lockFile.Close() }()
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if !told {
			fmt.Printf("Waiting for the landing in progress to finish (%s)\n", lockPath)
		}
		if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
			fmt.Fprintf(os.Stderr, "land-queue: locking %s: %v\n", lockPath, err)
			return 1
		}
	}
	_ = lockFile.Truncate(0)
	_, _ = lockFile.WriteAt([]byte(mine+"\n"), 0)
	defer func() { _ = lockFile.Truncate(0) }()

	return execHolding(command, lockFile, ticketFile)
}

// execHolding runs the command with both locks' descriptors inherited, as
// flock(1) does, so the lock lasts as long as the command even if this
// process is killed alone.
func execHolding(command []string, files ...*os.File) int {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.ExtraFiles = files
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "land-queue: %v\n", err)
		return 127
	}
	go func() {
		for s := range signals {
			_ = cmd.Process.Signal(s)
		}
	}()
	err := cmd.Wait()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal())
		}
		return exit.ExitCode()
	default:
		fmt.Fprintf(os.Stderr, "land-queue: %v\n", err)
		return 1
	}
}

// take writes and locks a ticket under a temporary name, then renames it into
// the queue, so no reader ever sees a live ticket unlocked.
func take(queue, name string) (string, *os.File, error) {
	if err := os.MkdirAll(queue, 0o777); err != nil {
		return "", nil, err
	}
	guard, err := os.OpenFile(filepath.Join(queue, ".lock"), os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = guard.Close() }()
	if err := syscall.Flock(int(guard.Fd()), syscall.LOCK_EX); err != nil {
		return "", nil, err
	}

	n, err := next(queue)
	if err != nil {
		return "", nil, err
	}
	mine := fmt.Sprintf("%09d", n)
	worktree, _ := os.Getwd()
	data, err := json.Marshal(ticket{PID: os.Getpid(), Since: time.Now(), Target: name, Worktree: worktree, Head: head()})
	if err != nil {
		return "", nil, err
	}
	tmp, err := os.CreateTemp(queue, ".ticket-")
	if err != nil {
		return "", nil, err
	}
	fail := func(err error) (string, *os.File, error) {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", nil, err
	}
	if err := syscall.Flock(int(tmp.Fd()), syscall.LOCK_EX); err != nil {
		return fail(err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fail(err)
	}
	if err := os.Rename(tmp.Name(), filepath.Join(queue, mine)); err != nil {
		return fail(err)
	}
	if err := os.WriteFile(filepath.Join(queue, "next"), []byte(strconv.Itoa(n+1)+"\n"), 0o666); err != nil {
		_ = os.Remove(filepath.Join(queue, mine))
		return fail(err)
	}
	return mine, tmp, nil
}

// next is the counter, or past the highest ticket if the counter was lost.
func next(queue string) (int, error) {
	n := 1
	if data, err := os.ReadFile(filepath.Join(queue, "next")); err == nil {
		if v, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && v > n {
			n = v
		}
	}
	names, err := tickets(queue)
	if err != nil {
		return 0, err
	}
	for _, name := range names {
		if v, _ := strconv.Atoi(name); v >= n {
			n = v + 1
		}
	}
	return n, nil
}

func head() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "-"
	}
	return strings.TrimSpace(string(out))
}

func tickets(queue string) ([]string, error) {
	entries, err := os.ReadDir(queue)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.Name() != "" && strings.Trim(e.Name(), "0123456789") == "" {
			names = append(names, e.Name())
		}
	}
	sort.Slice(names, func(i, j int) bool {
		a, _ := strconv.Atoi(names[i])
		b, _ := strconv.Atoi(names[j])
		return a < b
	})
	return names, nil
}

// clearAhead counts the live tickets ahead of mine, removing any it finds dead.
func clearAhead(queue, mine string) (int, error) {
	names, err := tickets(queue)
	if err != nil {
		return 0, err
	}
	me, _ := strconv.Atoi(mine)
	ahead := 0
	for _, name := range names {
		if n, _ := strconv.Atoi(name); n >= me {
			continue
		}
		path := filepath.Join(queue, name)
		f, err := os.Open(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil {
			_ = os.Remove(path)
		} else {
			ahead++
		}
		_ = f.Close()
	}
	return ahead, nil
}

// alive probes a file's flock without waiting: true when someone holds it.
func alive(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	return syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB) != nil
}

func list(w io.Writer, lockPath, queue string, now time.Time) error {
	names, err := tickets(queue)
	if err != nil {
		return err
	}
	type entry struct {
		name string
		t    ticket
	}
	var live []entry
	for _, name := range names {
		path := filepath.Join(queue, name)
		data, err := os.ReadFile(path)
		if err != nil || !alive(path) {
			continue
		}
		var t ticket
		if json.Unmarshal(data, &t) != nil {
			continue
		}
		live = append(live, entry{name, t})
	}

	held := alive(lockPath)
	holder := ""
	if held {
		if data, err := os.ReadFile(lockPath); err == nil {
			holder = strings.TrimSpace(string(data))
		}
	}
	outside := held && (len(live) == 0 || live[0].name != holder)

	switch {
	case len(live) == 0 && !held:
		_, err := fmt.Fprintf(w, "Nobody is landing or waiting (%s)\n", queue)
		return err
	case outside:
		if _, err := fmt.Fprintf(w, "The lock is held outside the queue (%s), by a make that takes it with flock alone\n", lockPath); err != nil {
			return err
		}
	}
	if len(live) == 0 {
		_, err := fmt.Fprintln(w, "Nobody is waiting in the queue")
		return err
	}
	if _, err := fmt.Fprintf(w, "Landing queue (%s):\n", queue); err != nil {
		return err
	}
	for _, e := range live {
		state := "waiting"
		if held && e.name == holder {
			state = "holding"
		}
		n, _ := strconv.Atoi(e.name)
		if _, err := fmt.Fprintf(w, "  #%d %s since %s (%s) %s %s %s\n",
			n, state, e.t.Since.Format("15:04:05"), now.Sub(e.t.Since).Round(time.Second), e.t.Target, e.t.Head, e.t.Worktree); err != nil {
			return err
		}
	}
	return nil
}
