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

// Ties is each tied temporary-directory session's district directory,
// by session id: what the statistics place its spend by (item 285).
func (c *City) Ties() map[string]string {
	if c == nil {
		return nil
	}
	return c.ties
}

func ties(lists ...[]state.Session) map[string]string {
	var out map[string]string
	for _, list := range lists {
		for _, s := range list {
			if s.District == "" {
				continue
			}
			if out == nil {
				out = map[string]string{}
			}
			out[s.ID] = s.District
		}
	}
	return out
}
