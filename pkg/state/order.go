package state

import "strings"

// Order is every state by urgency. It is the one place the order lives:
// the strip, the window title, the bar and the TUI rows all walk it, so
// they cannot disagree.
var Order = []State{NeedsYou, Working, Waiting, Unattended, Resting}

// Rank is a state's place in Order, for sorting; unknown states sort last.
func Rank(st State) int {
	for i, o := range Order {
		if o == st {
			return i
		}
	}
	return len(Order)
}

// Live is the filter for states with a conversation going right now:
// not parked, and not empty (an empty session is never counted).
func Live(st State) bool { return st != Resting && st != Empty }

// Count is how many sessions are in a state.
type Count struct {
	State State
	N     int
}

// Tally counts sessions by state.
func Tally(sessions []Session) map[State]int {
	counts := map[State]int{}
	for _, s := range sessions {
		counts[s.State]++
	}
	return counts
}

// Nonzero walks counts in Order, keeping the states that pass every
// filter and have at least one session; a zero drops out but its slot
// stays where it was.
func Nonzero(counts map[State]int, filters ...func(State) bool) []Count {
	var out []Count
next:
	for _, st := range Order {
		if counts[st] == 0 {
			continue
		}
		for _, keep := range filters {
			if !keep(st) {
				continue next
			}
		}
		out = append(out, Count{State: st, N: counts[st]})
	}
	return out
}

// LongestWaiting orders sessions by how long they have waited, oldest
// first, with ties broken by ID. Triage and the bar both sort with it, so
// they agree about who is first in line (item 174).
func LongestWaiting(a, b Session) int {
	if c := a.LastActivity.Compare(b.LastActivity); c != 0 {
		return c
	}
	return strings.Compare(a.ID, b.ID)
}
