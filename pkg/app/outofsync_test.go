package app

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/proto"
)

// Item 152: a command refused by a daemon out of sync says so on one
// line, with what to do about it.
func TestErrorLine(t *testing.T) {
	hint := func(*proto.Mismatch) string { return `restart it with "systemctl --user restart botropolisd"` }
	mismatch := &proto.Mismatch{Daemon: proto.Peer{Release: "0.2.2", Protocol: 1}, Ours: proto.Peer{Release: "0.2.3", Protocol: 1}}

	for _, c := range []struct {
		scenario string
		err      error
		line     string
	}{
		{"when the daemon is out of sync", mismatch,
			`botropolis: daemon is 0.2.2, this is 0.2.3: restart it with "systemctl --user restart botropolisd"`},
		{"when the mismatch is wrapped", fmt.Errorf("find: %w", mismatch),
			`botropolis: daemon is 0.2.2, this is 0.2.3: restart it with "systemctl --user restart botropolisd"`},
		{"when it is any other error", errors.New("no such session"), "botropolis: no such session"},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			t.Run("it should print the line", func(t *testing.T) {
				assert.Equal(t, c.line, errorLine(c.err, hint))
			})
		})
	}
}

// Item 152: Restart Botropolis runs the map again from its path on disk,
// which after an upgrade is the new build, with its own arguments.
func TestRestartPath(t *testing.T) {
	look := func(name string) (string, error) {
		if name == "botropolis" {
			return "/usr/bin/botropolis", nil
		}
		return "", exec.ErrNotFound
	}
	executable := func() (string, error) { return "/opt/homebrew/Cellar/botropolis/0.2.2/bin/botropolis", nil }

	for _, c := range []struct {
		scenario string
		args0    string
		path     string
	}{
		{"when it was started by name", "botropolis", "/usr/bin/botropolis"},
		{"when it was started by a path", "./bin/botropolis", "./bin/botropolis"},
		{"when its name is not on the PATH", "botropolis-dev", "/opt/homebrew/Cellar/botropolis/0.2.2/bin/botropolis"},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			path, _ := restartPath(c.args0, look, executable)

			t.Run("it should run that path", func(t *testing.T) {
				assert.Equal(t, c.path, path)
			})
		})
	}
}
