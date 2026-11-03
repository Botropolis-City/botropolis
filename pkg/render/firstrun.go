package render

import (
	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/pkg/state"
)

// The first run is the tidy-up walk (item 333): it opens when the
// groups have never been reviewed, once a launch, and only for someone
// looking. Nothing is accepted quietly (item 87).

// maybeFirstRun opens the walk when the groups have never been reviewed
// and there is something to tidy: a screenshot, a recording or a driven
// run is not asked.
func (g *Game) maybeFirstRun() {
	// Not while sharing: the walk would show the veil's names, and
	// saving it would keep them (item 168).
	if g.firstRunAsked || !g.looking() {
		return
	}
	if !g.scene.Snapshot().GroupsReviewed && g.openWalk() {
		g.firstRunAsked = true
	}
}

// reviewOf is the groups as a review, each kept until a choice is made.
func reviewOf(groups []state.Group) group.Review {
	var r group.Review
	for _, gr := range groups {
		row := group.ReviewRow{ID: gr.ID, Name: gr.Name, Reason: gr.Reason, Keep: true}
		for i, id := range gr.Members {
			dir := ""
			if i < len(gr.MemberDirs) {
				dir = gr.MemberDirs[i]
			}
			title := id
			if i < len(gr.MemberTitles) {
				title = gr.MemberTitles[i]
			}
			row.Members = append(row.Members, group.ReviewMember{Session: id, Dir: dir, Title: title})
		}
		r.Rows = append(r.Rows, row)
	}
	return r
}
