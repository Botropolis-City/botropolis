//go:build linux

package acceptance_test

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/demo"
	"botropolis.city/botropolis/pkg/script"
)

// theClear is the colour the city clears its frame to under the ground.
var theClear = color.NRGBA{0x12, 0x14, 0x1a, 0xff}

// loopFPS is how many frames a second of the loop are checked.
const loopFPS = 2

// recordLoop records the metropolis's loop shot as the film does: its
// window, its keys and its script, at its clock's hour, two frames a
// second, and returns the path of every frame the film keeps or blends
// in, those after its lead.
func recordLoop(t *testing.T) []string {
	t.Helper()
	bin, err := debugCity()
	require.NoError(t, err)
	path := filepath.Join("..", "..", "demo", "scenarios", "metropolis.yaml")
	scenario, err := demo.LoadScenario(path)
	require.NoError(t, err)
	var loop demo.Shot
	for _, s := range scenario.Shots {
		if s.Name == "loop" {
			loop = s
		}
	}
	require.NotNil(t, loop.Record, "the metropolis should have a recorded loop shot")
	home := stageDemo(t, "metropolis")
	work := t.TempDir()
	cues := filepath.Join(work, "script.json")
	require.NoError(t, script.Save(cues, loop.Script))
	frames := filepath.Join(work, "frames")
	require.NoError(t, os.MkdirAll(frames, 0o755))
	seconds := demo.LoopLead + loop.Record.Seconds + loop.Loop.Crossfade
	cmd := exec.Command(bin, "city", "--headless", "--home", home, "--socket", filepath.Join(work, "none.sock"),
		"--window", "1920x1080", "--set", "group_lines=false", "--script", cues,
		"--record", frames, "--seconds", fmt.Sprint(seconds), "--fps", fmt.Sprint(loopFPS))
	cmd.Env = append(os.Environ(), "TZ="+zoneAt(time.Now().UTC().Hour(), 18))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Run(), stderr.String())
	all, err := filepath.Glob(filepath.Join(frames, "frame-*.png"))
	require.NoError(t, err)
	lead := int(demo.LoopLead * loopFPS)
	require.Greater(t, len(all), lead)
	return all[lead:]
}

// clearPixels counts the pixels of a frame painted in the clear colour.
func clearPixels(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	require.NoError(t, err)
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA) == theClear {
				n++
			}
		}
	}
	return n
}

// Item 323: the hero loop's pan looked past the drawn sea's edge, and the
// frame's clear colour showed as a black wedge in its corner.
func TestTheHeroLoopsSeaFillsTheFrame(t *testing.T) {
	t.Run("when the metropolis loop is recorded as it is filmed", func(t *testing.T) {
		frames := recordLoop(t)
		bare := map[string]int{}
		for _, f := range frames {
			if n := clearPixels(t, f); n > 0 {
				bare[filepath.Base(f)] = n
			}
		}

		t.Run("it should paint no pixel of any frame in the clear colour", func(t *testing.T) {
			assert.Empty(t, bare)
		})
	})
}
