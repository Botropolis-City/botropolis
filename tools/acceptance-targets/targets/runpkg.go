package targets

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// RunPackage builds dir's test binary once and runs each of its tests,
// less the performance tier, in a process of its own, parallel at once
// (item 226). Each process is the package's own TestMain, so each test gets
// the isolation TestMain gives a whole run. timeout is each test's
// -test.timeout; a child still running a minute past it is killed, with
// everything it started. pattern, when not empty, narrows the run to the
// tests it matches, as go test -run would.
func RunPackage(dir, measurePath, pattern string, parallel int, timeout time.Duration, w io.Writer) int {
	match, err := regexp.Compile(pattern)
	if err != nil {
		fmt.Fprintln(w, err)
		return 2
	}
	tiers, err := ReadTiers(dir, measurePath)
	if err != nil {
		fmt.Fprintln(w, err)
		return 1
	}
	work, err := os.MkdirTemp("", "acceptance-run-")
	if err != nil {
		fmt.Fprintln(w, err)
		return 1
	}
	defer func() { _ = os.RemoveAll(work) }()
	bin := filepath.Join(work, "package.test")
	build := exec.Command("go", "test", "-c", "-o", bin, ".")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(w, "building the test binary: %v\n%s", err, out)
		return 1
	}
	list := exec.Command(bin, "-test.list", ".")
	list.Dir = dir
	out, err := list.Output()
	if err != nil {
		fmt.Fprintf(w, "listing the tests: %v\n%s", err, out)
		return 1
	}
	var listed []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Test") {
			listed = append(listed, line)
		}
	}
	runnable, err := Runnable(listed, tiers)
	if err != nil {
		fmt.Fprintln(w, err)
		return 1
	}
	var tests []string
	for _, name := range runnable {
		if match.MatchString(name) {
			tests = append(tests, name)
		}
	}
	return RunPool(tests, parallel, timeout+time.Minute, processChild(bin, dir, timeout), w)
}

func processChild(bin, dir string, timeout time.Duration) Child {
	return func(ctx context.Context, test string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, bin, "-test.run", "^"+test+"$", "-test.v", "-test.count=1", "-test.timeout", timeout.String())
		cmd.Dir = dir
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
		cmd.WaitDelay = 10 * time.Second
		return cmd.CombinedOutput()
	}
}
