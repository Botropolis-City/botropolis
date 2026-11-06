package render

// applyFrameFocus focuses the group a scripted frame asked for, once its
// buildings are on the map, keeping the camera where it stands so the
// frame compares with the same one unfocused (item 348).
func (g *Game) applyFrameFocus() {
	if g.focusFor == "" || g.focusDone {
		return
	}
	cam := g.scene.Camera()
	kept := *cam
	if g.scene.FocusGroup(g.focusFor) {
		*cam = kept
		g.focusDone = true
	}
}
