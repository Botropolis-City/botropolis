package city

import (
	"strings"

	"botropolis.city/botropolis/pkg/state"
)

// findGroupsShown is the most groups the finder lists for one query.
const findGroupsShown = 3

// GroupsMatching are the groups whose name, short name or anchor holds
// every word of a query, ignoring case, in the order given, which is
// most recently active first; at most three (item 90).
func GroupsMatching(groups []state.Group, query string) []state.Group {
	words := strings.Fields(strings.ToLower(query))
	out := []state.Group{}
	if len(words) == 0 {
		return out
	}
	for _, g := range groups {
		text := strings.ToLower(g.Name + " " + g.Short + " " + g.Anchor)
		all := true
		for _, w := range words {
			all = all && strings.Contains(text, w)
		}
		if all {
			out = append(out, g)
		}
		if len(out) == findGroupsShown {
			break
		}
	}
	return out
}
