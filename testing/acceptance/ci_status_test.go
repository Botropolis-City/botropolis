package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 202: main's ci was red for two days because nothing read CI after a
// landing. tools/ci-status asks gh for the last completed run on main, says
// what it found, records it where the status line reads it, and never
// fails the landing. gh here is a stand-in that answers as gh's --jq would.
func TestCIStatus(t *testing.T) {
	t.Parallel()
	script := filepath.Join(repoRoot(t), "tools", "ci-status")
	recorded := regexp.MustCompile(`^(green|red|unknown) (\S+) \d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ\n$`)

	type result struct {
		out    string
		err    error
		record []string
	}
	run := func(t *testing.T, answer string, exit string) result {
		t.Helper()
		repo := t.TempDir()
		require.NoError(t, exec.Command("git", "init", "-q", repo).Run())
		bin := t.TempDir()
		gh := "#!/usr/bin/env bash\nprintf '%s\\n' \"$ANSWER\"\nexit \"$EXIT\"\n"
		require.NoError(t, os.WriteFile(filepath.Join(bin, "gh"), []byte(gh), 0o755))
		cmd := exec.Command(script)
		cmd.Dir = repo
		cmd.Env = append(landEnviron(), "PATH="+bin+":"+os.Getenv("PATH"), "ANSWER="+answer, "EXIT="+exit)
		out, err := cmd.CombinedOutput()
		data, _ := os.ReadFile(filepath.Join(repo, ".git", "botropolis-ci"))
		m := recorded.FindStringSubmatch(string(data))
		if m != nil {
			m = m[1:]
		}
		return result{string(out), err, m}
	}

	t.Run("when the last completed ci run on main failed", func(t *testing.T) {
		t.Parallel()
		r := run(t, "failure abc1234 https://github.com/Botropolis-City/botropolis/actions/runs/1", "0")

		t.Run("it should say main is red, with the commit and the run's URL", func(t *testing.T) {
			assert.Contains(t, r.out, "CI on main: RED at abc1234, https://github.com/Botropolis-City/botropolis/actions/runs/1")
		})

		t.Run("it should exit 0", func(t *testing.T) {
			assert.NoError(t, r.err, r.out)
		})

		t.Run("it should record red and the commit in the git common dir", func(t *testing.T) {
			assert.Equal(t, []string{"red", "abc1234"}, r.record)
		})
	})

	t.Run("when it succeeded", func(t *testing.T) {
		t.Parallel()
		r := run(t, "success abc1234 https://github.com/Botropolis-City/botropolis/actions/runs/1", "0")

		t.Run("it should say main is green", func(t *testing.T) {
			assert.Contains(t, r.out, "CI on main: green at abc1234")
		})

		t.Run("it should record green and the commit", func(t *testing.T) {
			assert.Equal(t, []string{"green", "abc1234"}, r.record)
		})
	})

	t.Run("when gh fails", func(t *testing.T) {
		t.Parallel()
		r := run(t, "HTTP 401: Bad credentials", "1")

		t.Run("it should say the state is unknown, with gh's error", func(t *testing.T) {
			assert.Contains(t, r.out, "CI on main: unknown, gh failed: HTTP 401: Bad credentials")
		})

		t.Run("it should exit 0", func(t *testing.T) {
			assert.NoError(t, r.err, r.out)
		})

		t.Run("it should record unknown", func(t *testing.T) {
			assert.Equal(t, []string{"unknown", "-"}, r.record)
		})
	})
}
