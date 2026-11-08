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

	fixtureGit(t, root, "init", "-q", "--bare", bare)
	fixtureGit(t, repo, "init", "-q")
	fixtureGit(t, repo, "add", "-A")
	fixtureGit(t, repo, "commit", "-q", "-m", "fixture")
	fixtureGit(t, repo, "remote", "add", "github", bare)
	fixtureGit(t, repo, "push", "-q", "github", "HEAD:main")
	return filepath.Join(repo, "tools", "check-publishable")
}

func fixtureGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

const publishableToken = "zorblax"

type pullCommit struct{ file, text, message string }

type publishableRun struct {
	out  string
	code int
}

// Item 369: GitHub keeps refs/pull/N/head after a pull request's branch is
// deleted, and a squash merge leaves it outside main's history for good.
// setup pushes to the remote from a second clone; the check then runs in a
// fresh clone, which has fetched none of what it is about to judge. It clones
// with --no-local, because a local clone copies every object in the remote,
// and a check that skipped the fetch would still find them.
func runPublishable(t *testing.T, tokens bool, setup func(other string)) publishableRun {
	t.Helper()
	src, err := os.ReadFile(helpers.Tool(t, "check-publishable"))
	require.NoError(t, err)

	root := t.TempDir()
	bare := filepath.Join(root, "remote.git")
	other := filepath.Join(root, "other")
	publisher := filepath.Join(root, "publisher")
	fixtureGit(t, root, "init", "-q", "--bare", bare)
	fixtureGit(t, root, "clone", "-q", "-o", "github", bare, other)
	fixtureGit(t, other, "checkout", "-q", "-b", "main")
	require.NoError(t, os.MkdirAll(filepath.Join(other, "tools"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(other, "tools", "check-publishable"), src, 0o755))
	fixtureGit(t, other, "add", "-A")
	fixtureGit(t, other, "commit", "-q", "-m", "the guard")
	fixtureGit(t, other, "push", "-q", "github", "main")
	setup(other)
	fixtureGit(t, root, "clone", "-q", "--no-local", "-o", "github", bare, publisher)
	if tokens {
		require.NoError(t, os.WriteFile(filepath.Join(publisher, ".publishable-tokens"), []byte(publishableToken+"\n"), 0o644))
	}

	cmd := exec.Command(filepath.Join(publisher, "tools", "check-publishable"))
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	run := publishableRun{out: string(out)}
	if exit, ok := err.(*exec.ExitError); ok {
		run.code = exit.ExitCode()
	} else {
		require.NoError(t, err, string(out))
	}
	return run
}

func mergePullRequest(t *testing.T, other string, commits ...pullCommit) {
	t.Helper()
	fixtureGit(t, other, "checkout", "-q", "-b", "pull-1")
	for _, c := range commits {
		require.NoError(t, os.WriteFile(filepath.Join(other, c.file), []byte(c.text), 0o644))
		fixtureGit(t, other, "add", c.file)
		fixtureGit(t, other, "commit", "-q", "-m", c.message)
	}
	fixtureGit(t, other, "push", "-q", "github", "HEAD:refs/pull/1/head")
	fixtureGit(t, other, "checkout", "-q", "main")
	fixtureGit(t, other, "merge", "-q", "--squash", "pull-1")
	fixtureGit(t, other, "commit", "-q", "-m", "Squashed pull request 1")
	fixtureGit(t, other, "push", "-q", "github", "HEAD:main")
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

	t.Run("when a merged pull request's ref is outside the history being published", func(t *testing.T) {
		const pull = "refs/pull/1/head"
		cases := []struct {
			name    string
			tokens  bool
			commits []pullCommit
			code    int
			says    string
			should  string
		}{
			{
				name:   "and its commits are clean",
				tokens: true,
				commits: []pullCommit{
					{"feature", "a feature\n", "Add a feature"},
					{"feature", "a better feature\n", "Improve the feature"},
				},
				code: 0, says: pull, should: "it should pass",
			},
			{
				name:   "and a file it adds carries a token",
				tokens: true,
				commits: []pullCommit{
					{"feature", "made for " + publishableToken + "\n", "Add a feature"},
					{"feature", "made for nobody\n", "Take the name out"},
				},
				code: 1, says: pull, should: "it should fail",
			},
			{
				name:   "and its commit message carries a token",
				tokens: true,
				commits: []pullCommit{
					{"feature", "a feature\n", "Add the feature " + publishableToken + " asked for"},
				},
				code: 1, says: pull, should: "it should fail",
			},
			{
				name:    "and there is no tokens list",
				tokens:  false,
				commits: []pullCommit{{"feature", "a feature\n", "Add a feature"}},
				code:    0, says: pull + " is outside the history being published, and NOT swept", should: "it should pass",
			},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				run := runPublishable(t, c.tokens, func(other string) { mergePullRequest(t, other, c.commits...) })

				t.Run(c.should, func(t *testing.T) {
					assert.Equal(t, c.code, run.code, run.out)
				})
				t.Run("it should name the pull request's ref", func(t *testing.T) {
					assert.Contains(t, run.out, c.says)
				})
			})
		}
	})

	t.Run("when a remote branch is outside the history being published", func(t *testing.T) {
		run := runPublishable(t, true, func(other string) {
			fixtureGit(t, other, "checkout", "-q", "-b", "side")
			require.NoError(t, os.WriteFile(filepath.Join(other, "never-landed"), []byte("never landed\n"), 0o644))
			fixtureGit(t, other, "add", "never-landed")
			fixtureGit(t, other, "commit", "-q", "-m", "never landed")
			fixtureGit(t, other, "push", "-q", "github", "side")
		})

		t.Run("it should fail", func(t *testing.T) {
			assert.Equal(t, 1, run.code, run.out)
		})
		t.Run("it should name the branch", func(t *testing.T) {
			assert.Contains(t, run.out, "refs/heads/side")
		})
	})

	t.Run("when an open pull request's branch is outside the history being published", func(t *testing.T) {
		run := runPublishable(t, true, func(other string) {
			fixtureGit(t, other, "checkout", "-q", "-b", "side")
			require.NoError(t, os.WriteFile(filepath.Join(other, "awaiting-review"), []byte("awaiting review\n"), 0o644))
			fixtureGit(t, other, "add", "awaiting-review")
			fixtureGit(t, other, "commit", "-q", "-m", "awaiting review")
			fixtureGit(t, other, "push", "-q", "github", "side", "HEAD:refs/pull/2/head")
		})

		t.Run("it should pass", func(t *testing.T) {
			assert.Equal(t, 0, run.code, run.out)
		})
		t.Run("it should name the branch", func(t *testing.T) {
			assert.Contains(t, run.out, "refs/heads/side")
		})
	})
}
