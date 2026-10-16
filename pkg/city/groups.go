package city

import "botropolis.city/botropolis/pkg/state"

// placeGroups hands each building its group (item 87).
func (c *City) placeGroups(groups []state.Group) {
	byID := map[string]*state.Group{}
	for i := range groups {
		byID[groups[i].ID] = &groups[i]
	}
	for _, b := range c.Buildings() {
		b.Group = byID[b.Session.Group]
	}
}
