package city

import (
	"slices"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/state"
)

// JournalEntry is one line of the journal that opens after a gap (item
// 86, slice 5): a group, or a session on its own, with what it was doing
// last time and what is next, read from what Claude already wrote.
type JournalEntry struct {
	Title    string
	Sessions []string
	Pinned   bool
	LastTime string
	Next     string
	// Where is the projects it is in, and Fly the session the camera
	// goes to: its most recently touched.
	Where   []string
	Fly     string
	Touched time.Time
}

// notRecorded is Next when nothing says what is next.
const notRecorded = "not recorded"

// BuildJournal makes the journal from a snapshot: one entry per group
// and per session in no group, leaving out what is empty or packed up,
// pinned entries first and then most recently touched. reply is the last
// paragraph of a session's last reply, read when the journal opens and
// never kept.
func BuildJournal(snapshot state.Snapshot, pinned []string, reply func(id string) string) []JournalEntry {
	byID := map[string]state.Session{}
	roots := make([]string, 0, len(snapshot.Sessions))
	for _, s := range snapshot.Sessions {
		roots = append(roots, SessionRoot(s))
	}
	names := DistrictNames(roots)
	for _, s := range snapshot.Sessions {
		if s.State == state.Empty || s.Packed {
			continue
		}
		byID[s.ID] = s
	}
	grouped := map[string]bool{}
	var entries []JournalEntry
	for _, g := range snapshot.Groups {
		var members []state.Session
		for _, id := range g.Members {
			if s, ok := byID[id]; ok {
				members = append(members, s)
				grouped[id] = true
			}
		}
		if len(members) > 0 {
			entries = append(entries, journalEntry(g.Name, members, names, pinned, reply))
		}
	}
	for _, s := range snapshot.Sessions {
		if _, ok := byID[s.ID]; ok && !grouped[s.ID] {
			title := s.Title
			if title == "" {
				title = state.ShortID(s.ID)
			}
			entries = append(entries, journalEntry(title, []state.Session{s}, names, pinned, reply))
		}
	}
	slices.SortStableFunc(entries, func(a, b JournalEntry) int {
		if a.Pinned != b.Pinned {
			if a.Pinned {
				return -1
			}
			return 1
		}
		return b.Touched.Compare(a.Touched)
	})
	return entries
}

func journalEntry(title string, members []state.Session, names map[string]string, pinned []string, reply func(string) string) JournalEntry {
	slices.SortStableFunc(members, func(a, b state.Session) int { return b.LastActivity.Compare(a.LastActivity) })
	latest := members[0]
	e := JournalEntry{Title: title, Fly: latest.ID, Touched: latest.LastActivity}
	where := map[string]bool{}
	for _, s := range members {
		e.Sessions = append(e.Sessions, s.ID)
		e.Pinned = e.Pinned || slices.Contains(pinned, s.ID)
		where[names[SessionRoot(s)]] = true
	}
	for w := range where {
		e.Where = append(e.Where, w)
	}
	slices.Sort(e.Where)
	e.LastTime, e.Next = LastAndNext(latest, reply)
	return e
}

// LastAndNext is a session's Last time and Next: the recap and its own
// Next line where there is one; else your last prompt and the last
// paragraph of the reply, and the question the session last asked.
//
// The journal fills an entry in with this when it is first drawn, so
// building the journal reads nothing and a reply is read only for an
// entry someone can see. Item 91.
func LastAndNext(s state.Session, reply func(string) string) (string, string) {
	var last, next string
	if s.Recap.Text != "" {
		var kept []string
		for _, line := range strings.Split(s.Recap.Text, "\n") {
			if after, ok := strings.CutPrefix(strings.TrimSpace(line), "Next:"); ok {
				next = strings.TrimSpace(after)
				continue
			}
			kept = append(kept, line)
		}
		last = strings.TrimSpace(strings.Join(kept, "\n"))
	}
	paragraph := ""
	if reply != nil {
		paragraph = lastParagraph(reply(s.ID))
	}
	if last == "" {
		if s.LastPrompt != "" {
			last = "You asked: “" + s.LastPrompt + "”"
		}
		if paragraph != "" {
			if last != "" {
				last += " — "
			}
			last += paragraph
		}
	}
	switch {
	case next != "":
	case s.State == state.NeedsYou && s.Note != "":
		next = s.Note
	case strings.HasSuffix(paragraph, "?"):
		next = lastSentence(paragraph)
	default:
		next = notRecorded
	}
	return last, next
}

func lastParagraph(text string) string {
	paragraphs := strings.Split(strings.TrimSpace(text), "\n\n")
	return strings.TrimSpace(paragraphs[len(paragraphs)-1])
}

func lastSentence(text string) string {
	cut := strings.LastIndexAny(strings.TrimSuffix(text, "?"), ".!?\n")
	return strings.TrimSpace(text[cut+1:])
}

// NoteSeen notes the moment the map was last seen, and which sessions
// were live then: the city's last sight, which Restore the desk falls
// back on when you did not close up (item 86).
func (s *Scene) NoteSeen(now time.Time) {
	if s.layout == nil {
		return
	}
	s.layout.Seen = now
	s.layout.LastLive = s.layout.LastLive[:0]
	for _, sess := range s.snapshot.Sessions {
		if state.Live(sess.State) && !sess.Packed {
			s.layout.LastLive = append(s.layout.LastLive, sess.ID)
		}
	}
}
