package city

import (
	"cmp"

	"botropolis.city/botropolis/pkg/state"
)

// TemporaryRoot is the one district every session in a temporary
// directory folds into, and TemporaryName what it is called (item 272).
const (
	TemporaryRoot = "/tmp"
	TemporaryName = "Temporary"
)

// Temporary is whether dir lies under a temporary root.
func Temporary(dir string) bool {
	return state.Temporary(dir)
}

// SessionRoot is the district a session stands in: the one its
// transcript ties it to, or its own directory's.
func SessionRoot(s state.Session) string {
	return ProjectRoot(cmp.Or(s.District, s.CWD))
}
