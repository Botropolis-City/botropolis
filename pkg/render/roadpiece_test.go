package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// What the kit's road pieces actually look like, measured off the atlas
// rather than assumed.
//
// Each sprite was read at all four baked headings and its open sides
// taken from the tile diamond's edge midpoints: a closed side carries
// the kit's raised kerb, which is near-white, and an open side is road
// surface, which is mid grey. The check that the method is sound is that
// every piece came out with exactly the number of open sides its name
// implies, at every turn — a straight always two, a T always three.
//
// At camera heading 0 the projection is screen = ((x-y)s, (x+y)s/2) with
// +X east and +Y south, so the diamond's upper-right edge faces north,
// lower-right east, lower-left south and upper-left west.
var openAtTurnZero = map[string]int{
	"city-kit-roads/road-straight":     city.DirE | city.DirW,
	"city-kit-roads/road-crossroad":    city.DirN | city.DirE | city.DirS | city.DirW,
	"city-kit-roads/road-bend":         city.DirS | city.DirW,
	"city-kit-roads/road-intersection": city.DirE | city.DirS | city.DirW,
	"city-kit-roads/road-end":          city.DirE,
	"city-kit-roads/road-square":       0,
}

// turned is a piece's open sides after a turn. Every +90 takes each open
// side one step along E -> N -> W -> S, which is the sense the atlas
// bakes its headings in — measured, and the same for all five pieces.
func turned(open, turn int) int {
	step := map[int]int{city.DirE: city.DirN, city.DirN: city.DirW, city.DirW: city.DirS, city.DirS: city.DirE}
	for i := 0; i < ((turn/90)%4+4)%4; i++ {
		next := 0
		for _, d := range []int{city.DirN, city.DirE, city.DirS, city.DirW} {
			if open&d != 0 {
				next |= step[d]
			}
		}
		open = next
	}
	return open
}

func maskName(mask int) string {
	s := ""
	for _, d := range []struct {
		bit  int
		name string
	}{{city.DirN, "N"}, {city.DirE, "E"}, {city.DirS, "S"}, {city.DirW, "W"}} {
		if mask&d.bit != 0 {
			s += d.name
		}
	}
	if s == "" {
		return "none"
	}
	return s
}

// The class, not the instance: every one of the sixteen ways a cell can
// join its neighbours must pick a piece and a turn whose open sides are
// exactly those neighbours. Checking only the bend would have let the
// same mistake back in through the T or the end — which is where it also
// was.
func TestRoadPieceOpensWhereItJoins(t *testing.T) {
	for mask := 0; mask < 16; mask++ {
		t.Run("when a street cell joins "+maskName(mask), func(t *testing.T) {
			name, turn := roadPiece(mask)
			open, known := openAtTurnZero[name]
			require.True(t, known, "no measured geometry for %s", name)

			t.Run("it should choose a piece open on exactly those sides", func(t *testing.T) {
				assert.Equal(t, maskName(mask), maskName(turned(open, turn)),
					"%s turned %d is open %s", name, turn, maskName(turned(open, turn)))
			})
		})
	}
}
