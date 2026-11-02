package render

import (
	"os"

	"github.com/hajimehoshi/ebiten/v2"

	"botropolis.city/botropolis/pkg/memlog"
)

// memoryLog is the city's own memory, logged for the light-cycle check
// by a devtools build asked for it, and nil, doing nothing, otherwise
// (item 318). memoryLogGame is the game whose record it reads.
var (
	memoryLogGame *Game
	memoryLog     = memlog.New(os.Getenv("BOTROPOLIS_MEMORY_LOG"), memlog.Sources{Images: imageBytes, Records: loggedRecord})
)

// logMemory holds the record to its rule this frame and samples the log
// when one is due, only while a log is asked for.
func (g *Game) logMemory() {
	if memoryLog == nil {
		return
	}
	memoryLogGame = g
	g.noteRecordRule()
	memoryLog.Sample()
}

// loggedRecord is the logged game's record slack and broken half.
func loggedRecord() (int64, string) {
	if memoryLogGame == nil {
		return 0, ""
	}
	return memoryLogGame.recordReading()
}

// imageBytes is what Ebitengine's images take, by its own count.
func imageBytes() int64 {
	var d ebiten.DebugInfo
	ebiten.ReadDebugInfo(&d)
	return d.TotalGPUImageMemoryUsageInBytes
}
