package group

import (
	"path/filepath"
	"sort"
	"strings"
)

const (
	nameWords  = 2
	shortRunes = 12
)

// Short is the group's name as a badge carries it: its anchor, or the
// first word of its name, cut to fit.
func Short(g Group) string {
	short := g.Anchor
	if short == "" {
		if fields := strings.Fields(g.Name); len(fields) > 0 {
			short = fields[0]
		}
	}
	if r := []rune(short); len(r) > shortRunes {
		short = string(r[:shortRunes]) + "…"
	}
	return short
}

// Describe says why a session joined, naming the session it matched by
// title.
func (r Reason) Describe(title func(id string) string) string {
	with := ""
	if r.With != "" {
		with = title(r.With)
	}
	switch r.Signal {
	case SignalStartedFrom:
		return "started from " + r.Detail
	case SignalReference:
		return "mentions " + r.Detail
	case SignalParentage:
		return "works with " + with
	case SignalPasted:
		return "pasted the same text as " + with
	case SignalWording:
		return `shares "` + r.Detail + `" with ` + with
	case SignalBranch:
		return "its branch shares " + r.Detail + " with " + with
	case SignalFiles:
		return "edits " + filepath.Base(r.Detail) + " like " + with
	case SignalTiming:
		return "started with " + with
	case SignalContinuing:
		return "continued from " + with
	}
	return string(r.Signal)
}

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
