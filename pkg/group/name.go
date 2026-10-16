package group

import (
	"path/filepath"
	"sort"
	"strings"
)

const nameWords = 2

// name gives a group its anchor, the reference most of its members
// share, and a name made of the anchor and its members' most
// distinctive words, in the order they were first typed.
func (d *detection) name(g *Group) {
	count := map[string]int{}
	ids := make([]string, 0, len(g.Members))
	var text strings.Builder
	refTokens := map[string]bool{}
	for _, m := range g.Members {
		i := d.index[m.Session]
		ids = append(ids, m.Session)
		for _, r := range d.refs[i] {
			count[r]++
			refTokens[strings.ToLower(r)] = true
		}
		for _, p := range d.members[i].Prompts {
			text.WriteString(" ")
			text.WriteString(p)
		}
	}
	g.Anchor = ""
	for r, c := range count {
		if c >= 2 && (g.Anchor == "" || c > count[g.Anchor] || (c == count[g.Anchor] && r < g.Anchor)) {
			g.Anchor = r
		}
	}
	var words []string
	for _, w := range d.wording.Top(ids, nameWords+len(refTokens)) {
		if !refTokens[w] && len(words) < nameWords {
			words = append(words, w)
		}
	}
	typed := tokens(text.String())
	first := func(w string) int {
		for i, t := range typed {
			if t == w {
				return i
			}
		}
		return len(typed)
	}
	sort.SliceStable(words, func(i, j int) bool { return first(words[i]) < first(words[j]) })
	var parts []string
	if g.Anchor != "" {
		parts = append(parts, g.Anchor)
	}
	if len(words) > 0 {
		parts = append(parts, strings.Join(words, " "))
	}
	if len(parts) == 0 {
		parts = append(parts, filepath.Base(g.Dirs()[0]))
	}
	g.Name = strings.Join(parts, " · ")
}
