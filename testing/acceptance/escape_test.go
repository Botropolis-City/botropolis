package acceptance_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/render"
)

// openPanels names everything a frame drew on top of the map.
func openPanels(s render.Shown) []string {
	var open []string
	for name, on := range map[string]bool{
		"reference": s.Help, "lens key": s.Views, "settings": s.Settings, "breakdown": s.Breakdown,
		"timeline": s.Timeline, "finder": s.Search, "sidebar": s.Sidebar, "journal": s.Journal != nil,
		"triage": s.Triage != "", "card": s.CardRect.Area() > 0, "lens panel": s.LensRect.Area() > 0,
		"overlay": s.Overlays > 0, "city plan": s.Plan, "hidden": s.Hidden,
	} {
		if on {
			open = append(open, name)
		}
	}
	slices.Sort(open)
	return open
}

// Item 246: the validation checklist's step 6, scripted. Every panel a
// key opens closes again with Escape, and leaves the map as it found it.
func TestEveryPanelClosesOnEscape(t *testing.T) {
	home := stageDemo(t, "hamlet")
	overlay := func(s render.Shown) bool { return s.Overlays > 0 }

	for _, tc := range []struct {
		key, panel string
		selected   bool
		flies      bool
		leaves     []string
		open       func(render.Shown) bool
	}{
		{key: "b", panel: "the sidebar", open: func(s render.Shown) bool { return s.Sidebar }},
		{key: "t", panel: "the timeline", open: func(s render.Shown) bool { return s.Timeline }},
		{key: "s", panel: "settings", open: func(s render.Shown) bool { return s.Settings }},
		{key: "?", panel: "the reference", open: func(s render.Shown) bool { return s.Help }},
		{key: "x", panel: "the power breakdown", open: func(s render.Shown) bool { return s.Breakdown }},
		{key: "g", panel: "the groups board", open: overlay},
		{key: "Slash", panel: "the finder", open: func(s render.Shown) bool { return s.Search }},
		{key: "w", panel: "triage", flies: true, leaves: []string{"card"}, open: func(s render.Shown) bool { return s.Triage != "" }},
		{key: "shift+w", panel: "the journal", open: func(s render.Shown) bool { return s.Journal != nil }},
		{key: "v", panel: "the lens key", open: func(s render.Shown) bool { return s.Views }},
		{key: "shift+n", panel: "New session", open: overlay},
		{key: "shift+q", panel: "close up", open: overlay},
		{key: "m", panel: "the City Plan", open: func(s render.Shown) bool { return s.Plan }},
		{key: "h", panel: "the hidden interface", open: func(s render.Shown) bool { return s.Hidden }},
		{key: "o", panel: "the reading pane", selected: true, open: overlay},
		{key: "i", panel: "the selection's details", selected: true, open: func(s render.Shown) bool { return s.Details }},
	} {
		t.Run("when "+tc.key+" opens "+tc.panel, func(t *testing.T) {
			d, err := render.NewDrive(render.DriveOptions{Home: home})
			require.NoError(t, err)
			_, err = d.Until(func(s render.Shown) bool { return s.Settled }, settleTicks)
			require.NoError(t, err)
			if tc.selected {
				require.NoError(t, d.Press("Tab"))
				_, err = d.Until(func(s render.Shown) bool { return s.Selected != "" && s.Settled }, settleTicks*3)
				require.NoError(t, err, "a session should be selected")
			}
			before := d.Shown()
			require.NoError(t, d.Press(tc.key))
			_, err = d.Until(tc.open, settleTicks)

			t.Run("it should open it", func(t *testing.T) {
				assert.NoError(t, err)
			})

			t.Run("and Escape is pressed", func(t *testing.T) {
				require.NoError(t, d.Press("Escape"))
				require.NoError(t, d.Run(3))
				_, err := d.Until(func(s render.Shown) bool { return s.Settled }, settleTicks*3)
				require.NoError(t, err)
				after := d.Shown()

				t.Run("it should close it", func(t *testing.T) {
					assert.False(t, tc.open(after))
				})

				t.Run("it should leave nothing open that was not open before", func(t *testing.T) {
					var left []string
					for _, p := range openPanels(after) {
						if !slices.Contains(openPanels(before), p) && !slices.Contains(tc.leaves, p) {
							left = append(left, p)
						}
					}
					assert.Empty(t, left)
				})

				if !tc.flies {
					t.Run("it should leave the camera where it was", func(t *testing.T) {
						assert.Equal(t, before.Camera, after.Camera)
					})
				}
			})
		})
	}
}
