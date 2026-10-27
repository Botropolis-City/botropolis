package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"
)

const claudeBinary = "claude"

// copyTimeout is how long a clipboard tool gets before it is stopped
// and its failure becomes a notice (item 284).
const copyTimeout = 2 * time.Second

var ErrUntrusted = errors.New("workspace not trusted")

var (
	jobIDPattern        = regexp.MustCompile(`^[0-9a-f]{8}$`)
	backgroundedPattern = regexp.MustCompile(`backgrounded\s*\x{B7}?\s*([0-9a-f]{8})`)
	untrustedPattern    = regexp.MustCompile(`(?i)workspace not trusted`)
	ansiPattern         = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
)

type Runner interface {
	Run(ctx context.Context, dir string, name string, args ...string) (string, error)
	Start(dir string, name string, args ...string) error
	// Interactive hands the caller's terminal to a command.
	Interactive(name string, args ...string) error
	// Feed runs a command with input on its stdin.
	Feed(ctx context.Context, input string, name string, args ...string) error
}

type Control struct {
	runner  Runner
	desktop Desktop
	getenv  func(string) string
	onPath  func(string) bool
}

func New(runner Runner, desktop Desktop, getenv func(string) string, onPath func(string) bool) *Control {
	return &Control{runner: runner, desktop: desktop, getenv: getenv, onPath: onPath}
}

func Default(terminal string) *Control {
	getenv := func(key string) string {
		if key == "BOTROPOLIS_TERMINAL" && terminal != "" {
			return terminal
		}
		return os.Getenv(key)
	}
	return New(execRunner{}, Native(), getenv, func(name string) bool {
		_, err := exec.LookPath(name)
		return err == nil
	})
}

func (c *Control) New(ctx context.Context, dir, prompt string) (string, error) {
	args := []string{"--bg"}
	if prompt != "" {
		args = append(args, prompt)
	}
	return c.background(ctx, dir, args)
}

func (c *Control) Resume(ctx context.Context, dir, sessionID string) (string, error) {
	return c.background(ctx, dir, []string{"--bg", "--resume", sessionID})
}

// Fork starts a fresh session from a copy of another's conversation,
// leaving the original as it was.
func (c *Control) Fork(ctx context.Context, dir, sessionID string) (string, error) {
	return c.background(ctx, dir, []string{"--bg", "--resume", sessionID, "--fork-session"})
}

func (c *Control) background(ctx context.Context, dir string, args []string) (string, error) {
	out, err := c.runner.Run(ctx, dir, claudeBinary, args...)
	if err != nil && untrustedPattern.MatchString(err.Error()) {
		return "", untrustedError{err}
	}
	if err != nil {
		return "", err
	}
	if m := backgroundedPattern.FindStringSubmatch(ansiPattern.ReplaceAllString(out, "")); m != nil {
		return m[1], nil
	}
	return c.newestJob(ctx, dir)
}

type untrustedError struct{ err error }

func (e untrustedError) Error() string   { return e.err.Error() }
func (e untrustedError) Unwrap() []error { return []error{ErrUntrusted, e.err} }

type Agent struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	CWD       string `json:"cwd"`
	Kind      string `json:"kind"`
	Status    string `json:"status"`
	State     string `json:"state"`
	PID       int    `json:"pid"`
	StartedAt int64  `json:"startedAt"`
}

type agentJSON = Agent

func (c *Control) Agents(ctx context.Context, all bool) ([]Agent, error) {
	args := []string{"agents", "--json"}
	if all {
		args = []string{"agents", "--all", "--json"}
	}
	out, err := c.runner.Run(ctx, "", claudeBinary, args...)
	if err != nil {
		return nil, err
	}
	var agents []Agent
	if err := json.Unmarshal([]byte(out), &agents); err != nil {
		return nil, fmt.Errorf("claude agents --json is unreadable: %w", err)
	}
	return agents, nil
}

func (c *Control) newestJob(ctx context.Context, dir string) (string, error) {
	out, err := c.runner.Run(ctx, "", claudeBinary, "agents", "--json")
	if err != nil {
		return "", fmt.Errorf("claude started a session but printed no job id, and listing agents failed: %w", err)
	}
	var agents []agentJSON
	if err := json.Unmarshal([]byte(out), &agents); err != nil {
		return "", fmt.Errorf("claude started a session but printed no job id, and agents --json is unreadable: %w", err)
	}
	var newest agentJSON
	for _, a := range agents {
		if a.Kind == "background" && a.CWD == dir && a.ID != "" && a.StartedAt > newest.StartedAt {
			newest = a
		}
	}
	if newest.ID == "" {
		return "", fmt.Errorf("claude started a session in %s but no job id could be found for it", dir)
	}
	return newest.ID, nil
}

func (c *Control) Attach(id string) error {
	jobID, err := JobID(id)
	if err != nil {
		return err
	}
	argv, err := c.desktop.TerminalCommand(c.getenv, c.onPath, []string{claudeBinary, "attach", jobID})
	if err != nil {
		return err
	}
	return c.runner.Start("", argv[0], argv[1:]...)
}

// AttachHere attaches to a session in the terminal botropolis runs in.
func (c *Control) AttachHere(id string) error {
	jobID, err := JobID(id)
	if err != nil {
		return err
	}
	return c.runner.Interactive(claudeBinary, "attach", jobID)
}

// Reveal opens a directory in the desktop's file manager.
func (c *Control) Reveal(dir string) error {
	if dir == "" {
		return errors.New("nothing to reveal")
	}
	return c.runner.Start("", c.desktop.Opener, dir)
}

// Copy puts text on the clipboard through whichever tool is installed.
// A tool that has not finished within copyTimeout is stopped, and the
// copy fails rather than holding up whoever asked.
func (c *Control) Copy(ctx context.Context, text string) error {
	for _, tool := range c.desktop.Clipboards {
		if c.onPath(tool[0]) {
			ctx, cancel := context.WithTimeout(ctx, copyTimeout)
			defer cancel()
			return c.runner.Feed(ctx, text, tool[0], tool[1:]...)
		}
	}
	return errors.New("no clipboard tool found: " + c.desktop.ClipboardHint)
}

func (c *Control) Stop(ctx context.Context, id string) error {
	return c.simple(ctx, "stop", id)
}

func (c *Control) Remove(ctx context.Context, id string) error {
	return c.simple(ctx, "rm", id)
}

func (c *Control) simple(ctx context.Context, verb, id string) error {
	jobID, err := JobID(id)
	if err != nil {
		return err
	}
	_, err = c.runner.Run(ctx, "", claudeBinary, verb, jobID)
	return err
}

func JobID(id string) (string, error) {
	if len(id) > 8 && id[8] == '-' {
		id = id[:8]
	}
	if !jobIDPattern.MatchString(id) {
		return "", fmt.Errorf("%q is not a job id (eight hex characters) or a session id", id)
	}
	return id, nil
}

type execRunner struct{}

// Feed waits for the tool to exit and never for its output to close:
// clipboard tools fork a child that keeps serving the selection with
// the tool's stdout and stderr still open (item 284). So stdout goes
// nowhere and stderr to a file, neither of which Wait waits on.
func (execRunner) Feed(ctx context.Context, input string, name string, args ...string) error {
	stderr, err := os.CreateTemp("", "botropolis-feed-")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(stderr.Name()) }()
	defer func() { _ = stderr.Close() }()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = stderr
	cmd.WaitDelay = feedWaitDelay
	keepFromChildren()
	if err := cmd.Run(); err != nil {
		said, _ := os.ReadFile(stderr.Name())
		return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(said)))
	}
	return nil
}

const feedWaitDelay = time.Second

func (execRunner) Run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	keepFromChildren()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(string(out))
		}
		if msg != "" {
			return string(out), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, msg)
		}
		return string(out), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return string(out), nil
}

func (execRunner) Start(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	keepFromChildren()
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// Interactive replaces this process with the command, so it has the
// terminal, its signals and its exit status to itself.
func (execRunner) Interactive(name string, args ...string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return err
	}
	return syscall.Exec(path, append([]string{name}, args...), os.Environ())
}

var errNoTerminal = errors.New("no terminal found: set BOTROPOLIS_TERMINAL (a command prefix, e.g. \"wezterm start --\") or TERMINAL")
