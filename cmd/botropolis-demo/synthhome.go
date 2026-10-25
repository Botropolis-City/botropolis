package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"botropolis.city/botropolis/pkg/demo"
)

func synthHomeCommand() *cobra.Command {
	var home, end string
	var sessions, projects int
	var seed int64
	cmd := &cobra.Command{
		Use:   "synth-home",
		Short: "Write a synthetic Claude history of a given size, for a machine that must never see a real one",
		Long: "Writes a made-up history into --home/.claude: --sessions small sessions\n" +
			"in exactly --projects projects, started across the 30 days before --end,\n" +
			"each with a transcript and its prompts in history.jsonl, all from --seed,\n" +
			"so the same seed writes the same home. It is the generator the cost tests\n" +
			"use. It refuses a --home that already has a .claude, and one whose .claude\n" +
			"would be inside the real one: ~/.claude, $HOME/.claude or $CLAUDE_CONFIG_DIR.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			at := time.Now().UTC()
			if end != "" {
				t, err := time.Parse(time.RFC3339, end)
				if err != nil {
					return fmt.Errorf("--end: %w", err)
				}
				at = t.UTC()
			}
			if err := demo.RefuseRealClaude(home, demo.RealClaudeDirs()); err != nil {
				return err
			}
			if _, err := os.Lstat(filepath.Join(home, ".claude")); !errors.Is(err, fs.ErrNotExist) {
				if err != nil {
					return err
				}
				return fmt.Errorf("refusing %s: it already has a .claude, and a second history would be added to the first", home)
			}
			history, err := demo.SynthHistory(seed, sessions, projects, at)
			if err != nil {
				return err
			}
			written, err := demo.WriteHome(home, history, "2006-01-02T15:04:05.000Z")
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %d sessions in %d projects, %d prompts, to %s\n",
				written.Sessions, written.Projects, written.Prompts, filepath.Join(home, ".claude"))
			return nil
		},
	}
	cmd.Flags().StringVar(&home, "home", "", "the directory to write the .claude into")
	cmd.Flags().IntVar(&sessions, "sessions", 200, "how many sessions")
	cmd.Flags().IntVar(&projects, "projects", 150, "how many projects they work in")
	cmd.Flags().Int64Var(&seed, "seed", 248, "the seed the history is made from")
	cmd.Flags().StringVar(&end, "end", "", "the history's last day, RFC 3339 (default now)")
	_ = cmd.MarkFlagRequired("home")
	return cmd
}
