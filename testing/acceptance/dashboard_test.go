//go:build linux

package acceptance_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/render"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 207: an unfocused city that can be seen is a dashboard, painting
// a frame for each snapshot it lays out. Filed as drawing the whole
// world for each because the standing city was never reused, it was
// reused all along: this held before the item's fix, and stays as the
// guard the item asked for. What a dashboard frame cost was the dry
// pass's CPU, which BenchmarkADashboardFrame reports.
func TestAnUnfocusedCityUnderAStreamOfChanges(t *testing.T) {
	helpers.NeedXvfb(t)
	bin, err := debugCity()
	require.NoError(t, err)
	b := newBusyCity(t)
	display := helpers.Display(t, render.DefaultWidth, render.DefaultHeight)
	city := exec.Command(bin, "city", "--home", b.home.Path, "--socket", b.sock, "--config", b.config())
	city.Env = append(cleanEnv(), "HOME="+filepath.Join(b.root, "home"), "XDG_STATE_HOME="+filepath.Join(b.root, "city-state"), "TMPDIR="+b.root, "BOTROPOLIS_FRAMETIME=1")
	city.Env = append(city.Env, display.Env()...)
	var said, out syncBuffer
	city.Stderr, city.Stdout = &said, &out
	require.NoError(t, city.Start())
	t.Cleanup(func() { _ = city.Process.Kill(); _ = city.Wait() })
	require.Eventually(t, func() bool { return strings.Contains(said.String(), "FRAME ") }, 60*time.Second, 100*time.Millisecond)
	time.Sleep(5 * time.Second)
	helpers.Unfocus(t, display)
	require.Eventually(t, func() bool { return strings.Contains(said.String(), "focused=false visible=true") }, 30*time.Second, 100*time.Millisecond)
	time.Sleep(2 * time.Second)
	from := len(out.String())
	b.write(200*time.Millisecond, 6*time.Second)
	time.Sleep(time.Second)
	var drawn []helpers.Frame
	for _, f := range helpers.ParseFrameLog(strings.NewReader(out.String()[from:])) {
		if f.Drawn {
			drawn = append(drawn, f)
		}
	}
	worst := helpers.MostWork(drawn)
	t.Logf("%d dashboard frames, worst: %+v", len(drawn), worst.Still)

	t.Run("it should paint a frame for the snapshots it lays out", func(t *testing.T) {
		assert.GreaterOrEqual(t, len(drawn), 3)
	})

	t.Run("it should compose the standing city whole on no frame", func(t *testing.T) {
		assert.Zero(t, worst.Still.Composes+worst.Still.Ground)
	})

	t.Run("it should draw the city straight on no frame", func(t *testing.T) {
		assert.Zero(t, worst.Still.Direct)
	})
}
