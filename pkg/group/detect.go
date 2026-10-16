// Package group finds the groups sessions belong to — work that runs
// across directories — from what the sessions already say and how they
// were started, and remembers them with their corrections (item 87).
package group

import "time"

const (
	// together is how close two starts, or a close and a start, must be
	// to suggest one piece of work on their own; burst and continuing
	// are how close they must be to strengthen another signal.
	together   = time.Minute
	burst      = 15 * time.Minute
	continuing = 10 * time.Minute
)

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
	// Messaged are the sessions this one sent messages to.
	Messaged []string
	Files    []string
	// StartedFrom is the group the session was started from, if any.
	StartedFrom string
}

type Tier int

const (
	Possible Tier = iota + 1
	Likely
	Sure
)

type Signal string

const (
	SignalStartedFrom Signal = "started-from"
	SignalContinuing  Signal = "continuing"
	SignalReference   Signal = "reference"
	SignalParentage   Signal = "parentage"
	SignalPasted      Signal = "pasted"
	SignalWording     Signal = "wording"
	SignalBranch      Signal = "branch"
	SignalFiles       Signal = "files"
	SignalTiming      Signal = "timing"
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

type edge struct {
	a, b   int
	tier   Tier
	reason Reason
}

type detection struct {
	members []Member
	index   map[string]int
	refs    [][]string
	wording *Wording
	blocked map[[2]string]bool
}
