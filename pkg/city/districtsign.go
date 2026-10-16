package city

import "botropolis.city/botropolis/pkg/state"

// DistrictSign is the board standing in each district (item 88): its
// name, and whose move its sessions leave, the same move its card says.
type DistrictSign struct {
	Name string
	Move string
	Tone state.Move
	// At is where the post stands, in world space: the district's front
	// corner, a cell in.
	At Point
}

// Sign is the district's board.
func (d *District) Sign() DistrictSign {
	move := d.move()
	in := min(CellSize, d.Rect.Width()/4, d.Rect.Height()/4)
	return DistrictSign{
		Name: d.Name,
		Move: movePhrase(move),
		Tone: move,
		At:   Point{X: d.Rect.Max.X - in, Y: d.Rect.Max.Y - in},
	}
}

// move is the most urgent move among the district's sessions.
func (d *District) move() state.Move {
	sessions := make([]state.Session, 0, len(d.Buildings))
	for _, b := range d.Buildings {
		sessions = append(sessions, b.Session)
	}
	move, _ := state.GroupMove(sessions)
	return move
}
