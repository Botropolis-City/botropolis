package group

import (
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// A Review is the first run's proposal as it stands on the screen: the
// groups found, each kept or dropped, merged or split, until it is
// confirmed. Nothing in it is accepted until then (item 87).
type Review struct {
	Rows []ReviewRow
	// Apart are sets of sessions a split has separated, which must not be
	// joined again: each part, and each session left loose, is one set.
	Apart [][][]string
	// Out are the pairs a member taken out of a row makes with the rest
	// of it, remembered as not related (item 333).
	Out []Pair
}

type ReviewRow struct {
	ID      string
	Name    string
	Reason  string
	Members []ReviewMember
	Keep    bool
	// Changed is a row a merge or a split made, whose members have to be
	// put in it by hand because detection would not put them there.
	Changed bool
	// Renamed is a row given its name by hand, and Decided one a choice
	// was made about, which is all leaving the walk for now writes
	// (item 333).
	Renamed bool
	Decided bool
}

type ReviewMember struct {
	Session string
	Dir     string
	// Title is the member's name as the city says it (item 333).
	Title string
}

// NewReview proposes every group found, each kept.
func NewReview(r Result) Review {
	var review Review
	for _, g := range r.Groups {
		row := ReviewRow{ID: g.ID, Name: g.Name, Keep: true}
		for _, m := range g.Members {
			row.Members = append(row.Members, ReviewMember{Session: m.Session, Dir: m.Dir})
		}
		review.Rows = append(review.Rows, row)
	}
	return review
}

// Dirs are the row's directories, in its members' order.
func (r ReviewRow) Dirs() []string {
	var dirs []string
	seen := map[string]bool{}
	for _, m := range r.Members {
		if !seen[m.Dir] {
			seen[m.Dir] = true
			dirs = append(dirs, m.Dir)
		}
	}
	return dirs
}

// Toggle keeps or drops a row.
func (r *Review) Toggle(i int) {
	if i >= 0 && i < len(r.Rows) {
		r.Rows[i].Keep = !r.Rows[i].Keep
		r.Rows[i].Decided = true
	}
}

// Decide marks a row decided as it stands: kept, or dropped.
func (r *Review) Decide(i int) {
	if i >= 0 && i < len(r.Rows) {
		r.Rows[i].Decided = true
	}
}

// Rename names a row by hand (item 333).
func (r *Review) Rename(i int, name string) {
	if i >= 0 && i < len(r.Rows) {
		r.Rows[i].Name, r.Rows[i].Renamed, r.Rows[i].Decided = strings.TrimSpace(name), true, true
	}
}

// TakeOut takes a member out of a row, not related to the rest of it
// (item 333).
func (r *Review) TakeOut(i int, session string) {
	if i < 0 || i >= len(r.Rows) {
		return
	}
	row := &r.Rows[i]
	at := slices.IndexFunc(row.Members, func(m ReviewMember) bool { return m.Session == session })
	if at < 0 {
		return
	}
	row.Members = slices.Delete(slices.Clone(row.Members), at, at+1)
	for _, m := range row.Members {
		r.Out = append(r.Out, Pair{session, m.Session})
	}
	row.Decided = true
}

// AnyDecided is whether any choice has been made in the review.
func (r Review) AnyDecided() bool {
	return len(r.Apart) > 0 || len(r.Out) > 0 || slices.ContainsFunc(r.Rows, func(row ReviewRow) bool { return row.Decided })
}

// DecidedOnly is the review with only the rows a choice was made about,
// and every split and take-out.
func (r Review) DecidedOnly() Review {
	out := Review{Apart: r.Apart, Out: r.Out}
	for _, row := range r.Rows {
		if row.Decided {
			out.Rows = append(out.Rows, row)
		}
	}
	return out
}

// Merge moves row from's members into row into, and removes from.
func (r *Review) Merge(from, into int) {
	if from == into || from < 0 || into < 0 || from >= len(r.Rows) || into >= len(r.Rows) {
		return
	}
	r.Rows[into].Members = append(r.Rows[into].Members, r.Rows[from].Members...)
	r.Rows[into].Changed, r.Rows[into].Decided = true, true
	r.Rows = append(r.Rows[:from], r.Rows[from+1:]...)
}

// Split breaks a row into one group per directory: a directory with two
// or more of its sessions becomes a group, and the rest go loose.
func (r *Review) Split(i int) {
	if i < 0 || i >= len(r.Rows) {
		return
	}
	row := r.Rows[i]
	byDir := map[string][]ReviewMember{}
	for _, m := range row.Members {
		byDir[m.Dir] = append(byDir[m.Dir], m)
	}
	var parts []ReviewRow
	var apart [][]string
	for _, dir := range row.Dirs() {
		ms := byDir[dir]
		ids := make([]string, 0, len(ms))
		for _, m := range ms {
			ids = append(ids, m.Session)
		}
		if len(ms) < 2 {
			apart = append(apart, ids)
			continue
		}
		apart = append(apart, ids)
		parts = append(parts, ReviewRow{ID: newID(row.ID + "/" + dir), Name: row.Name + " · " + filepath.Base(dir),
			Reason: row.Reason, Members: ms, Keep: true, Changed: true, Decided: true})
	}
	r.Apart = append(r.Apart, apart)
	r.Rows = append(r.Rows[:i], append(parts, r.Rows[i+1:]...)...)
}

// Confirm is memory with the review accepted: kept rows stored as groups,
// changed rows' members put there by hand, dropped rows' and split parts'
// members remembered as not related, and the first run marked reviewed.
func (m Memory) Confirm(r Review, at time.Time) Memory {
	for _, row := range r.Rows {
		if !slices.Contains(m.ReviewedGroups, row.ID) {
			m.ReviewedGroups = append(slices.Clone(m.ReviewedGroups), row.ID)
		}
		ids := make([]string, 0, len(row.Members))
		for _, mm := range row.Members {
			ids = append(ids, mm.Session)
		}
		if !row.Keep {
			m.NotRelated = append(m.NotRelated, pairsWithin(ids)...)
			continue
		}
		m.Groups = upsert(m.Groups, Stored{ID: row.ID, Name: row.Name, Renamed: row.Renamed, Members: ids})
		if row.Changed {
			for _, id := range ids {
				m = m.WithMove(id, row.ID)
			}
		}
	}
	m.NotRelated = append(m.NotRelated, r.Out...)
	for _, apart := range r.Apart {
		for i := range apart {
			for j := i + 1; j < len(apart); j++ {
				for _, a := range apart[i] {
					for _, b := range apart[j] {
						m.NotRelated = append(m.NotRelated, Pair{a, b})
					}
				}
			}
		}
	}
	reviewed := at.UTC()
	m.Reviewed = &reviewed
	return m
}

// ConfirmDecided is memory with only the review's decisions accepted,
// leaving every row nobody chose about to detection's guesses, and the
// tidy-up marked reviewed (item 333).
func (m Memory) ConfirmDecided(r Review, at time.Time) Memory {
	return m.Confirm(r.DecidedOnly(), at)
}

func pairsWithin(ids []string) []Pair {
	var pairs []Pair
	for i := range ids {
		for j := i + 1; j < len(ids); j++ {
			pairs = append(pairs, Pair{ids[i], ids[j]})
		}
	}
	return pairs
}

func upsert(groups []Stored, s Stored) []Stored {
	for i := range groups {
		if groups[i].ID == s.ID {
			if groups[i].Renamed && !s.Renamed {
				s.Name, s.Renamed = groups[i].Name, true
			}
			groups[i] = s
			return groups
		}
	}
	return append(groups, s)
}
