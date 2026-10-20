//go:build devtools

// The city's frame tooling: flags for screenshots, recordings, scripted
// runs and a virtual display. Only builds with the devtools tag have
// them, so release builds neither show nor promise them. Item 167.

package cli

import "github.com/spf13/pflag"

// addFrameFlags registers the frame tooling's flags; the root takes
// every one but --signage.
func addFrameFlags(flags *pflag.FlagSet, signage bool) {
	AddScreenshotFlag(flags)
	AddKeysFlag(flags)
	AddRecordFlags(flags)
	AddHeadlessFlag(flags)
	AddHoverFlag(flags)
	AddWindowFlag(flags)
	AddScriptFlag(flags)
	if signage {
		AddSignageFlag(flags)
	}
}

func AddScreenshotFlag(flags *pflag.FlagSet) {
	flags.String(ScreenshotFlag, "", "render one frame to this PNG and exit")
}

func AddKeysFlag(flags *pflag.FlagSet) {
	flags.StringSlice(KeysFlag, nil, "keys to press before the screenshot, e.g. equal,equal,b")
}

func AddRecordFlags(flags *pflag.FlagSet) {
	flags.String(RecordFlag, "", "write frames into this directory for --seconds, then exit")
	flags.Float64(SecondsFlag, 20, "how long to record with --record")
	flags.Int(FPSFlag, defaultFPS, "frames a second to write with --record: 1, 2, 3, 5, 6, 10, 15 or 30")
}

func AddScriptFlag(flags *pflag.FlagSet) {
	flags.String(ScriptFlag, "", "with --record, play this choreography (see pkg/script)")
}

func AddWindowFlag(flags *pflag.FlagSet) {
	flags.String(WindowFlag, "", "window size as WIDTHxHEIGHT, e.g. 1920x1080")
}

func AddHeadlessFlag(flags *pflag.FlagSet) {
	flags.Bool(HeadlessFlag, false, "run on a virtual display (Xvfb) so no window opens")
}

func AddSignageFlag(flags *pflag.FlagSet) {
	flags.String(SignageFlag, defaultSignage, "how a session's title is shown: hover (the default), plates, gantry, board or plaque")
}

func AddHoverFlag(flags *pflag.FlagSet) {
	flags.String(HoverFlag, "", "park the pointer at X,Y in window pixels for the screenshot")
}
