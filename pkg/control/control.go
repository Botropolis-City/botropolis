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
	// Read runs a command and returns what it printed.
	Read(ctx context.Context, name string, args ...string) (string, error)
}

type Control struct {
	runner   Runner
	desktop  Desktop
	getenv   func(string) string
	onPath   func(string) bool
	timeouts timeouts
}

type timeouts struct {
	start, end, list time.Duration
}

func New(runner Runner, desktop Desktop, getenv func(string) string, onPath func(string) bool) *Control {
	return &Control{runner: runner, desktop: desktop, getenv: getenv, onPath: onPath, timeouts: defaultTimeouts}
}

// defaultTimeouts are how long each kind of claude call gets before it is
// stopped and its failure said, so no claude that stalls can hold up
// whoever asked (item 286): starting or resuming a session, ending one,
// and listing them.
var defaultTimeouts = timeouts{start: 2 * time.Minute, end: 30 * time.Second, list: 20 * time.Second}

// claude runs one claude call within its time limit; one that runs out
// says so by the verb it was asked.
func (c *Control) claude(ctx context.Context, verb string, limit time.Duration, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	out, err := c.runner.Run(ctx, dir, claudeBinary, args...)
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return out, fmt.Errorf("claude %s did not answer within %s: %w", verb, within(limit), err)
	}
	return out, err
}

func within(d time.Duration) string {
	if s := d.String(); strings.HasSuffix(s, "m0s") {
		return strings.TrimSuffix(s, "0s")
	}
	return d.String()
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
	return c.background(ctx, "new", dir, args)
}

func (c *Control) Resume(ctx context.Context, dir, sessionID string) (string, error) {
	return c.background(ctx, "resume", dir, []string{"--bg", "--resume", sessionID})
}

// Fork starts a fresh session from a copy of another's conversation,
// leaving the original as it was.
func (c *Control) Fork(ctx context.Context, dir, sessionID string) (string, error) {
	return c.background(ctx, "fork", dir, []string{"--bg", "--resume", sessionID, "--fork-session"})
}

func (c *Control) background(ctx context.Context, verb, dir string, args []string) (string, error) {
	out, err := c.claude(ctx, verb, c.timeouts.start, dir, args...)
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
	out, err := c.claude(ctx, "agents", c.timeouts.list, "", args...)
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
	out, err := c.claude(ctx, "agents", c.timeouts.list, "", "agents", "--json")
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

// Attach opens a terminal on a session, started in the session's
// directory, dir: when the session ends, claude attach turns into Claude
// Code's agent view in that terminal's directory, which must be one the
// session already trusts rather than wherever the city was started
// (item 302). Empty dir starts it where the city runs.
func (c *Control) Attach(id, dir string) error {
	jobID, err := JobID(id)
	if err != nil {
		return err
	}
	argv, err := c.desktop.TerminalCommand(c.getenv, c.onPath, []string{claudeBinary, "attach", jobID})
	if err != nil {
		return err
	}
	return c.runner.Start(dir, argv[0], argv[1:]...)
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
	_, err = c.claude(ctx, verb, c.timeouts.end, "", verb, jobID)
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

// runWaitDelay is how long Run reads on once claude has exited: claude
// --bg starts a session that may keep claude's output open, and the job
// id is printed by then, so a clean exit with its pipes still held is a
// success (item 286).
const runWaitDelay = time.Second

func (execRunner) Run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	keepFromChildren()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	cmd.WaitDelay = runWaitDelay
	out, err := cmd.Output()
	if errors.Is(err, exec.ErrWaitDelay) {
		err = nil
	}
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

// Paste reads the clipboard's text through the tool Copy would use,
// under the same deadline (item 290).
func (c *Control) Paste(ctx context.Context) (string, error) {
	for _, reader := range c.desktop.Pastes {
		if !c.onPath(reader[0]) {
			continue
		}
		ctx, cancel := context.WithTimeout(ctx, copyTimeout)
		defer cancel()
		text, err := c.runner.Read(ctx, reader[0], reader[1:]...)
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("the clipboard did not answer within %s (%s)", copyTimeout, reader[0])
		}
		if err != nil {
			return "", err
		}
		if text == "" {
			return "", errors.New("the clipboard holds no text")
		}
		return text, nil
	}
	return "", errors.New("nothing to paste: no clipboard tool found: " + c.desktop.ClipboardHint)
}

// Read takes the tool's output through a file rather than a pipe, for
// the reason Feed does: a child it forks cannot hold Wait (item 284).
func (execRunner) Read(ctx context.Context, name string, args ...string) (string, error) {
	stdout, err := os.CreateTemp("", "botropolis-read-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(stdout.Name()) }()
	defer func() { _ = stdout.Close() }()
	stderr, err := os.CreateTemp("", "botropolis-read-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(stderr.Name()) }()
	defer func() { _ = stderr.Close() }()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.WaitDelay = feedWaitDelay
	keepFromChildren()
	if err := cmd.Run(); err != nil {
		said, _ := os.ReadFile(stderr.Name())
		return "", fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(said)))
	}
	out, err := os.ReadFile(stdout.Name())
	return string(out), err
}
