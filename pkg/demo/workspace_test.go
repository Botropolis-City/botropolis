package demo

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspacePrepare(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("needs git")
	}
	in, home := t.TempDir(), t.TempDir()
	realHome := t.TempDir()
	sentinel := `{"sentinel":true}` + "\n"
	writeFile(t, filepath.Join(realHome, ".claude.json"), sentinel)
	t.Setenv("HOME", realHome)
	writeFile(t, filepath.Join(in, "projects", "tidepool", "main.go"), "package main\n")
	writeFile(t, filepath.Join(in, "tracker", "tidepool.json"), "[]\n")
	writeFile(t, filepath.Join(in, "skills", "changelog", "SKILL.md"), "---\nname: changelog\n---\n")
	w := Workspace{Projects: filepath.Join(in, "projects"), Tracker: filepath.Join(in, "tracker"), Skills: filepath.Join(in, "skills"),
		Home: home, Src: filepath.Join(home, "src"), Self: "/opt/botropolis/botropolis-demo"}
	config, err := w.Prepare()
	require.NoError(t, err)

	t.Run("when the workspace is prepared", func(t *testing.T) {
		t.Run("it should make each project a git repository with one commit", func(t *testing.T) {
			out, err := exec.Command("git", "-C", filepath.Join(home, "src", "tidepool"), "rev-list", "--count", "HEAD").Output()
			require.NoError(t, err)
			assert.Equal(t, "1", strings.TrimSpace(string(out)))
		})

		t.Run("it should install the skills where claude looks for them", func(t *testing.T) {
			assert.FileExists(t, filepath.Join(home, ".claude", "skills", "changelog", "SKILL.md"))
		})

		t.Run("it should point the MCP servers at this binary", func(t *testing.T) {
			data, err := os.ReadFile(config)
			require.NoError(t, err)
			assert.Contains(t, string(data), `"command": "/opt/botropolis/botropolis-demo"`)
		})

		state := readJSONFile(t, filepath.Join(home, ".claude.json"))
		t.Run("it should mark onboarding done in the recorder's home", func(t *testing.T) {
			assert.Equal(t, true, state["hasCompletedOnboarding"])
		})
		t.Run("it should accept each toy project's folder trust in the recorder's home", func(t *testing.T) {
			projects, _ := state["projects"].(map[string]any)
			trust, _ := projects[filepath.Join(home, "src", "tidepool")].(map[string]any)
			assert.Equal(t, true, trust["hasTrustDialogAccepted"])
		})
		t.Run("it should skip the bypass-permissions warning in the recorder's home", func(t *testing.T) {
			assert.Equal(t, true, readJSONFile(t, filepath.Join(home, ".claude", "settings.json"))["skipDangerousModePermissionPrompt"])
		})
		t.Run("it should write nothing outside the recorder's home", func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(realHome, ".claude.json"))
			require.NoError(t, err)
			assert.Equal(t, sentinel, string(data))
		})
	})
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var v map[string]any
	require.NoError(t, json.Unmarshal(data, &v))
	return v
}
