package city

import (
	"slices"
	"strings"
)

// The card's face (item 350): one direct verb, then three columns,
// Session, Project and Group, which the action menu opens. The columns
// hold the same verbs Actions holds, grouped, so the state logic there
// decides what is offered and this only decides where it goes.

// VerbColumn is one column of the action menu: its title and every kind
// that can take a row in it, in the order the rows are shown.
type VerbColumn struct {
	Title string
	Kinds []ActionKind
}

// VerbColumns is the one list of the menu's columns, which the card's
// face, the menu and its key table all read.
var VerbColumns = []VerbColumn{
	{"Session", []ActionKind{ActionAttach, ActionResume, ActionStop, ActionRead, ActionGenerate, ActionFork, ActionPack, ActionUnpack, ActionBreakdown, ActionOpenPR, ActionDemolish}},
	{"Project", []ActionKind{ActionNew, ActionReveal, ActionCopyPath, ActionStar, ActionUnstar, ActionHide, ActionDistrictType, ActionShowRoad}},
	{"Group", []ActionKind{ActionMoveToGroup, ActionJoinSuggested, ActionTakeOut, ActionRenameGroup, ActionCloseGroup, ActionCopyStandup, ActionMuteWord, ActionFocusLine}},
}

// Verbs is what the selection's face offers: the direct verb, which is
// also what Enter does, and the three columns.
type Verbs struct {
	Direct                  ActionKind
	Session, Project, Group []ActionKind
}

// Columns is the three columns in order.
func (v Verbs) Columns() [][]ActionKind {
	return [][]ActionKind{v.Session, v.Project, v.Group}
}

// All is every verb in every column, which is what the menu is opened
// with.
func (v Verbs) All() []ActionKind {
	return slices.Concat(v.Session, v.Project, v.Group)
}

// Verbs groups the selection's actions: the first is the direct verb,
// and each other takes its row in its column. The details fold is not
// a verb and takes no row.
func (s *Scene) Verbs() Verbs {
	var v Verbs
	has := map[ActionKind]bool{}
	for _, k := range s.Actions() {
		if k == ActionDetails || k == ActionDetailsHide {
			continue
		}
		if v.Direct == ActionNone {
			v.Direct = k
		}
		has[k] = true
	}
	into := []*[]ActionKind{&v.Session, &v.Project, &v.Group}
	for i, col := range VerbColumns {
		for _, k := range col.Kinds {
			if has[k] {
				*into[i] = append(*into[i], k)
			}
		}
	}
	return v
}

// faceButtons is the card's four buttons: the direct verb's label, then
// one per column, each marked as a menu.
func (v Verbs) faceButtons() []string {
	labels := []string{string(v.Direct)}
	for _, col := range VerbColumns {
		labels = append(labels, col.Title+" ▾")
	}
	return labels
}

// faceHints is one hint per face button: the direct verb's description,
// and for each column the rows it would open on.
func (v Verbs) faceHints(describe func(ActionKind) string) []string {
	hints := []string{describe(v.Direct)}
	for i, col := range VerbColumns {
		kinds := v.Columns()[i]
		if len(kinds) == 0 {
			hints = append(hints, col.Title+": nothing applies here")
			continue
		}
		var labels []string
		for _, k := range kinds {
			labels = append(labels, string(k))
		}
		hints = append(hints, col.Title+": "+strings.Join(labels, ", "))
	}
	return hints
}
