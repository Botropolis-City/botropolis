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
	in := min(CellSize, d.Rect.Width()/4, d.Rect.Height()/4)
	// The depot is not a district: its sign says what it holds, and no
	// move, since nothing in it is anyone's to make (item 86).
	if d.Depot {
		return DistrictSign{Name: DepotName + " · " + plural(len(d.Buildings), "crate"), At: Point{X: d.Rect.Max.X - in, Y: d.Rect.Max.Y - in}}
	}
	move, phrase := d.move(), d.phrase
	if d.moved == nil {
		phrase = movePhrase(move)
	}
	return DistrictSign{
		Name: d.Name,
		Move: phrase,
		Tone: move,
		At:   Point{X: d.Rect.Max.X - in, Y: d.Rect.Max.Y - in},
	}
}

// move is the most urgent move among the district's sessions, worked
// out once its city is built, since a frame asks every district for its
// sign (item 270). The sign's name is read afresh each time: sharing
// renames a district after its city is built (item 168).
func (d *District) move() state.Move {
	if d.moved != nil {
		return *d.moved
	}
	return d.urgentMove()
}

func (d *District) urgentMove() state.Move {
	sessions := make([]state.Session, 0, len(d.Buildings))
	for _, b := range d.Buildings {
		sessions = append(sessions, b.Session)
	}
	move, _ := state.GroupMove(sessions)
	return move
}
