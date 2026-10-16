package render

import (
	"hash/fnv"
	"math"
	"strconv"

	"botropolis.city/botropolis/pkg/city"
)

const kitSapling = "nature-kit/tree_small_dark"

// overgrowthPieces are what grows on a lot quiet for long enough: grass
// and weeds, and now and then a sapling (item 86).
var overgrowthPieces = []string{
	"nature-kit/grass", "nature-kit/grass_large", "nature-kit/grass_leafs", "nature-kit/plant_flatShort", kitSapling,
}

// weed is one piece of growth and where on the lot it stands.
type weed struct {
	Piece string
	At    city.Point
}

// lot is what a building's lot carries for its rung: the beacon and the
// sign while it is live, boards once it is shuttered, and growth once it
// is overgrown or while it is being mowed.
type lot struct {
	Beacon, Sign, Boards bool
	Weeds                []weed
}

// dressLot dresses a building's lot. mowing is how far a mowing has got,
// from 0 to 1, or below 0 when there is none: the growth clears in the
// order it was scattered.
func dressLot(b *city.Building, mowing float64) lot {
	d := lot{Beacon: !b.Blinds, Sign: !b.Blinds, Boards: b.BoardedUp}
	if !b.Overgrown && mowing < 0 {
		return d
	}
	weeds := scatter(b)
	if mowing > 0 {
		weeds = weeds[min(len(weeds), int(math.Round(mowing*float64(len(weeds))))):]
	}
	d.Weeds = weeds
	return d
}

// scatter places a lot's growth along its two front edges, where the
// building does not hide it, seeded by the session's id so the same lot
// always grows the same way.
func scatter(b *city.Building) []weed {
	seed := hashID(b.Session.ID)
	n := 8 + int(seed%4)
	r := b.Rect
	w, h := r.Width(), r.Height()
	out := make([]weed, 0, n)
	for i := 0; i < n; i++ {
		f := fnv.New32a()
		_, _ = f.Write([]byte(b.Session.ID + "/" + strconv.Itoa(i)))
		v := f.Sum32()
		along := 0.15 + 0.7*float64(v%1000)/1000
		inset := 0.02 + 0.06*float64((v/1000)%100)/100
		at := city.Point{X: r.Max.X - inset*w, Y: r.Min.Y + along*h}
		if (v/100000)%2 == 0 {
			at = city.Point{X: r.Min.X + along*w, Y: r.Max.Y - inset*h}
		}
		piece := overgrowthPieces[int(v>>24)%(len(overgrowthPieces)-1)]
		if i == 0 && seed%2 == 0 {
			piece = kitSapling
		}
		out = append(out, weed{Piece: piece, At: at})
	}
	return out
}

// boards are the planks across a shuttered building's two visible faces,
// in screen space: a cross on each, over the storey a door would be on.
func boards(r city.Rect) [][2]city.Point {
	mid := r.Min.X + r.Width()/2
	top, bottom := r.Min.Y+r.Height()*0.6, r.Min.Y+r.Height()*0.82
	inset := r.Width() * 0.14
	var out [][2]city.Point
	for _, face := range [][2]float64{{r.Min.X + inset, mid - inset}, {mid + inset, r.Max.X - inset}} {
		out = append(out,
			[2]city.Point{{X: face[0], Y: top}, {X: face[1], Y: bottom}},
			[2]city.Point{{X: face[0], Y: bottom}, {X: face[1], Y: top}})
	}
	return out
}
