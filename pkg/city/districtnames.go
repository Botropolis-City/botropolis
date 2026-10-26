package city

import (
	"path/filepath"
	"strings"

	"botropolis.city/botropolis/pkg/stats"
)

// FolderUnknown names the sessions whose record carries no folder,
// never "." (item 139).
const FolderUnknown = "folder unknown"

// DistrictNames labels each root with as much of its path as tells it
// apart from the others: the base name when that is unique, else the
// fewest trailing folders no other root ends in, else the whole path
// (item 139).
func DistrictNames(roots []string) map[string]string {
	trimmed := map[string]string{}
	for _, root := range roots {
		if root != "" {
			trimmed[root] = strings.Trim(filepath.ToSlash(root), "/")
		}
	}
	// A tail is a root's own when no other root ends in it: each tail
	// is counted once per root that ends in it, rather than every root
	// being held against every other (item 201).
	ending := map[string]int{}
	for _, path := range trimmed {
		tails(path, func(tail string) bool { ending[tail]++; return true })
	}
	names := make(map[string]string, len(roots))
	for _, root := range roots {
		if root == "" {
			names[root] = FolderUnknown
			continue
		}
		if root == TemporaryRoot {
			names[root] = TemporaryName
			continue
		}
		names[root] = root
		tails(trimmed[root], func(tail string) bool {
			if ending[tail] == 1 {
				names[root] = tail
				return false
			}
			return true
		})
	}
	return names
}

// tails hands each run of a path's trailing folders to yield, the
// shortest first, until yield says to stop.
func tails(path string, yield func(string) bool) {
	for end := len(path); ; {
		cut := strings.LastIndexByte(path[:end], '/')
		if !yield(path[cut+1:]) || cut < 0 {
			return
		}
		end = cut
	}
}

// storeNames names the store's project roots alongside every root the
// city knows, so a root only the store remembers is still told apart
// from its twin on the map.
func (c *City) storeNames(slices []stats.Slice) func(string) string {
	roots := make([]string, 0, len(c.names)+len(slices))
	for root := range c.names {
		roots = append(roots, root)
	}
	for _, sl := range slices {
		roots = append(roots, sl.Key)
	}
	names := DistrictNames(roots)
	return func(root string) string { return names[root] }
}
