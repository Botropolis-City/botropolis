package acceptance_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/tools/acceptance-targets/targets"
)

// TestTargetedRuns runs the tool with go run, which the test cache cannot see;
// referencing its entry point links the tool's code into this test binary.
var targetsEntry = targets.Main

const universe = "^(TestStatus|TestReleaseAliases)$"

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

func runTargets(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run", "./tools/acceptance-targets"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return string(out)
}

func selected(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "selected") {
			return line
		}
	}
	return ""
}

func appendTo(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = f.WriteString(text)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

func TestTheTestBinaryLinksTheTargetsTool(t *testing.T) {
	t.Run("when the acceptance test binary is built", func(t *testing.T) {
		t.Run("it should link the entry point of the tool TestTargetedRuns runs", func(t *testing.T) {
			assert.NotZero(t, reflect.ValueOf(targetsEntry).Pointer())
		})
	})
}

func TestTargetedRuns(t *testing.T) {
	t.Run("when the map is made for TestStatus and TestReleaseAliases only", func(t *testing.T) {
		if testing.Short() {
			t.Skip("runs the acceptance tests under coverage")
		}
		dir := scratchModule(t, "tools", ".gitignore")
		gitIn(t, dir, "init", "-q")
		gitIn(t, dir, "add", "-A")
		gitIn(t, dir, "commit", "-q", "-m", "base")
		runTargets(t, dir, "map", "-run", universe)
		maps, err := filepath.Glob(filepath.Join(targets.MapDir(os.Getenv, ""), "*.json"))
		require.NoError(t, err)
		require.Len(t, maps, 1, "one map, under the shared directory, for the one revision")
		data, err := os.ReadFile(maps[0])
		require.NoError(t, err)
		var m targets.Map
		require.NoError(t, json.Unmarshal(data, &m))

		t.Run("it should map TestStatus to pkg/state", func(t *testing.T) {
			assert.Contains(t, m.Tests["TestStatus"], "botropolis.city/botropolis/pkg/state")
		})

		ci := filepath.Join(dir, ".git", "botropolis-ci")
		require.NoError(t, os.WriteFile(ci, []byte("red 0000000 2026-10-24T12:00:00Z\n"), 0o644))
		appendTo(t, filepath.Join(dir, "pkg", "state", "state.go"), "\n// item 117 probe\n")
		redRun := runTargets(t, dir, "changed", "-run", universe)

		t.Run("and main's CI was last recorded red", func(t *testing.T) {
			t.Run("it should run everything and say why", func(t *testing.T) {
				assert.Contains(t, redRun, "full run: main's CI is red")
			})
		})

		require.NoError(t, os.WriteFile(ci, []byte("green 0000000 2026-10-24T12:00:00Z\n"), 0o644))
		stateOut := runTargets(t, dir, "changed", "-run", universe)
		stateRun := selected(stateOut)

		t.Run("and a file under pkg/state is then changed", func(t *testing.T) {
			t.Run("it should run TestStatus", func(t *testing.T) {
				assert.Contains(t, stateRun, "TestStatus", stateOut)
			})

			t.Run("and it should not run TestReleaseAliases", func(t *testing.T) {
				assert.NotContains(t, stateRun, "TestReleaseAliases")
			})
		})

		gitIn(t, dir, "checkout", "--", "pkg/state/state.go")
		status := filepath.Join(dir, "testing", "acceptance", "status_test.go")
		text, err := os.ReadFile(status)
		require.NoError(t, err)
		probed := strings.Replace(string(text), "func TestStatus(t *testing.T) {\n", "func TestStatus(t *testing.T) {\n\t_ = \"item 226 probe\"\n", 1)
		require.NotEqual(t, string(text), probed)
		require.NoError(t, os.WriteFile(status, []byte(probed), 0o644))
		ownOut := runTargets(t, dir, "changed", "-run", universe)
		ownRun := selected(ownOut)

		t.Run("and a line in TestStatus's own body is then changed", func(t *testing.T) {
			t.Run("it should run TestStatus", func(t *testing.T) {
				assert.Contains(t, ownRun, "TestStatus", ownOut)
			})

			t.Run("and it should not run TestReleaseAliases", func(t *testing.T) {
				assert.NotContains(t, ownRun, "TestReleaseAliases")
			})
		})

		appendTo(t, filepath.Join(dir, "tools", "release-aliases"), "\n# item 117 probe\n")
		toolsRun := runTargets(t, dir, "changed", "-run", universe)

		t.Run("and a file under tools/ is then changed", func(t *testing.T) {
			t.Run("it should fall back to a full run and say why", func(t *testing.T) {
				assert.Contains(t, toolsRun, "full run: tools/release-aliases is outside pkg/ and cmd/")
			})
		})
	})
}
