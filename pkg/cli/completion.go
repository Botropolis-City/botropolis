package cli

import (
	"github.com/spf13/cobra"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// jobIDLength is how much of a session id is its background job id.
const jobIDLength = 8

// completeSessions offers, for a command's one id argument, the sessions
// pick accepts, each with its title. Without this a shell falls back to
// file names, which is worse than offering nothing.
func completeSessions(load Loader, services Services, pick func(state.Session) (string, bool)) cobra.CompletionFunc {
	return func(_ *cobra.Command, args []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		cfg, err := load()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		snapshot, err := services.Status(cfg).Snapshot(false)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var out []cobra.Completion
		for _, s := range snapshot.Sessions {
			if id, ok := pick(s); ok {
				out = append(out, cobra.CompletionWithDesc(id, s.Title))
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

func jobID(s state.Session) string {
	if len(s.ID) < jobIDLength {
		return s.ID
	}
	return s.ID[:jobIDLength]
}

func liveBackground(s state.Session) (string, bool) {
	return jobID(s), s.Kind == claude.KindBackground && s.Alive
}

// liveOrResting is what attach can open: a live background job by its id,
// or a resting session by its own, which attach resumes.
func liveOrResting(s state.Session) (string, bool) {
	if id, ok := liveBackground(s); ok {
		return id, true
	}
	return parked(s)
}

func anyBackground(s state.Session) (string, bool) {
	return jobID(s), s.Kind == claude.KindBackground
}

func parked(s state.Session) (string, bool) {
	return s.ID, s.State == state.Resting
}

func completeDirThenNothing(_ *cobra.Command, args []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveFilterDirs
}
