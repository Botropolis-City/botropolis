package hooks_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/pkg/hooks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const existing = `{
  "model": "opus[1m]",
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "/usr/bin/other-hook"}]}
    ],
    "Stop": [
      {"hooks": [{"type": "command", "command": "botropolis-hook"}]}
    ]
  }
}
`

func settingsPath(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if content != "" {
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	return path
}

func readSettings(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))
	return settings
}

func commandsFor(t *testing.T, settings map[string]any, event string) []string {
	t.Helper()
	var commands []string
	hooksByEvent, _ := settings["hooks"].(map[string]any)
	entries, _ := hooksByEvent[event].([]any)
	for _, entry := range entries {
		group, _ := entry.(map[string]any)
		inner, _ := group["hooks"].([]any)
		for _, h := range inner {
			hook, _ := h.(map[string]any)
			commands = append(commands, hook["command"].(string))
		}
	}
	return commands
}

func TestInstall(t *testing.T) {
	t.Run("when there is no settings file", func(t *testing.T) {
		path := settingsPath(t, "")
		changed, err := hooks.Install(path, "botropolis-hook")
		require.NoError(t, err)
		settings := readSettings(t, path)

		t.Run("it should report a change", func(t *testing.T) {
			assert.True(t, changed)
		})

		t.Run("it should register the hook for every event", func(t *testing.T) {
			for _, event := range hooks.Events {
				assert.Equal(t, []string{"botropolis-hook"}, commandsFor(t, settings, event), event)
			}
		})

		t.Run("it should give the hook a short timeout", func(t *testing.T) {
			entries := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
			hook := entries[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)
			assert.Equal(t, float64(hooks.TimeoutSeconds), hook["timeout"])
		})

		t.Run("it should create the file privately", func(t *testing.T) {
			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		})
	})

	t.Run("when the settings file already has other hooks and one of ours", func(t *testing.T) {
		path := settingsPath(t, existing)
		changed, err := hooks.Install(path, "botropolis-hook")
		require.NoError(t, err)
		settings := readSettings(t, path)

		t.Run("it should report a change", func(t *testing.T) {
			assert.True(t, changed)
		})

		t.Run("it should keep unrelated settings", func(t *testing.T) {
			assert.Equal(t, "opus[1m]", settings["model"])
		})

		t.Run("it should keep the other hook", func(t *testing.T) {
			assert.Equal(t, []string{"/usr/bin/other-hook", "botropolis-hook"}, commandsFor(t, settings, "PreToolUse"))
		})

		t.Run("it should not duplicate the one already present", func(t *testing.T) {
			assert.Equal(t, []string{"botropolis-hook"}, commandsFor(t, settings, "Stop"))
		})

		t.Run("it should back up the original", func(t *testing.T) {
			backup, err := os.ReadFile(path + hooks.BackupSuffix)
			require.NoError(t, err)
			assert.Equal(t, existing, string(backup))
		})

		t.Run("and it runs again", func(t *testing.T) {
			changed, err := hooks.Install(path, "botropolis-hook")
			require.NoError(t, err)

			t.Run("it should report no change", func(t *testing.T) {
				assert.False(t, changed)
			})
		})
	})

	t.Run("when ours is installed under a different command", func(t *testing.T) {
		path := settingsPath(t, existing)
		_, err := hooks.Install(path, "/home/avesta/workspaces/github/auroq/botropolis/bin/botropolis-hook")
		require.NoError(t, err)
		changed, err := hooks.Install(path, "/usr/bin/botropolis-hook")
		require.NoError(t, err)
		settings := readSettings(t, path)

		t.Run("it should report a change", func(t *testing.T) {
			assert.True(t, changed)
		})

		t.Run("it should re-point every event at the new command", func(t *testing.T) {
			for _, event := range hooks.Events {
				assert.Contains(t, commandsFor(t, settings, event), "/usr/bin/botropolis-hook", event)
			}
		})

		t.Run("it should not keep the old command anywhere", func(t *testing.T) {
			for _, event := range hooks.Events {
				assert.Len(t, commandsFor(t, settings, event), map[bool]int{true: 2, false: 1}[event == "PreToolUse"], event)
			}
		})
	})

	t.Run("when the settings file is not valid JSON", func(t *testing.T) {
		path := settingsPath(t, `{"model":`)
		_, err := hooks.Install(path, "botropolis-hook")

		t.Run("it should return an error", func(t *testing.T) {
			assert.Error(t, err)
		})

		t.Run("it should leave the file alone", func(t *testing.T) {
			data, _ := os.ReadFile(path)
			assert.Equal(t, `{"model":`, string(data))
		})
	})
}

func TestRemove(t *testing.T) {
	t.Run("when our hooks are installed beside another", func(t *testing.T) {
		path := settingsPath(t, existing)
		_, err := hooks.Install(path, "botropolis-hook")
		require.NoError(t, err)
		changed, err := hooks.Remove(path)
		require.NoError(t, err)
		settings := readSettings(t, path)

		t.Run("it should report a change", func(t *testing.T) {
			assert.True(t, changed)
		})

		t.Run("it should keep the other hook", func(t *testing.T) {
			assert.Equal(t, []string{"/usr/bin/other-hook"}, commandsFor(t, settings, "PreToolUse"))
		})

		t.Run("it should drop events that only had ours", func(t *testing.T) {
			assert.NotContains(t, settings["hooks"].(map[string]any), "Stop")
		})

		t.Run("it should keep unrelated settings", func(t *testing.T) {
			assert.Equal(t, "opus[1m]", settings["model"])
		})
	})

	t.Run("when nothing of ours is installed", func(t *testing.T) {
		path := settingsPath(t, `{"model":"opus[1m]"}`)
		changed, err := hooks.Remove(path)
		require.NoError(t, err)

		t.Run("it should report no change", func(t *testing.T) {
			assert.False(t, changed)
		})
	})
}

func TestRender(t *testing.T) {
	t.Run("when rendering the hook block", func(t *testing.T) {
		rendered := hooks.Render("botropolis-hook")

		t.Run("it should be valid JSON with every event", func(t *testing.T) {
			var block map[string]any
			require.NoError(t, json.Unmarshal([]byte(rendered), &block))
			assert.Len(t, block, len(hooks.Events))
		})
	})
}
