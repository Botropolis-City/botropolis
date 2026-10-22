package group

import (
	"slices"
	"time"
)

// Orders reads groups' orders and remembers them, so a group whose
// sites say nothing new is not read again (item 197). Only a site's
// being done can change without its words changing, and a wait on a
// site being done is the one wait that follows it, so that is
// recomputed on every reading from what is remembered.
type Orders struct {
	last, next map[string]remembered
	// Reads is how many orders have been read rather than remembered.
	Reads int
}

type remembered struct {
	sites []reading
	order []int
	waits []Wait
}

// reading is what a site's order is read from, without whether it is
// done.
type reading struct {
	name     string
	started  time.Time
	prs      []SitePR
	words    []string
	commands []string
	recap    string
}

func readingOf(s OrderSite) reading {
	return reading{name: s.Name, started: s.Started, prs: s.PRs, words: s.Words, commands: s.Commands, recap: s.Recap}
}

func (r reading) equal(o reading) bool {
	return r.name == o.name && r.started.Equal(o.started) && r.recap == o.recap &&
		slices.Equal(r.prs, o.prs) && slices.Equal(r.words, o.words) && slices.Equal(r.commands, o.commands)
}

// Order is Order(sites) for the group key names, read again only when
// its sites changed since the last time it was asked.
func (o *Orders) Order(key string, sites []OrderSite) ([]int, []Wait) {
	if o.next == nil {
		o.next = map[string]remembered{}
	}
	in := make([]reading, len(sites))
	for i, s := range sites {
		in[i] = readingOf(s)
	}
	r, ok := o.last[key]
	if !ok || !slices.EqualFunc(r.sites, in, reading.equal) {
		order, waits := Order(sites)
		o.Reads++
		r = remembered{sites: cloneReadings(in), order: order, waits: waits}
	}
	o.next[key] = r
	waits := slices.Clone(r.waits)
	for i, w := range waits {
		if w.Until == sites[w.On].Name+" done" {
			waits[i].Done = sites[w.On].Done
		}
	}
	return slices.Clone(r.order), waits
}

// Sweep forgets every group not asked for since the last sweep.
func (o *Orders) Sweep() {
	o.last, o.next = o.next, nil
}

func cloneReadings(in []reading) []reading {
	out := make([]reading, len(in))
	for i, r := range in {
		out[i] = reading{name: r.name, started: r.started, recap: r.recap,
			prs: slices.Clone(r.prs), words: slices.Clone(r.words), commands: slices.Clone(r.commands)}
	}
	return out
}
