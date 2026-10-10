package commands

import (
	"fmt"
	"io"
	"path/filepath"

	"botropolis.city/botropolis/pkg/hooks"
)

type Hooks struct {
	settings string
	command  string
}

func NewHooks(home, command string) *Hooks {
	return &Hooks{settings: filepath.Join(home, ".claude", "settings.json"), command: command}
}

func (h *Hooks) Install(out io.Writer) error {
	changed, err := hooks.Install(h.settings, h.command)
	if err != nil {
		return err
	}
	if !changed {
		fmt.Fprintf(out, "%s: nothing to change\n", h.settings)
		return nil
	}
	fmt.Fprintf(out, "%s: botropolis hooks installed for %d events (backup at %s%s)\n",
		h.settings, len(hooks.Events), h.settings, hooks.BackupSuffix)
	return nil
}

func (h *Hooks) Remove(out io.Writer) error {
	changed, err := hooks.Remove(h.settings)
	if err != nil {
		return err
	}
	if !changed {
		fmt.Fprintf(out, "%s: nothing to change\n", h.settings)
		return nil
	}
	fmt.Fprintf(out, "%s: botropolis hooks removed (backup at %s%s)\n", h.settings, h.settings, hooks.BackupSuffix)
	return nil
}

func (h *Hooks) DryRun(out io.Writer) {
	fmt.Fprintf(out, "would merge into %s:\n%s\n", h.settings, hooks.Render(h.command))
}
