package render

import (
	"sync"
	_ "unsafe"
)

// Ebitengine v2.10.2 keeps every draw made outside its game loop in
// internal/atlas's deferred list, which only the loop's BeginFrame runs
// and empties. A Drive never runs the loop, so without this every frame
// it drew stayed in memory for the life of the process, and make bench
// grew to about 48 GB (item 104). Nothing in a driven process can ever
// run those draws, so dropping them changes nothing a Drive shows or
// measures.
//
// This reaches two unexported variables in
// github.com/hajimehoshi/ebiten/v2/internal/atlas (image.go): deferred
// and deferredM. An Ebitengine upgrade that renames either fails at link
// time; TestDrive fails if one keeps the names and stops using them.

//go:linkname atlasDeferred github.com/hajimehoshi/ebiten/v2/internal/atlas.deferred
var atlasDeferred []func()

//go:linkname atlasDeferredM github.com/hajimehoshi/ebiten/v2/internal/atlas.deferredM
var atlasDeferredM sync.Mutex

// dropDeferred discards the draws Ebitengine is holding for a frame that
// will never come.
func dropDeferred() {
	atlasDeferredM.Lock()
	atlasDeferred = nil
	atlasDeferredM.Unlock()
}
