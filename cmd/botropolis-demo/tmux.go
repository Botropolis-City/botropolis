package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"botropolis.city/botropolis/pkg/demo"
)

// tmuxSession is claude running interactively in a detached tmux
// session, which is where its teammates open their panes too.
type tmuxSession struct {
	name string
	once sync.Once
	err  error
}

// runTmux starts claude in a tmux session of its own. tmux passes the
// recorder's environment on, so the token reaches claude by name.
func runTmux(_ context.Context, dir string, args, env []string) (demo.Run, error) {
	s := &tmuxSession{name: fmt.Sprintf("botropolis-demo-%d", time.Now().UnixNano())}
	argv := []string{"new-session", "-d", "-s", s.name, "-x", "200", "-y", "50", "-c", dir}
	for _, e := range env {
		argv = append(argv, "-e", e)
	}
	argv = append(append(argv, "claude"), args...)
	if out, err := exec.Command("tmux", argv...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("tmux new-session: %w: %s", err, out)
	}
	return s, nil
}

// Stop asks claude to exit, gives it a moment, and then ends the tmux
// session and every pane in it, teammates included. The last screen
// goes to stderr, so a run that stalled on a prompt says which.
func (s *tmuxSession) Stop() error {
	s.once.Do(func() {
		if screen, err := exec.Command("tmux", "capture-pane", "-p", "-t", s.name).Output(); err == nil {
			fmt.Fprintf(os.Stderr, "last screen of %s:\n%s\n", s.name, strings.TrimRight(string(screen), "\n"))
		}
		_ = exec.Command("tmux", "send-keys", "-t", s.name, "/exit", "Enter").Run()
		for end := time.Now().Add(10 * time.Second); time.Now().Before(end); time.Sleep(500 * time.Millisecond) {
			if exec.Command("tmux", "has-session", "-t", s.name).Run() != nil {
				return
			}
		}
		if out, err := exec.Command("tmux", "kill-session", "-t", s.name).CombinedOutput(); err != nil {
			s.err = fmt.Errorf("tmux kill-session: %w: %s", err, out)
		}
	})
	return s.err
}
