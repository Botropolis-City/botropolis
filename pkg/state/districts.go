package state

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var temporaryRoots = []string{"/tmp", "/var/tmp", "/var/folders", "/private/var/folders"}

// Temporary is whether dir lies under a temporary root, matched on
// whole path segments, so /home/u/tmp-project is not (item 272).
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

// tie is how a temporary-directory session is tied to a district, the
// strongest last.
type tie int

const (
	tieTouch tie = iota + 1
	tieMessage
	tieTeam
	tieParent
	tieScratchpad
)

var scratchpad = regexp.MustCompile(`/claude-\d+/([^/]+)(?:/|$)`)

var notAlnum = regexp.MustCompile(`[^A-Za-z0-9]`)

// encodeProject is a directory as Claude Code names it in a path.
func encodeProject(dir string) string {
	return notAlnum.ReplaceAllString(dir, "-")
}

// AssignDistricts sets District on each temporary-directory session its
// transcript ties to a session elsewhere: the directory of the session
// it is tied to, by the strongest tie, followed through other tied
// temporary sessions. An untied one is left to the Temporary district
// (item 272).
func AssignDistricts(sessions []Session) {
	temporary := make([]bool, len(sessions))
	some := false
	for i := range sessions {
		sessions[i].District = ""
		temporary[i] = Temporary(sessions[i].CWD)
		some = some || temporary[i]
	}
	if !some {
		return
	}
	byID := make(map[string]int, len(sessions))
	teams, leads, messagedBy, visitedBy := map[string][]int{}, map[string][]int{}, map[string][]int{}, map[string][]int{}
	encoded := map[string]string{}
	var temp []int
	var settled []Session
	for i := range sessions {
		s := &sessions[i]
		byID[s.ID] = i
		if s.Team != "" {
			teams[s.Team] = append(teams[s.Team], i)
		}
		if s.Leads != "" {
			leads[s.Leads] = append(leads[s.Leads], i)
		}
		for _, id := range s.Messaged {
			messagedBy[id] = append(messagedBy[id], i)
		}
		for dir := range s.Visits {
			visitedBy[filepath.Clean(dir)] = append(visitedBy[filepath.Clean(dir)], i)
		}
		if temporary[i] {
			temp = append(temp, i)
		} else if s.CWD != "" {
			encoded[encodeProject(s.CWD)] = s.CWD
			settled = append(settled, Session{CWD: s.CWD})
		}
	}
	roots := projectRoots(settled)
	placed := func(i int) string {
		if temporary[i] {
			return sessions[i].District
		}
		return sessions[i].CWD
	}
	for {
		found := map[int]string{}
		for _, i := range temp {
			if sessions[i].District != "" {
				continue
			}
			if dir := districtFor(sessions, i, placed, candidates{
				byID: byID, teams: teams, leads: leads, messagedBy: messagedBy, visitedBy: visitedBy,
				encoded: encoded, roots: roots,
			}); dir != "" {
				found[i] = dir
			}
		}
		if len(found) == 0 {
			return
		}
		for i, dir := range found {
			sessions[i].District = dir
		}
	}
}

type candidates struct {
	byID                                map[string]int
	teams, leads, messagedBy, visitedBy map[string][]int
	encoded                             map[string]string
	roots                               []string
}

func districtFor(sessions []Session, i int, placed func(int) string, c candidates) string {
	s := sessions[i]
	type score struct {
		tie   tie
		count int
	}
	scores := map[string]score{}
	add := func(dir string, t tie, n int) {
		if dir == "" {
			return
		}
		sc := scores[dir]
		if t > sc.tie {
			sc = score{tie: t}
		}
		if t == sc.tie {
			sc.count += n
		}
		scores[dir] = sc
	}
	other := func(j int, t tie) {
		if j != i {
			add(placed(j), t, 1)
		}
	}
	if m := scratchpad.FindStringSubmatch(s.CWD); m != nil {
		add(c.encoded[m[1]], tieScratchpad, 1)
	}
	if s.Team != "" {
		for _, j := range c.leads[s.Team] {
			other(j, tieParent)
		}
		for _, j := range c.teams[s.Team] {
			other(j, tieTeam)
		}
	}
	for dir := filepath.Clean(s.CWD); dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		for _, j := range c.visitedBy[dir] {
			other(j, tieParent)
		}
	}
	for _, id := range s.Messaged {
		if j, ok := c.byID[id]; ok {
			other(j, tieMessage)
		}
	}
	for _, j := range c.messagedBy[s.ID] {
		other(j, tieMessage)
	}
	for _, m := range s.PeerMessages {
		if from, ok := peerSender(sessions, m); ok {
			if j, ok := c.byID[from.ID]; ok {
				other(j, tieMessage)
			}
		}
	}
	for path, n := range s.Touches {
		if root := projectOf(c.roots, path); root != "" {
			add(root, tieTouch, n)
		}
	}
	var best string
	var top score
	for _, dir := range slices.Sorted(maps.Keys(scores)) {
		sc := scores[dir]
		if sc.tie > top.tie || sc.tie == top.tie && sc.count > top.count {
			best, top = dir, sc
		}
	}
	return best
}
