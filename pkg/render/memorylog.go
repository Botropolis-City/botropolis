package render

import (
	"os"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/memlog"
)

// memoryLog is the city's own memory, logged for the light-cycle check
// by a devtools build asked for it, and nil, doing nothing, otherwise
// (item 318).
var memoryLog = memlog.New(os.Getenv("BOTROPOLIS_MEMORY_LOG"), imageBytes)

// imageBytes is what Ebitengine's images take, by its own count.
func imageBytes() int64 {
	var d ebiten.DebugInfo
	ebiten.ReadDebugInfo(&d)
	return d.TotalGPUImageMemoryUsageInBytes
}
