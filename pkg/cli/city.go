package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type CityRunner interface {
	Run(cmd *cobra.Command) error
}

// ScreenshotFlag names the flag that makes the city render one frame to a
// PNG and exit; it lives on the root and the city command alike.
const ScreenshotFlag = "screenshot"

// Screenshot is the path given with --screenshot, or "" when not asked for.
func Screenshot(cmd *cobra.Command) string {
	path, _ := cmd.Flags().GetString(ScreenshotFlag)
	return path
}

// KeysFlag names the flag that presses keys, one per frame, before a
// screenshot is taken, so a zoomed or sidebar frame can be shot
// without a hand on the keyboard.
const KeysFlag = "keys"

// RecordFlag names the flag that writes a frame every tenth of a
// second into a directory for --seconds, pressing --keys a couple of
// seconds apart, then exits: the raw material for a GIF.
const (
	RecordFlag  = "record"
	SecondsFlag = "seconds"
	FPSFlag     = "fps"
)

// recordTicks is the city's tick rate while recording; a frame rate
// has to divide it, or frames would land unevenly.
const recordTicks = 30

// defaultFPS and defaultSignage are what a build without the flags uses.
const (
	defaultFPS     = 10
	defaultSignage = "hover"
)

// FPS is the frame rate given with --fps.
func FPS(cmd *cobra.Command) (int, error) {
	fps, err := cmd.Flags().GetInt(FPSFlag)
	if err != nil {
		return defaultFPS, nil
	}
	if fps <= 0 || recordTicks%fps != 0 {
		return 0, fmt.Errorf("--fps %d: must divide %d", fps, recordTicks)
	}
	return fps, nil
}

// ScriptFlag names the flag that plays a clip's choreography while
// recording: the pointer gliding, clicking, zooming and panning.
const ScriptFlag = "script"

// Script is the path given with --script, or "".
func Script(cmd *cobra.Command) string {
	path, _ := cmd.Flags().GetString(ScriptFlag)
	return path
}

// WindowFlag names the flag that sizes the window, and the virtual
// display under --headless with it, for frames bigger than the default.
const WindowFlag = "window"

// Window is the size given with --window, or 0, 0 when not asked for.
func Window(cmd *cobra.Command) (width, height int, err error) {
	raw, _ := cmd.Flags().GetString(WindowFlag)
	if raw == "" {
		return 0, 0, nil
	}
	w, h, ok := strings.Cut(raw, "x")
	width, errW := strconv.Atoi(w)
	height, errH := strconv.Atoi(h)
	if !ok || errW != nil || errH != nil || width <= 0 || height <= 0 {
		return 0, 0, fmt.Errorf("--window %q: want WIDTHxHEIGHT", raw)
	}
	return width, height, nil
}

// Record is the directory given with --record and how long to record.
func Record(cmd *cobra.Command) (string, float64) {
	dir, _ := cmd.Flags().GetString(RecordFlag)
	seconds, _ := cmd.Flags().GetFloat64(SecondsFlag)
	return dir, seconds
}

// HeadlessFlag names the flag that runs the city on a virtual X display
// (Xvfb) so no window opens: for screenshots, recordings and tests
// while someone is using the desktop.
const HeadlessFlag = "headless"

// Headless reports whether --headless was asked for.
func Headless(cmd *cobra.Command) bool {
	headless, _ := cmd.Flags().GetBool(HeadlessFlag)
	return headless
}

// SignageFlag names the flag that picks how a session's title reaches
// the map: phase 21 item 38's prototypes, for comparison frames.
const SignageFlag = "signage"

func Signage(cmd *cobra.Command) string {
	raw, err := cmd.Flags().GetString(SignageFlag)
	if err != nil {
		return defaultSignage
	}
	return raw
}

// HoverFlag names the flag that parks the pointer at a screen position
// for a scripted frame. Some of what the city does only happens under
// the pointer — the contextual highlight most of all — and --keys cannot
// move a mouse, so without this those frames cannot be shot headless at
// all.
const HoverFlag = "hover"

// Hover is the point given with --hover, and whether one was given.
func Hover(cmd *cobra.Command) (x, y float64, ok bool) {
	raw, _ := cmd.Flags().GetString(HoverFlag)
	if raw == "" {
		return 0, 0, false
	}
	parts := strings.SplitN(raw, ",", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	x, errX := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	y, errY := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if errX != nil || errY != nil {
		return 0, 0, false
	}
	return x, y, true
}

// Keys is the list given with --keys, in order.
func Keys(cmd *cobra.Command) []string {
	keys, _ := cmd.Flags().GetStringSlice(KeysFlag)
	return keys
}

func NewCityCLI(load Loader, services CityServices) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "city",
		Short: "Open the city",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			return services.City(cfg).Run(cmd)
		},
	}
	addFrameFlags(cmd.Flags(), true)
	return cmd
}
