package city

import (
	"fmt"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

// Standup is a group's stand-up line, one phrase a site in line order,
// computed from what the map holds and nothing else (item 90).
func (s *Scene) Standup(id string) string {
	for _, l := range s.city.Lines {
		if l.Group.ID == id {
			return standup(l, s.Clock())
		}
	}
	return ""
}

func standup(l Line, now time.Time) string {
	phrases := make([]string, 0, len(l.Stations))
	for _, st := range l.Stations {
		if t := st.Teammate; t != nil {
			s := state.Session{State: state.Resting, LastActivity: t.Seen, PRs: []claude.PR{t.PR}}
			phrases = append(phrases, st.Name()+" "+sitePhrase([]state.Session{s}, now))
			continue
		}
		var sessions []state.Session
		for _, b := range st.District.Buildings {
			if b.Group == l.Group {
				sessions = append(sessions, b.Session)
			}
		}
		phrases = append(phrases, st.District.Name+" "+sitePhrase(sessions, now))
	}
	return strings.Join(phrases, "; ")
}

// sitePhrase is what a site is doing, by the most urgent of its
// sessions: needing you, working, a pull request open, merged or
// closed, or stopped.
func sitePhrase(sessions []state.Session, now time.Time) string {
	var last time.Time
	open, merged, closed := false, false, false
	for _, s := range sessions {
		switch s.State {
		case state.NeedsYou:
			return "needs you"
		}
		if s.LastActivity.After(last) {
			last = s.LastActivity
		}
		for _, pr := range s.PRs {
			switch pr.State {
			case claude.PRMerged:
				merged = true
			case claude.PRClosed:
				closed = true
			default:
				open = true
			}
		}
	}
	for _, s := range sessions {
		switch s.State {
		case state.Working, state.Unattended, state.Waiting:
			return "in progress"
		}
	}
	days := int(now.Sub(last) / (24 * time.Hour))
	switch {
	case open && days >= 1:
		// Botropolis knows a pull request only as a session last saw it,
		// so an old open state says when it was seen rather than how
		// long it has waited: it may have merged since.
		return "in review, seen " + dayCount(days) + " ago"
	case open:
		return "in review"
	case merged:
		return "merged"
	case closed:
		return "closed"
	case days >= 1:
		return "stopped, " + dayCount(days)
	}
	return "stopped"
}

func dayCount(n int) string {
	if n == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", n)
}
