package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

// Item 135: the image check matched each image by piping the reviewed list
// into `grep -q` under pipefail, so grep exiting on an early match killed the
// writer with SIGPIPE and failed an image that was on the list. A list tens of
// thousands of lines long with the image first makes that near-certain.
//
// It returns the path of the padded copy it runs. The real script is read
// through helpers.Tool, which puts it in the test cache's key.
func publishableRepo(t *testing.T, image string, filler int) string {
	t.Helper()
	src, err := os.ReadFile(helpers.Tool(t, "check-publishable"))
	require.NoError(t, err)

	head := "reviewed=$(cat <<'LIST'\n"
	require.Contains(t, string(src), head)
	var pad strings.Builder
	pad.WriteString(head + image + "\n")
	for i := 0; i < filler; i++ {
		pad.WriteString("docs/screenshots/filler-" + strconv.Itoa(i) + ".png\n")
	}
	script := strings.Replace(string(src), head, pad.String(), 1)

	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	bare := filepath.Join(root, "remote.git")
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "tools"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repo, filepath.Dir(image)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "tools", "check-publishable"), []byte(script), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, image), []byte("not really a png"), 0o644))

	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
			"GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	git(root, "init", "-q", "--bare", bare)
	git(repo, "init", "-q")
	git(repo, "add", "-A")
	git(repo, "commit", "-q", "-m", "fixture")
	git(repo, "remote", "add", "github", bare)
	git(repo, "push", "-q", "github", "HEAD:main")
	return filepath.Join(repo, "tools", "check-publishable")
}

func TestCheckPublishable(t *testing.T) {
	t.Run("when an image is on a reviewed list tens of thousands of lines long", func(t *testing.T) {
		t.Run("and it is the first entry", func(t *testing.T) {
			checker := publishableRepo(t, "docs/screenshots/first.png", 50000)
			const runs = 20
			var failures []string
			for i := 0; i < runs; i++ {
				out, err := exec.Command(checker).CombinedOutput()
				if err != nil {
					failures = append(failures, string(out))
				}
			}

			t.Run("it should pass on every one of 20 runs", func(t *testing.T) {
				assert.Empty(t, failures)
			})
		})
	})
}
