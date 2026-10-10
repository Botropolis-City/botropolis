package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/state"
)

type SessionsRunner interface {
	New(ctx context.Context, dir, prompt string) (string, error)
	Attach(id string) error
	Stop(ctx context.Context, id string) error
	Remove(ctx context.Context, id string) error
	Resume(ctx context.Context, dir, sessionID string) (string, error)
	Prune(ctx context.Context, snapshot state.Snapshot, olderThan time.Duration, now time.Time, dryRun bool) ([]commands.Pruned, error)
}

func NewSessionCLIs(load Loader, services Services) []*cobra.Command {
	sessions := func() (SessionsRunner, error) {
		cfg, err := load()
		if err != nil {
			return nil, err
		}
		return services.Sessions(cfg), nil
	}
	newCmd := &cobra.Command{
		Use:   "new <dir> [prompt...]",
		Short: "Start a background session in <dir> and print its job id",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := sessions()
			if err != nil {
				return err
			}
			id, err := s.New(cmd.Context(), args[0], strings.Join(args[1:], " "))
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), id)
			return nil
		},
	}
	attach := &cobra.Command{
		Use:   "attach <id>",
		Short: "Open a terminal on a background session",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			s, err := sessions()
			if err != nil {
				return err
			}
			return s.Attach(args[0])
		},
	}
	stop := &cobra.Command{
		Use:   "stop <id>",
		Short: "Stop a background session; its conversation is kept",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := sessions()
			if err != nil {
				return err
			}
			return s.Stop(cmd.Context(), args[0])
		},
	}
	rm := &cobra.Command{
		Use:   "rm <id>",
		Short: "Delete a background session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := sessions()
			if err != nil {
				return err
			}
			return s.Remove(cmd.Context(), args[0])
		},
	}
	resume := &cobra.Command{
		Use:   "resume <session-id>",
		Short: "Resume a parked session in the background and attach to it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := sessions()
			if err != nil {
				return err
			}
			dir, _ := cmd.Flags().GetString("dir")
			id, err := s.Resume(cmd.Context(), dir, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), id)
			return nil
		},
	}
	resume.Flags().String("dir", "", "directory to resume in (default: the session's own cwd)")
	prune := &cobra.Command{
		Use:   "prune",
		Short: "Delete parked background sessions older than a cutoff, and empty ones that sat for an hour",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			olderThan, _ := cmd.Flags().GetDuration("older-than")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			snapshot, err := services.Status(cfg).Snapshot(true)
			if err != nil {
				return err
			}
			pruned, err := services.Sessions(cfg).Prune(cmd.Context(), snapshot, olderThan, time.Now(), dryRun)
			verb := "removed"
			if dryRun {
				verb = "would remove"
			}
			for _, p := range pruned {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s  %s  (last seen %s)\n", verb, p.ID, p.Title, p.LastSeen.Format("2006-01-02"))
			}
			if err != nil {
				return err
			}
			if len(pruned) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "nothing to prune")
			}
			return nil
		},
	}
	prune.Flags().Duration("older-than", 7*24*time.Hour, "prune sessions with no activity for this long")
	prune.Flags().Bool("dry-run", false, "list what would be removed and change nothing")
	return []*cobra.Command{newCmd, attach, stop, rm, resume, prune}
}
