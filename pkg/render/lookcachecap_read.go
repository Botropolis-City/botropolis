//go:build ebitenginedebug || devtools

package render

import "os"

// lookCacheBytes is the look cache's cap, which the performance tests
// can set in MiB by BOTROPOLIS_LOOK_CACHE_CAP to run it under pressure:
// in the debug build, and in the profiling build, whose per-frame CPU
// the debug build's command log would weigh on (item 212).
func lookCacheBytes() int { return lookCacheCapFrom(os.Getenv("BOTROPOLIS_LOOK_CACHE_CAP")) }
