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
		if strings.HasPrefix(line, "selected:") {
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
		data, err := os.ReadFile(filepath.Join(dir, ".cache", "acceptance-map.json"))
		require.NoError(t, err)
		var m targets.Map
		require.NoError(t, json.Unmarshal(data, &m))

		t.Run("it should map TestStatus to pkg/state", func(t *testing.T) {
			assert.Contains(t, m.Tests["TestStatus"], "botropolis.city/botropolis/pkg/state")
		})

		appendTo(t, filepath.Join(dir, "pkg", "state", "state.go"), "\n// item 117 probe\n")
		stateRun := selected(runTargets(t, dir, "changed", "-run", universe))

		t.Run("and a file under pkg/state is then changed", func(t *testing.T) {
			t.Run("it should run TestStatus", func(t *testing.T) {
				assert.Contains(t, stateRun, "TestStatus")
			})

			t.Run("and it should not run TestReleaseAliases", func(t *testing.T) {
				assert.NotContains(t, stateRun, "TestReleaseAliases")
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
