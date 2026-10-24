package targets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// MapDir is where the maps are kept, shared by every checkout and worktree
// on the machine, one file per revision (item 226, slice 3).
func MapDir(getenv func(string) string, home string) string {
	cache := getenv("XDG_CACHE_HOME")
	if cache == "" {
		cache = filepath.Join(home, ".cache")
	}
	return filepath.Join(cache, "botropolis", "acceptance-map")
}

// SaveMap writes m under its revision.
func SaveMap(dir string, m *Map) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, m.Rev+".json"), append(data, '\n'), 0o644)
}

// NearestMap is the map in dir whose revision is the nearest ancestor of
// HEAD, by behind, which counts the commits from a revision to HEAD or says
// -1 when it is not an ancestor. With maps but none an ancestor, it hands
// one back with -1, so the caller can say so.
func NearestMap(dir string, behind func(rev string) int) (*Map, int) {
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	var best *Map
	bestBehind := -1
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var m Map
		if json.Unmarshal(data, &m) != nil || m.Rev == "" {
			continue
		}
		n := behind(m.Rev)
		switch {
		case n >= 0 && (bestBehind < 0 || n < bestBehind):
			best, bestBehind = &m, n
		case best == nil:
			best = &m
		}
	}
	return best, bestBehind
}

// ReadCI reads what tools/ci-status last recorded in the git common
// directory: green, red, or unknown for anything else or nothing.
func ReadCI(commonDir string) string {
	data, err := os.ReadFile(filepath.Join(commonDir, "botropolis-ci"))
	if err != nil {
		return "unknown"
	}
	state, _, _ := strings.Cut(strings.TrimSpace(string(data)), " ")
	if state == "green" || state == "red" {
		return state
	}
	return "unknown"
}
