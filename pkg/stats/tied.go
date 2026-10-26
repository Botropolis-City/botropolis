package stats

import (
	"sort"
	"strings"
)

// A tie puts a temporary-directory session's spend in the district its
// transcript ties it to, by session id: tied maps a session to that
// district's directory. A message counts at the least of its tied
// sessions' directories, and at its own directory when none holds it,
// so a fork's copy counts once (item 285).

// tiedPlaces is the SQL for each message a tied session holds, as id,
// and the directory it counts at, as place.
func tiedPlaces(tied map[string]string) string {
	ids := make([]string, 0, len(tied))
	for id := range tied {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	rows := make([]string, len(ids))
	for i, id := range ids {
		rows[i] = "(" + quote(id) + ", " + quote(tied[id]) + ")"
	}
	return "(SELECT ms.message_id AS id, min(t.column2) AS place FROM (VALUES " + strings.Join(rows, ", ") +
		") t CROSS JOIN message_sessions ms ON ms.session_id = t.column1 GROUP BY ms.message_id)"
}

func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// placeOf is the directory a held message counts at.
func placeOf(h *held, tied map[string]string) string {
	place, found := "", false
	for _, s := range h.sessions {
		if d, ok := tied[s]; ok && (!found || d < place) {
			place, found = d, true
		}
	}
	if !found {
		return h.dir
	}
	return place
}
