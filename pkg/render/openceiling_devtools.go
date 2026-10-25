//go:build devtools

package render

import "os"

// frameCeiling is the open ceiling's mode the environment asks for, which
// only builds with the frame tooling read (item 249).
var frameCeiling = openCeilingMode(os.Getenv("BOTROPOLIS_FRAME_OPEN_CEILING"))
