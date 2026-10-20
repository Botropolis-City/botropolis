package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/state"
)

type SessionsRunner interface {
	New(ctx context.Context, dir, prompt string) (string, error)
	Open(ctx context.Context, id, dir string, here bool) error
	Stop(ctx context.Context, id string) error
	Remove(ctx context.Context, id string) error
	Prune(ctx context.Context, snapshot state.Snapshot, olderThan time.Duration, now time.Time, dryRun bool) ([]commands.Pruned, error)
}

// defaultPruneAge is what prune, deprecated, cleared by default.
const defaultPruneAge = 7 * 24 * time.Hour

// NewSessionCLIs are the session verbs. isTerminal says whether the
// command runs in a terminal, where attach attaches rather than opening
// one.
func NewSessionCLIs(load Loader, services Services, isTerminal func(*cobra.Command) bool) []*cobra.Command {
	sessions := func() (SessionsRunner, error) {
		cfg, err := load()
		if err != nil {
			return nil, err
		}
		return services.Sessions(cfg), nil
	}
	newCmd := &cobra.Command{
		Use:   "new <dir> [prompt...]",
		Short: "Start a session in the background and print its id",
		Long: "Start a session in the background in <dir>, optionally with a first prompt,\n" +
			"and print its id. A plain claude does this for you once shell-init is in your\nrc file.",
		Args: cobra.MinimumNArgs(1),
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
	open := func(cmd *cobra.Command, id string, here bool) error {
		s, err := sessions()
		if err != nil {
			return err
		}
		dir, _ := cmd.Flags().GetString("dir")
		return s.Open(cmd.Context(), id, dir, here)
	}
	attach := &cobra.Command{
		Use:   "attach <id>",
		Short: "Open a session in this terminal, resuming it if it is resting",
		Long: "Open a session. In a terminal it attaches there; from anywhere else, or with\n" +
			"--new-window, it opens a terminal. A resting session is resumed first, in its\n" +
			"own directory unless --dir says otherwise.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			newWindow, _ := cmd.Flags().GetBool("new-window")
			return open(cmd, args[0], isTerminal(cmd) && !newWindow)
		},
	}
	attach.Flags().String("dir", "", "resume a resting session in this `dir`")
	attach.Flags().Bool("new-window", false, "open a new terminal even when run from one")
	resume := deprecate(&cobra.Command{
		Use:   "resume <session-id>",
		Short: "Resume a resting session in the background and attach to it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return open(cmd, args[0], false)
		},
	}, "attach")
	resume.Flags().String("dir", "", "directory to resume in (default: the session's own cwd)")
	stop := &cobra.Command{
		Use:   "stop <id>",
		Short: "Stop a session; its conversation is kept",
		Long:  "Stop a session. Its conversation is kept, and attach brings it back.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := sessions()
			if err != nil {
				return err
			}
			return s.Stop(cmd.Context(), args[0])
		},
	}
	prune := func(cmd *cobra.Command, olderThan time.Duration, dryRun bool) error {
		cfg, err := load()
		if err != nil {
			return err
		}
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
	}
	rm := &cobra.Command{
		Use:   "rm <id>... | --older-than <duration>",
		Short: "Delete a session, or every resting one older than a cutoff",
		Long: "Delete a session, or with --older-than every resting session with no\n" +
			"activity for that long and every empty one that sat for an hour.",
		RunE: func(cmd *cobra.Command, args []string) error {
			olderThan, _ := cmd.Flags().GetDuration("older-than")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			switch {
			case olderThan > 0 && len(args) > 0:
				return errors.New("rm takes ids or --older-than, not both")
			case olderThan > 0:
				return prune(cmd, olderThan, dryRun)
			case len(args) == 0:
				return errors.New("rm needs an id, or --older-than")
			}
			if dryRun {
				for _, id := range args {
					fmt.Fprintln(cmd.OutOrStdout(), "would remove "+id)
				}
				return nil
			}
			s, err := sessions()
			if err != nil {
				return err
			}
			for _, id := range args {
				if err := s.Remove(cmd.Context(), id); err != nil {
					return err
				}
			}
			return nil
		},
	}
	rm.Flags().Duration("older-than", 0, "delete every resting session quiet for this long, e.g. 168h")
	rm.Flags().Bool("dry-run", false, "list what would be deleted and change nothing")
	pruneCmd := deprecate(&cobra.Command{
		Use:   "prune",
		Short: "Delete resting background sessions older than a cutoff, and empty ones that sat for an hour",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			olderThan, _ := cmd.Flags().GetDuration("older-than")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			return prune(cmd, olderThan, dryRun)
		},
	}, "rm --older-than")
	pruneCmd.Flags().Duration("older-than", defaultPruneAge, "prune sessions with no activity for this long")
	pruneCmd.Flags().Bool("dry-run", false, "list what would be removed and change nothing")
	newCmd.ValidArgsFunction = completeDirThenNothing
	attach.ValidArgsFunction = completeSessions(load, services, liveOrResting)
	stop.ValidArgsFunction = completeSessions(load, services, liveBackground)
	rm.ValidArgsFunction = completeSessions(load, services, anyBackground)
	resume.ValidArgsFunction = completeSessions(load, services, parked)
	return []*cobra.Command{newCmd, attach, stop, rm, resume, pruneCmd}
}
