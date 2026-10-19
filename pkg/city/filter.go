package city

import (
	"strings"
)

// Filter is the search: every word must appear in a session's title,
// project, branch, state or model, case-insensitively — or, once the
// finder has answered, the session must be among those it found.
type Filter struct {
	words []string
	// found is the finder's answer, by session id; nil until it answers.
	// Item 82.
	found map[string]bool
}

// ParseFilter splits a query into words.
func ParseFilter(query string) Filter {
	return Filter{words: strings.Fields(strings.ToLower(query))}
}

func (f Filter) Empty() bool {
	return len(f.words) == 0
}

// Matches reports whether a building's session has every word.
func (f Filter) Matches(b *Building) bool {
	if f.Empty() {
		return true
	}
	if f.found != nil {
		return f.found[b.Session.ID]
	}
	s := b.Session
	hay := strings.ToLower(strings.Join([]string{s.Title, s.ID, b.Where, s.Branch, string(s.State), s.Model}, " "))
	for _, w := range f.words {
		if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}

// SetFilter is the search: the map dims what does not match.
func (s *Scene) SetFilter(query string) {
	s.filter = ParseFilter(query)
}

// SetFound filters the map to the sessions the finder found for a
// query: what was said in them is not on the map, so the map follows
// the finder's answer rather than matching the words itself. Item 82.
func (s *Scene) SetFound(query string, ids []string) {
	found := make(map[string]bool, len(ids))
	for _, id := range ids {
		found[id] = true
	}
	s.filter = Filter{words: ParseFilter(query).words, found: found}
}

// Filter is the search as it stands.
func (s *Scene) Filter() Filter {
	return s.filter
}

// Dimmed reports whether a building falls outside the search.
func (s *Scene) Dimmed(b *Building) bool {
	return !s.filter.Matches(b)
}

// Matching counts the buildings the search keeps.
func (s *Scene) Matching() int {
	n := 0
	for _, b := range s.city.Buildings() {
		if s.filter.Matches(b) {
			n++
		}
	}
	return n
}
