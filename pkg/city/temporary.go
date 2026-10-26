package city

import (
	"os"
	"path/filepath"
	"strings"
)

// TemporaryRoot is the one district every session in a temporary
// directory folds into, and TemporaryName what it is called (item 272).
const (
	TemporaryRoot = "/tmp"
	TemporaryName = "Temporary"
)

var temporaryRoots = []string{"/tmp", "/var/tmp", "/var/folders", "/private/var/folders"}

// Temporary is whether dir lies under a temporary root, matched on
// whole path segments, so /home/u/tmp-project is not.
func Temporary(dir string) bool {
	if dir == "" {
		return false
	}
	dir = filepath.Clean(dir)
	for _, root := range append([]string{os.TempDir(), os.Getenv("TMPDIR")}, temporaryRoots...) {
		if root == "" {
			continue
		}
		root = filepath.Clean(root)
		if root == "/" {
			continue
		}
		if dir == root || strings.HasPrefix(dir, root+"/") {
			return true
		}
	}
	return false
}
