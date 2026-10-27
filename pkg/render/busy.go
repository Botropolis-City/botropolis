package render

import (
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

// running is the notice shown while an action runs beside the loop.
func running(kind city.ActionKind, name string) string {
	return doingWhat(kind, name) + "…"
}

// already is the notice a second press of a running action gets.
func already(kind city.ActionKind, name string) string {
	return "already " + doingWhat(kind, name)
}
