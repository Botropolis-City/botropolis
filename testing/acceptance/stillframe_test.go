package acceptance_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

// Item 193: the standing city is composed once and only what moves is
// drawn again, inside rectangles, from the ground up. Every frame of the
// metropolis, as it stands, with a car hovered, a view up and a building
// selected, by day and by night, must be the frame drawn straight, so a
// mover behind a building stays behind it and nothing still goes stale.
func TestTheStillCityMatchesTheCityDrawnDirectly(t *testing.T) {
	helpers.NeedXvfb(t)
	demo := filepath.Join(t.TempDir(), "botropolis-demo")
	build := exec.Command("go", "build", "-o", demo, "../../cmd/botropolis-demo")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "%s", out)

	for _, light := range []struct {
		name string
		hour int
	}{{"day", 14}, {"night", 22}} {
		t.Run("when the metropolis is drawn for 30 frames each way by "+light.name, func(t *testing.T) {
			t.Parallel()
			home := stageDemo(t, "metropolis")
			args := []string{"sheet", "--check-frames", home, "--hour", fmt.Sprint(light.hour), "--frames", "30"}
			if dir := os.Getenv("BOTROPOLIS_FRAME_DUMP"); dir != "" {
				args = append(args, "--dump", filepath.Join(dir, light.name))
			}
			out, err := exec.Command(demo, args...).CombinedOutput()

			t.Run("it should match the city drawn straight on every frame", func(t *testing.T) {
				assert.NoError(t, err, "%s", out)
			})
		})
	}
}
