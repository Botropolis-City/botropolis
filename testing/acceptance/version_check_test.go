package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

// A tree is the files a check runs against: VERSION, the changelog and
// the surface golden files, by name.
type tree struct {
	version   string
	changelog string
	surface   map[string]string
}

// changelog is a Keep a Changelog file: an [Unreleased] section holding
// unreleased, then a section per release, newest first.
func changelog(unreleased string, releases ...string) string {
	return "# Changelog\n\n## [Unreleased]\n\n" + unreleased + "\n" + strings.Join(releases, "\n")
}

func release(version, body string) string {
	return "## [" + version + "] - 2026-10-20\n\n" + body + "\n"
}

// checkVersion runs tools/check-version in a repository of its own whose
// baseline, when given, is committed and tagged v<baseline.version>, with
// the working tree then changed to now.
func checkVersion(t *testing.T, baseline *tree, now tree) (string, error) {
	t.Helper()
	script, err := os.ReadFile(helpers.Tool(t, "check-version"))
	require.NoError(t, err)
	dir := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	write := func(tr tree) {
		surface := filepath.Join(dir, "testing", "acceptance", "testdata", "surface")
		require.NoError(t, os.RemoveAll(surface))
		require.NoError(t, os.MkdirAll(surface, 0o755))
		for name, body := range tr.surface {
			require.NoError(t, os.WriteFile(filepath.Join(surface, name), []byte(body), 0o644))
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, "VERSION"), []byte(tr.version+"\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(tr.changelog), 0o644))
	}
	// The copy sits at tools/ inside the repository, because the script
	// finds the repository from its own path; helpers.Tool above read the
	// original, which keys the test cache on it.
	copied := filepath.Join(dir, "tools", "check-version")
	require.NoError(t, os.MkdirAll(filepath.Dir(copied), 0o755))
	require.NoError(t, os.WriteFile(copied, script, 0o755))
	git("init", "-q")
	if baseline != nil {
		write(*baseline)
		git("add", "-A")
		git("commit", "-q", "-m", "baseline")
		git("tag", "v"+baseline.version)
	}
	write(now)
	cmd := exec.Command(copied)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Item 149: removing anything 1.0.0 promises fails make version-check
// unless the changelog says so under Removed and, from 1.0.0 on, the
// major version rose.
func TestVersionCheckSurface(t *testing.T) {
	base := func(version string, surface map[string]string) *tree {
		return &tree{version: version, changelog: changelog("", release(version, "### Added\n\n- It.\n")), surface: surface}
	}
	two := map[string]string{"config.txt": "detail\tstring\nlabels\tbool\n"}
	one := map[string]string{"config.txt": "detail\tstring\n"}
	removedEntry := "### Removed\n\n- The `labels` key.\n"

	cases := []struct {
		scenario string
		baseline *tree
		now      tree
		passes   bool
		says     string
	}{
		{"when nothing was removed since the release", base("1.0.0", two),
			tree{"1.0.0", changelog("", release("1.0.0", "")), two}, true, ""},
		{"when a line was only added", base("1.0.0", one),
			tree{"1.0.0", changelog("", release("1.0.0", "")), two}, true, ""},
		{"when a line was removed and no Removed entry names it", base("1.0.0", two),
			tree{"1.0.0", changelog("", release("1.0.0", "")), one}, false, "labels: removed since v1.0.0 with no `labels` under Removed in CHANGELOG.md"},
		{"when a line was removed and only an Added entry names it", base("1.0.0", two),
			tree{"1.0.0", changelog("### Added\n\n- The `labels` key.\n", release("1.0.0", "")), one}, false, "no `labels` under Removed"},
		{"when a line was removed and only a release before the baseline names it", base("1.0.0", two),
			tree{"1.0.0", changelog("", release("1.0.0", ""), release("0.9.0", removedEntry)), one}, false, "no `labels` under Removed"},
		{"when a line was removed with its entry and the major version did not rise", base("1.0.0", two),
			tree{"1.0.0", changelog(removedEntry, release("1.0.0", "")), one}, false, "labels: removed since v1.0.0, which needs 2.0.0, and VERSION is 1.0.0"},
		{"when a line was removed with its entry and the major version rose", base("1.0.0", two),
			tree{"2.0.0", changelog("", release("2.0.0", removedEntry), release("1.0.0", "")), one}, true, ""},
		{"when a line was removed with its entry before 1.0.0", base("0.3.0", two),
			tree{"0.3.0", changelog(removedEntry, release("0.3.0", "")), one}, true, ""},
		{"when a line's type changed", base("1.0.0", two),
			tree{"1.0.0", changelog("", release("1.0.0", "")), map[string]string{"config.txt": "detail\tstring\nlabels\tstring\n"}}, false, "labels: removed since v1.0.0"},
		{"when a whole surface file was removed", base("1.0.0", map[string]string{"config.txt": "detail\tstring\n", "bar.txt": "class\tstring\n"}),
			tree{"1.0.0", changelog("", release("1.0.0", "")), one}, false, "class: removed since v1.0.0"},
		{"when the release has no surface files", base("1.0.0", nil),
			tree{"1.0.0", changelog("", release("1.0.0", "")), one}, true, "surface: no baseline at v1.0.0, removals unchecked"},
		{"when no release tag is reachable", nil,
			tree{"1.0.0", changelog("", release("1.0.0", "")), one}, false, "surface: no v* release tag is reachable from HEAD"},
	}
	for _, c := range cases {
		t.Run(c.scenario, func(t *testing.T) {
			t.Parallel()
			out, err := checkVersion(t, c.baseline, c.now)

			if c.passes {
				t.Run("it should pass", func(t *testing.T) {
					assert.NoError(t, err, out)
				})
			} else {
				t.Run("it should fail", func(t *testing.T) {
					assert.Error(t, err, out)
				})
			}
			if c.says != "" {
				t.Run("it should say "+c.says, func(t *testing.T) {
					assert.Contains(t, out, c.says)
				})
			}
		})
	}
}
