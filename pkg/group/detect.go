// Package group finds the groups sessions belong to — work that runs
// across directories — from what the sessions already say and how they
// were started, and remembers them with their corrections (item 87).
package group

import "time"

const (
	// burst and continuing are how close two starts, or a close and a
	// start, must be to strengthen another signal; timing never joins on
	// its own (item 271).
	burst      = 15 * time.Minute
	continuing = 10 * time.Minute
)

// MaxMembers is the most sessions one detected group holds (item 271).
const MaxMembers = 40

// Member is one session as detection sees it.
type Member struct {
	ID      string
	Dir     string
	Branch  string
	Started time.Time
	// Ended is when the session closed, or zero while it runs.
	Ended   time.Time
	Prompts []string
	Pasted  []string
	Parent  string
	Team    string
	// Messaged are the sessions this one sent messages to, and Visited
	// the directories outside its own where its subagents worked.
	Messaged []string
	Visited  []Visit
	Files    []string
	// StartedFrom is the group the session was started from, if any,
	// and Job the background job it was started as, by which memory
	// records where New session started it.
	StartedFrom string
	Job         string
}

// Visit is a directory a session's subagents worked in, and when.
type Visit struct {
	Dir      string
	From, To time.Time
}

type Tier int

const (
	Possible Tier = iota + 1
	Likely
	Sure
)

type Signal string

const (
	// SignalCoordinates is a session whose subagents worked in another's
	// directory while both ran: the strongest signal (item 90).
	SignalCoordinates Signal = "coordinates"
	SignalStartedFrom Signal = "started-from"
	SignalContinuing  Signal = "continuing"
	SignalReference   Signal = "reference"
	SignalParentage   Signal = "parentage"
	SignalPasted      Signal = "pasted"
	SignalWording     Signal = "wording"
	SignalBranch      Signal = "branch"
	SignalFiles       Signal = "files"
	SignalTiming      Signal = "timing"
	SignalMoved       Signal = "moved"
)

// Reason is why a session joined, or is suggested: the signal, what it
// matched on, and the session it matched.
type Reason struct {
	Signal Signal `json:"signal"`
	Detail string `json:"detail,omitempty"`
	With   string `json:"with,omitempty"`
}

type Membership struct {
	Session string
	Dir     string
	Started time.Time
	Tier    Tier
	Reason  Reason
}

type Group struct {
	ID      string
	Name    string
	Renamed bool
	Anchor  string
	Members []Membership
}

// Member is the session's membership, or the zero value.
func (g Group) Member(session string) Membership {
	for _, m := range g.Members {
		if m.Session == session {
			return m
		}
	}
	return Membership{}
}

// Dirs are the directories the group runs through, in the order its
// sessions there were started.
func (g Group) Dirs() []string {
	var dirs []string
	seen := map[string]bool{}
	for _, m := range g.Members {
		if !seen[m.Dir] {
			seen[m.Dir] = true
			dirs = append(dirs, m.Dir)
		}
	}
	return dirs
}

// Suggestion is a group a loose session might belong to, on one weak
// signal: shown, never joined.
type Suggestion struct {
	Session string
	Group   string
	Reason  Reason
}

type Result struct {
	Groups      []Group
	Suggestions []Suggestion
	Capped      []Capped
	// Common are the keys more sessions share than a group can hold,
	// such as "reference PROJ-42" or "branch feat/x", sorted: none of
	// them ties sessions together (item 271).
	Common []string
}

// Capped is a session left out of a group that was already as large as
// a group can be, and the tie that would have joined it.
type Capped struct {
	Session string
	Reason  Reason
}

func (r Result) GroupOf(session string) (Group, bool) {
	for _, g := range r.Groups {
		if g.Member(session).Session != "" {
			return g, true
		}
	}
	return Group{}, false
}

func (r Result) SuggestionsFor(session string) []Suggestion {
	var out []Suggestion
	for _, s := range r.Suggestions {
		if s.Session == session {
			out = append(out, s)
		}
	}
	return out
}

// edge is a scored pair; timed is a likely pair that needs its timing
// to be likely, which joins only one hop from evidence (item 271).
type edge struct {
	a, b   int
	tier   Tier
	timed  bool
	reason Reason
}

type detection struct {
	members []Member
	index   map[string]int
	refs    [][]string
	read    map[string]readRefs
	wording *Wording
	blocked map[[2]string]bool
	// moved are the sessions put in a group, or none, by hand; they sit
	// out detection and are placed where they were put.
	moved     map[string]bool
	confirmed map[string]bool
	// started are the sessions started from a group, by session; like
	// the moved, they sit out detection and join where they were started.
	started map[string]string
	// names are the names the detector gave groups last run, kept for
	// groups whose words have not moved (item 198); nil names every
	// group afresh.
	names *keptNames
	// capped are the sessions cluster left out of a full group.
	capped []Capped
	// common are the keys too many members share to be evidence.
	common map[indexKey]bool
	// weighed is how many pairs the wording has been weighed for
	// (item 270).
	weighed int
}
