package city

import (
	"fmt"
	"strings"

	"botropolis.city/botropolis/pkg/state"
)

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
		b.Suggested = nil
		if b.Group == nil && b.Session.Suggested != "" {
			b.Suggested = byID[b.Session.Suggested]
		}
		b.Station, b.Stations, b.Waits = 0, 0, nil
	}
	c.placeLines()
	for _, line := range c.Lines {
		if !line.Leads() {
			continue
		}
		for i, st := range line.Stations {
			if st.Teammate != nil {
				continue
			}
			for _, b := range st.District.Buildings {
				if b.Group == line.Group {
					b.Station, b.Stations, b.Waits = i+1, len(line.Stations), st.Waits
				}
			}
		}
	}
}

// MemberList names up to n of a group's members, and how many more
// there are (item 333).
func MemberList(titles []string, n int) string {
	if len(titles) <= n {
		return strings.Join(titles, ", ")
	}
	return fmt.Sprintf("%s +%d more", strings.Join(titles[:n], ", "), len(titles)-n)
}
