//go:build !ebitenginedebug && !devtools

package render

// lookCacheBytes is the look cache's cap: in a release build, always
// lookCacheCap, whatever the environment says (item 212).
func lookCacheBytes() int { return lookCacheCap }
