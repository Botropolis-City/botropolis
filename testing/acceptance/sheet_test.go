package acceptance_test

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

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

	for _, zoom := range []string{"1", "2"} {
		t.Run("when the sheet's buildings are lit at night at zoom "+zoom, func(t *testing.T) {
			out, err := exec.Command(demo, "sheet", "--check-lights", "--zoom", zoom).CombinedOutput()

			t.Run("it should light each lived-in building's panes and nothing else", func(t *testing.T) {
				assert.NoError(t, err, "%s", out)
			})
		})
	}
}
