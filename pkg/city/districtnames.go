package city

import (
	"path/filepath"
	"slices"
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
	split := map[string][]string{}
	for _, root := range roots {
		if root != "" {
			split[root] = strings.Split(strings.Trim(filepath.ToSlash(root), "/"), "/")
		}
	}
	names := make(map[string]string, len(roots))
	for _, root := range roots {
		if root == "" {
			names[root] = FolderUnknown
			continue
		}
		names[root] = root
		parts := split[root]
		for k := 1; k <= len(parts); k++ {
			tail := parts[len(parts)-k:]
			if !endsElsewhere(split, root, tail) {
				names[root] = strings.Join(tail, "/")
				break
			}
		}
	}
	return names
}

func endsElsewhere(split map[string][]string, root string, tail []string) bool {
	for other, parts := range split {
		if other != root && len(parts) >= len(tail) && slices.Equal(parts[len(parts)-len(tail):], tail) {
			return true
		}
	}
	return false
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
