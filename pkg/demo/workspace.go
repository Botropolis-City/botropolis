package demo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Workspace is what the recorder container starts from: the toy
// projects, the tracker's issues and the skills, all copied out of the
// read-only mount into the demo user's home.
type Workspace struct {
	Projects, Tracker, Skills string
	Home, Src                 string
	// Self is this binary, which serves the MCP servers too.
	Self string
}

// Prepare lays the workspace out and returns the MCP config to record
// with. A project already in Src is left as it is, so a re-run keeps
// whatever the sessions recorded so far did to it.
func (w Workspace) Prepare() (string, error) {
	projects, err := os.ReadDir(w.Projects)
	if err != nil {
		return "", err
	}
	for _, p := range projects {
		if !p.IsDir() {
			continue
		}
		dst := filepath.Join(w.Src, p.Name())
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		if err := copyTree(filepath.Join(w.Projects, p.Name()), dst); err != nil {
			return "", err
		}
		if err := gitInit(dst); err != nil {
			return "", fmt.Errorf("%s: %w", p.Name(), err)
		}
	}
	if err := w.settleFirstRun(projects); err != nil {
		return "", err
	}
	tracker := filepath.Join(w.Home, "tracker")
	if _, err := os.Stat(tracker); os.IsNotExist(err) {
		if err := copyTree(w.Tracker, tracker); err != nil {
			return "", err
		}
	}
	if err := copyTree(w.Skills, filepath.Join(w.Home, ".claude", "skills")); err != nil {
		return "", err
	}
	config := filepath.Join(w.Home, "mcp.json")
	return config, writeJSON(config, map[string]any{"mcpServers": map[string]any{
		"tracker": map[string]any{"type": "stdio", "command": w.Self, "args": []string{"mcp", "tracker", "--data", tracker}},
		"weather": map[string]any{"type": "stdio", "command": w.Self, "args": []string{"mcp", "weather"}},
	}})
}

// settleFirstRun answers, in the recorder's own home, the dialogs an
// interactive claude opens on before its prompt: onboarding, each toy
// project's folder trust, and the bypass-permissions warning. A -p run
// shows none of them, but a team is recorded interactively (item 105).
// Whatever claude already keeps in that home is left as it is.
func (w Workspace) settleFirstRun(projects []os.DirEntry) error {
	path := filepath.Join(w.Home, ".claude.json")
	state := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &state); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	state["hasCompletedOnboarding"] = true
	trusted, _ := state["projects"].(map[string]any)
	if trusted == nil {
		trusted = map[string]any{}
	}
	for _, p := range projects {
		if !p.IsDir() {
			continue
		}
		dir := filepath.Join(w.Src, p.Name())
		entry, _ := trusted[dir].(map[string]any)
		if entry == nil {
			entry = map[string]any{}
		}
		entry["hasTrustDialogAccepted"] = true
		trusted[dir] = entry
	}
	state["projects"] = trusted
	if err := writeJSON(path, state); err != nil {
		return err
	}
	return writeJSON(filepath.Join(w.Home, ".claude", "settings.json"), map[string]any{"skipDangerousModePermissionPrompt": true})
}

func gitInit(dir string) error {
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.name", "Demo Developer"},
		{"config", "user.email", "dev@example.invalid"},
		{"config", "commit.gpgsign", "false"},
		{"add", "-A"},
		{"commit", "-q", "-m", "Initial import"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %w: %s", args, err, out)
		}
	}
	return nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm()|0o600)
	})
}
