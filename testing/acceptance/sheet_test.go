package acceptance_test

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/testing/helpers"
)

// wheelZooms is every zoom a wheel step lands on, as the flag takes it:
// from the ladder itself, so a step added to it is checked too, drawn
// up or not (item 233).
func wheelZooms() []string {
	var out []string
	for _, z := range city.ZoomSteps {
		out = append(out, strconv.FormatFloat(z, 'g', -1, 64))
	}
	return out
}

// Item 166: at night a building's lit windows are its own window panes,
// lit, and not round halos over them. Every kind at every tier, and its
// ruins, lived in and boarded up, is drawn at night with its lights on
// and off, and the pixels its lights change must lie on its panes, a
// pixel of filtering aside; boarded windows and ruins light nothing.
func TestBuildingSheetLights(t *testing.T) {
	helpers.NeedXvfb(t)
	demo := filepath.Join(t.TempDir(), "botropolis-demo")
	build := exec.Command("go", "build", "-o", demo, "../../cmd/botropolis-demo")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "%s", out)

	for _, zoom := range wheelZooms() {
		t.Run("when the sheet's buildings are lit at night at zoom "+zoom, func(t *testing.T) {
			out, err := exec.Command(demo, "sheet", "--check-lights", "--zoom", zoom).CombinedOutput()

			t.Run("it should light each lived-in building's panes and nothing else", func(t *testing.T) {
				assert.NoError(t, err, "%s", out)
			})
		})
	}
}

// Item 192: a building's look is baked once and blitted after, and the
// blit must be the building as it was drawn before: every kind and
// dressing, from every heading, by day and by night, and drawn anew
// once its look changes after it was baked.
func TestBuildingLooksCached(t *testing.T) {
	helpers.NeedXvfb(t)
	demo := filepath.Join(t.TempDir(), "botropolis-demo")
	build := exec.Command("go", "build", "-o", demo, "../../cmd/botropolis-demo")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "%s", out)

	for _, zoom := range wheelZooms() {
		t.Run("when each look is drawn through the cache at zoom "+zoom, func(t *testing.T) {
			out, err := exec.Command(demo, "sheet", "--check-looks", "--zoom", zoom).CombinedOutput()

			t.Run("it should match the building drawn directly", func(t *testing.T) {
				assert.NoError(t, err, "%s", out)
			})
		})
	}
}
