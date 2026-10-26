package app

// CityMemoryLimit is the city's soft limit on Go's memory, so the
// collector works harder as the heap nears it rather than letting it
// double what is live (Aria, 2026-10-26, item 270). A settled city may
// hold 512 MiB, textures and all, and about 180 MB of it lies outside
// Go, in the driver and Ebitengine, so Go has about 300. GOMEMLIMIT
// overrides it, as it does the daemon's (item 157).
const CityMemoryLimit = 300 << 20

// cityMemoryLimit is the limit the city sets, or -1, which changes
// nothing, when GOMEMLIMIT has already set it.
func cityMemoryLimit(env string) int64 {
	if env != "" {
		return -1
	}
	return CityMemoryLimit
}
