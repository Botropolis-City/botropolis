package state

import (
	"fmt"

	"botropolis.city/botropolis/pkg/claude"
)

// Move is whose move a group's work is (item 87).
type Move string

const (
	MoveYours  Move = "yours"
	MoveAgents Move = "the agents'"
	MoveTheirs Move = "someone else's"
	MoveNobody Move = "nobody's"
)

// GroupMove is the most urgent move among a group's members, and why.
func GroupMove(members []Session) (Move, string) {
	for _, s := range members {
		if s.State == NeedsYou {
			return MoveYours, title(s) + " needs you"
		}
	}
	for _, s := range members {
		switch s.State {
		case Working, Unattended, Waiting:
			return MoveAgents, title(s) + " is working"
		}
	}
	prs := 0
	for _, s := range members {
		for _, pr := range s.PRs {
			prs++
			if pr.State == "" || pr.State == claude.PROpen {
				return MoveTheirs, fmt.Sprintf("PR #%d open in %s", pr.Number, pr.Repository)
			}
		}
	}
	if prs > 0 {
		return MoveNobody, "done: every PR merged or closed"
	}
	return MoveNobody, "nothing open"
}

func title(s Session) string {
	if s.Title != "" {
		return s.Title
	}
	return ShortID(s.ID)
}
