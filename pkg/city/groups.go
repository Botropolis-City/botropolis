package city

import (
	"slices"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/state"
)

// placeGroups hands each building its group (item 87), and its station:
// one per project the group runs through, in the order the group's
// first session there started.
func (c *City) placeGroups(groups []state.Group) {
	byID := map[string]*state.Group{}
	for i := range groups {
		byID[groups[i].ID] = &groups[i]
	}
	first := map[string]map[string]time.Time{}
	for _, b := range c.Buildings() {
		b.Group = byID[b.Session.Group]
		if b.Group == nil {
			continue
		}
		sites := first[b.Group.ID]
		if sites == nil {
			sites = map[string]time.Time{}
			first[b.Group.ID] = sites
		}
		root := ProjectRoot(b.Session.CWD)
		if at, ok := sites[root]; !ok || b.Session.StartedAt.Before(at) {
			sites[root] = b.Session.StartedAt
		}
	}
	order := map[string][]string{}
	for id, sites := range first {
		roots := make([]string, 0, len(sites))
		for root := range sites {
			roots = append(roots, root)
		}
		slices.SortFunc(roots, func(a, b string) int {
			if c := sites[a].Compare(sites[b]); c != 0 {
				return c
			}
			return strings.Compare(a, b)
		})
		order[id] = roots
	}
	for _, b := range c.Buildings() {
		if b.Group == nil {
			continue
		}
		roots := order[b.Group.ID]
		b.Station, b.Stations = slices.Index(roots, ProjectRoot(b.Session.CWD))+1, len(roots)
	}
}
