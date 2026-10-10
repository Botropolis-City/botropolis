package render

import (
	"testing"

	"botropolis.city/botropolis/pkg/city"
	"github.com/stretchr/testify/assert"
)

func TestRoadTile(t *testing.T) {
	cases := map[string]struct {
		mask int
		want string
	}{
		"a run along world x":        {city.DirE | city.DirW, "roadNS"},
		"a run along world y":        {city.DirN | city.DirS, "roadEW"},
		"a bend from -x to -y":       {city.DirW | city.DirN, "roadNE"},
		"a bend from +x to +y":       {city.DirE | city.DirS, "roadSW"},
		"a junction missing +y":      {city.DirW | city.DirN | city.DirE, "crossroadNES"},
		"a crossroads":               {city.DirN | city.DirE | city.DirS | city.DirW, "crossroad"},
		"a dead end reached from +x": {city.DirE, "endS"},
		"nothing joined":             {0, ""},
	}
	for name, c := range cases {
		t.Run("when a cell is "+name, func(t *testing.T) {
			t.Run("it should pick the pack's tile", func(t *testing.T) {
				assert.Equal(t, c.want, roadTile(c.mask))
			})
		})
	}
}

func TestRiverAndBridgeTiles(t *testing.T) {
	t.Run("when the river runs along world x", func(t *testing.T) {
		t.Run("it should use the pack's NS river", func(t *testing.T) {
			assert.Equal(t, "riverNS", riverTile(city.DirE|city.DirW))
		})
	})

	t.Run("when the river bends", func(t *testing.T) {
		t.Run("it should use the matching corner", func(t *testing.T) {
			assert.Equal(t, "riverNE", riverTile(city.DirW|city.DirN))
		})
	})

	t.Run("when the river starts at the map's edge with one join", func(t *testing.T) {
		t.Run("it should run straight along that axis", func(t *testing.T) {
			assert.Equal(t, "riverNS", riverTile(city.DirE))
		})
	})

	t.Run("when a straight street crosses the river", func(t *testing.T) {
		t.Run("it should be a bridge the same way", func(t *testing.T) {
			assert.Equal(t, "bridgeNS", bridgeTile(city.DirE|city.DirW))
			assert.Equal(t, "bridgeEW", bridgeTile(city.DirN|city.DirS))
		})
	})

	t.Run("when a street bends on the river", func(t *testing.T) {
		t.Run("it should get no bridge", func(t *testing.T) {
			assert.Empty(t, bridgeTile(city.DirN|city.DirE))
		})
	})
}

func TestLakeTile(t *testing.T) {
	t.Run("when a lake cell is surrounded by water", func(t *testing.T) {
		t.Run("it should be open water", func(t *testing.T) {
			assert.Equal(t, "water", lakeTile(0))
		})
	})

	t.Run("when a lake cell meets land on its world-west side", func(t *testing.T) {
		t.Run("it should shore the pack's N edge", func(t *testing.T) {
			assert.Equal(t, "waterN", lakeTile(city.DirW))
		})
	})

	t.Run("when a lake cell is a corner", func(t *testing.T) {
		t.Run("it should shore two adjacent edges", func(t *testing.T) {
			assert.Equal(t, "waterNE", lakeTile(city.DirW|city.DirN))
			assert.Equal(t, "waterSW", lakeTile(city.DirE|city.DirS))
		})
	})
}
