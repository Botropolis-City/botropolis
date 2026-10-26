package city

// A scripted run, shown, is a street stall rather than a building: a
// third of its lot, in the middle of it. Its subagents in flight queue
// in front of it as customers, where a building's fly over it as
// drones, since a drone over a cart reads wrong (item 273).

const (
	// MaxQueue is the most customers one stall's queue holds.
	MaxQueue = 5
	// stallShare is how much of its lot's width a stall takes.
	stallShare = 1.0 / 3
)

// Stall is whether the building is a stall: its session a scripted run.
func (b *Building) Stall() bool {
	return b.Session.Scripted()
}

// Storied is whether the building has storeys, and so carries what they
// draw: courses, a spire, a skip and lit windows. Ruins and stalls have
// none.
func (b *Building) Storied() bool {
	return !b.Session.ConversationGone && !b.Stall()
}

// StallBody is the ground the stall itself stands on, in the middle of
// its lot.
func (b *Building) StallBody() Rect {
	c := b.Rect.Center()
	w, h := b.Rect.Width()*stallShare, b.Rect.Height()*stallShare
	return RectAt(c.X-w/2, c.Y-h/2, w, h)
}

// Queue is where a stall's customers stand, one per subagent in flight,
// in a line out from the stall's front, the face toward +y, to its lot's
// edge. A building that is not a stall has none.
func (s *Scene) Queue(b *Building) []Point {
	if b == nil || !b.Stall() {
		return nil
	}
	n := min(b.Session.SubagentsInFlight, MaxQueue)
	if n <= 0 {
		return nil
	}
	body := b.StallBody()
	front, edge := body.Max.Y, b.Rect.Max.Y
	room := edge - front
	step := room / float64(MaxQueue+1)
	x := body.Center().X
	out := make([]Point, n)
	for i := range out {
		out[i] = Point{X: x + float64(i%2)*step*0.6 - step*0.3, Y: front + step*float64(i+1)}
	}
	return out
}
