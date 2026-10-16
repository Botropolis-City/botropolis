package state

import (
	"sort"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/group"
)

// GuessMembers are the sessions New session weighs a new one against
// (item 87): those detection groups, with every session still open
// counted as left at its last prompt, so having just been in one is the
// timing signal it would be had it closed.
func GuessMembers(history map[string][]claude.Prompt, sessions []Session, now time.Time) []group.Member {
	ms := members(history, sessions, now)
	for i := range ms {
		prompts := history[ms[i].ID]
		if ms[i].Ended.IsZero() && len(prompts) > 0 {
			ms[i].Ended = prompts[len(prompts)-1].At
		}
	}
	return ms
}

// RecentDirs are the directories prompted in, each once, the most
// recently prompted in first.
func RecentDirs(history map[string][]claude.Prompt) []string {
	last := map[string]time.Time{}
	for _, prompts := range history {
		for _, p := range prompts {
			if p.Dir != "" && p.At.After(last[p.Dir]) {
				last[p.Dir] = p.At
			}
		}
	}
	dirs := make([]string, 0, len(last))
	for d := range last {
		dirs = append(dirs, d)
	}
	sort.Slice(dirs, func(i, j int) bool {
		if !last[dirs[i]].Equal(last[dirs[j]]) {
			return last[dirs[i]].After(last[dirs[j]])
		}
		return dirs[i] < dirs[j]
	})
	return dirs
}
