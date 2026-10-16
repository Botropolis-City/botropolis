package city

import "botropolis.city/botropolis/pkg/state"

// placeGroups hands each building its group (item 87), draws the
// groups' lines, and tells each building which station on its line its
// district is.
func (c *City) placeGroups(groups []state.Group) {
	byID := map[string]*state.Group{}
	c.groups = nil
	for i := range groups {
		byID[groups[i].ID] = &groups[i]
		c.groups = append(c.groups, &groups[i])
	}
	for _, b := range c.Buildings() {
		b.Group = byID[b.Session.Group]
		b.Station, b.Stations = 0, 0
	}
	c.placeLines()
	for _, line := range c.Lines {
		for i, st := range line.Stations {
			for _, b := range st.District.Buildings {
				if b.Group == line.Group {
					b.Station, b.Stations = i+1, len(line.Stations)
				}
			}
		}
	}
}
