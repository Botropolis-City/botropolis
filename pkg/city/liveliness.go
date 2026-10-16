package city

// How much the city moves. Item 89.
//
// Three kinds of motion are on the map, and the setting treats them
// differently. Ambient motion — cars, drones, the train, smoke — runs at
// the level's pace, and at still it stops where it is, so every mover is
// still on the map as a marker. Transitions play in full above still and
// become short cross-fades at it. Attention is not the setting's to
// change.

// Liveliness is how much the city moves. The zero value is the default.
type Liveliness int

const (
	// Steady is the default, between calm and lively.
	Steady Liveliness = iota
	// Still stops ambient motion and cross-fades every transition.
	Still
	// Calm runs ambient motion at half pace.
	Calm
	// Lively runs ambient motion at full pace.
	Lively
)

// The levels' names, in one place: the setting, the flag and the panel
// all read them from here.
var livelinessNames = map[Liveliness]string{
	Still:  "still",
	Calm:   "calm",
	Steady: "steady",
	Lively: "lively",
}

// LivelinessLevels is every level, from still to lively.
var LivelinessLevels = []Liveliness{Still, Calm, Steady, Lively}

func (l Liveliness) String() string {
	if name, ok := livelinessNames[l]; ok {
		return name
	}
	return livelinessNames[Steady]
}

// ParseLiveliness reads the setting, and refuses a name it does not know.
func ParseLiveliness(s string) (Liveliness, bool) {
	for level, name := range livelinessNames {
		if name == s {
			return level, true
		}
	}
	return Steady, false
}

// Pace is how fast ambient motion runs, as a fraction of full speed.
func (l Liveliness) Pace() float64 {
	switch l {
	case Still:
		return 0
	case Calm:
		return 0.5
	case Lively:
		return 1
	}
	return 0.75
}

// CrossFades reports whether a transition is a short cross-fade rather
// than played in full.
func (l Liveliness) CrossFades() bool { return l == Still }
