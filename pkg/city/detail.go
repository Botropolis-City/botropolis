package city

// How much of the city is drawn.
//
// The scenery — the trees along every avenue, the lamps, the planting in
// the district yards — is the largest single thing in a frame by count,
// and none of it means anything. It is there because a city with no
// trees does not read as a city, which is a real job and worth the
// pixels most of the time. But it is the first thing to give up on a
// laptop on battery, or on a map projected on a wall across the room,
// and it is the only part of the map whose absence costs no information
// at all.
//
// One setting, not a checkbox per kind. A row of tick boxes for trees,
// lamps and bushes would be three ways to ask the same question, and
// each of them would have to be explained.

// Detail is how much scenery is drawn.
type Detail int

const (
	// DetailFull is the city as designed.
	DetailFull Detail = iota
	// DetailPlain drops the scenery and keeps everything that means
	// something: the ground, the streets, the districts, the buildings,
	// the plaza and the landmarks.
	DetailPlain
)

func (d Detail) String() string {
	if d == DetailPlain {
		return "plain"
	}
	return "full"
}

// ParseDetail reads the setting, and refuses a name it does not know
// rather than falling back to a default that would look like a bug.
func ParseDetail(s string) (Detail, bool) {
	switch s {
	case "full":
		return DetailFull, true
	case "plain":
		return DetailPlain, true
	}
	return DetailFull, false
}

// Detail is how much of the city is being drawn.
//
// Not to be confused with Scene.Detailed, which is a different question
// with a similar name: that one asks whether the camera is close enough
// for sprites to read at all, and it answers from the zoom. This one is
// the setting, and it holds at every zoom.
func (s *Scene) Detail() Detail { return s.detail }

// SetDetail changes it. The composed layer is keyed on the generation,
// so bumping it is what makes the change appear rather than waiting for
// the next snapshot.
func (s *Scene) SetDetail(d Detail) {
	if d == s.detail {
		return
	}
	s.detail = d
	s.generation++
	s.groundGeneration++
}

// Scenery reports whether the things that mean nothing are drawn.
func (s *Scene) Scenery() bool { return s.detail == DetailFull }
