package state

// The states of the daemon's statistics store (item 190).
const (
	// StoreOpen is a store the daemon reads and writes.
	StoreOpen = "open"
	// StoreOff is statistics turned off, by the setting or because the
	// daemon was pointed at a home, which keeps none.
	StoreOff = "off"
	// StoreOwed is statistics on with the on prompt still unanswered,
	// so the store stays closed until it is.
	StoreOwed = "owed"
	// StoreRefused is statistics on with a store that would not open.
	StoreRefused = "refused"
)

// StatsStore is what became of the daemon's statistics store: open,
// off, waiting on the prompt, or refused, with the file and the reason
// it was refused, so a refusal reaches the person and not only the
// journal (item 190).
type StatsStore struct {
	State  string `json:"state,omitempty"`
	Path   string `json:"path,omitempty"`
	Reason string `json:"reason,omitempty"`
	// Why is which refusal it was, so the map can say what to do
	// without reading Reason: WhyNewer, WhyDamaged, or "" for another.
	Why string `json:"why,omitempty"`
}

// The refusals the map says what to do about.
const (
	WhyNewer   = "newer"
	WhyDamaged = "damaged"
)
