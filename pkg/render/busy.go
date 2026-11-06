package render

import (
	"strings"
	"sync"

	"botropolis.city/botropolis/pkg/city"
)

// busy is the claude actions running beside the city's loop, one per
// action and session, so a second press cannot start a second claude
// that spends money on the same thing (item 286).
type busy struct {
	mu      sync.Mutex
	running map[busyKey]bool
}

type busyKey struct {
	kind  city.ActionKind
	about string
}

func keyOf(a city.Action) busyKey {
	about := a.SessionID
	if about == "" {
		about = a.Dir
	}
	return busyKey{a.Kind, about}
}

// begin marks a running, and says false when it already was.
func (b *busy) begin(a city.Action) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := keyOf(a)
	if b.running[k] {
		return false
	}
	if b.running == nil {
		b.running = map[busyKey]bool{}
	}
	b.running[k] = true
	return true
}

// is whether an action is running.
func (b *busy) is(a city.Action) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running[keyOf(a)]
}

func (b *busy) end(a city.Action) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.running, keyOf(a))
}

var doing = map[city.ActionKind]string{
	city.ActionResume:     "resuming ",
	city.ActionNew:        "starting a session in ",
	city.ActionNewSession: "starting a session in ",
	city.ActionFork:       "forking ",
	city.ActionStop:       "stopping ",
	city.ActionDemolish:   "removing ",
	city.ActionRestore:    "restoring the desk",
	city.ActionCloseUp:    "closing up",
}

func doingWhat(kind city.ActionKind, name string) string {
	d := doing[kind]
	if d != "" && d[len(d)-1] == ' ' {
		return d + name
	}
	return d
}

// progress is what a button says while its action runs: the doing
// word alone, since the card it is on already names the session (item
// 352).
func progress(kind city.ActionKind) string {
	words := strings.Fields(doing[kind])
	if len(words) == 0 {
		return string(kind)
	}
	return words[0] + "…"
}

// running is the notice shown while an action runs beside the loop with
// no button on screen to say so.
func running(kind city.ActionKind, name string) string {
	return doingWhat(kind, name) + "…"
}
