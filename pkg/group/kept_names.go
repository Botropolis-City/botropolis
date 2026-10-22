package group

import "strings"

// keptNames keeps each group's anchor and name from run to run, by the
// sessions in it, so that a prompt names again only the groups it can
// have renamed (item 198). A name is read from its members' references,
// prompts and words, and from how many sessions use each of those
// words, so it is kept while no member changed, no word a member uses
// was counted again, and the number of sessions did not change.
type keptNames struct {
	last, next map[string]named
	changed    map[string]bool
}

type named struct {
	anchor, name string
}

func (n *keptNames) name(d *detection, g *Group) {
	if n == nil {
		d.name(g)
		return
	}
	ids := make([]string, len(g.Members))
	for i, m := range g.Members {
		ids[i] = m.Session
	}
	key := strings.Join(ids, "\x00")
	if kept, ok := n.last[key]; ok && n.still(d, ids) {
		g.Anchor, g.Name = kept.anchor, kept.name
	} else {
		d.name(g)
	}
	n.next[key] = named{anchor: g.Anchor, name: g.Name}
}

// still is whether a name read from these sessions would read the same
// now. When the number of sessions changes every session is changed.
func (n *keptNames) still(d *detection, ids []string) bool {
	for _, id := range ids {
		if n.changed[id] || usesAny(d.wording.words[id], d.wording.touched) {
			return false
		}
	}
	return true
}
