package render

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/plan"
	"botropolis.city/botropolis/pkg/state"
)

// Item 280: the kit's building-n carries a green cross, a clinic's sign,
// and City Hall was drawn with it.
func TestHallPiece(t *testing.T) {
	t.Run("when City Hall is drawn", func(t *testing.T) {
		t.Run("it should be the modelled civic building, not the kit's clinic", func(t *testing.T) {
			assert.Equal(t, "botropolis/city-hall", kitHall)
		})
	})
}

func TestTreePiece(t *testing.T) {
	t.Run("when the plan may ask for a species", func(t *testing.T) {
		t.Run("it should have a park piece for every one", func(t *testing.T) {
			assert.Len(t, kitParkTrees, plan.ParkSpecies)
		})

		t.Run("it should have a street piece for every one", func(t *testing.T) {
			assert.Len(t, kitStreetTrees, plan.StreetSpecies)
		})

		t.Run("it should have a bush for every one", func(t *testing.T) {
			assert.Len(t, kitBushes, plan.BushSpecies)
		})
	})

	t.Run("when a planting is a planter", func(t *testing.T) {
		t.Run("it should be the kit's planter whatever its species", func(t *testing.T) {
			assert.Equal(t, kitPlanter, treePiece(city.Tree{Kind: plan.Planter, Variant: 3}))
		})
	})

	t.Run("when a planting names a species the palette does not have", func(t *testing.T) {
		t.Run("it should fall back to the first rather than panic", func(t *testing.T) {
			assert.Equal(t, kitParkTrees[0], treePiece(city.Tree{Kind: plan.ParkTree, Variant: 99}))
		})
	})

	t.Run("when every piece a planting can name is looked up in the atlas", func(t *testing.T) {
		atlases, err := assets.LoadKits()
		require.NoError(t, err)
		require.NotEmpty(t, atlases)
		var every []string
		every = append(every, kitParkTrees...)
		every = append(every, kitStreetTrees...)
		every = append(every, kitBushes...)
		every = append(every, kitPlanter)

		t.Run("it should be a piece the pipeline cut", func(t *testing.T) {
			for _, name := range every {
				_, ok := atlases[0].Sprite(name, 0)
				assert.True(t, ok, "%s is not in the atlas; add it to PIECES in render.py and re-run make sprites", name)
			}
		})
	})
}

func TestSprayFrame(t *testing.T) {
	t.Run("when the clock has just started", func(t *testing.T) {
		t.Run("it should show the first frame", func(t *testing.T) {
			assert.Equal(t, 0, sprayFrame(0, 3))
		})
	})

	t.Run("when a period has passed", func(t *testing.T) {
		t.Run("it should have stepped on one", func(t *testing.T) {
			assert.Equal(t, 1, sprayFrame(SprayPeriod*1.5, 3))
		})
	})

	t.Run("when the cycle has come round", func(t *testing.T) {
		t.Run("it should be back at the first", func(t *testing.T) {
			assert.Equal(t, 0, sprayFrame(SprayPeriod*3, 3))
		})
	})

	t.Run("when the clock stands still, as it does with motion reduced", func(t *testing.T) {
		held := sprayFrame(pulsePeriod/4, 3)

		t.Run("it should hold one frame however long you look", func(t *testing.T) {
			assert.Equal(t, held, sprayFrame(pulsePeriod/4, 3))
		})
	})

	t.Run("when there are no frames to show", func(t *testing.T) {
		t.Run("it should not divide by zero", func(t *testing.T) {
			assert.Equal(t, 0, sprayFrame(12, 0))
		})
	})
}

// Item 109: a building is its district type's kind for its storeys.
func TestBuildingPiece(t *testing.T) {
	t.Run("when a session is packed up", func(t *testing.T) {
		b := &city.Building{Packed: true, Hue: 1, Session: state.Session{Storeys: 4}}

		t.Run("it should be a crate in its district's hue", func(t *testing.T) {
			assert.Equal(t, crates[1], buildingPiece(b))
		})
	})

	for _, kind := range city.DistrictTypes {
		for n := 1; n <= state.MaxStoreys; n++ {
			t.Run(fmt.Sprintf("when a %s district's session reaches %d storeys", kind, n), func(t *testing.T) {
				b := &city.Building{Type: kind, Session: state.Session{Storeys: n}}

				t.Run("it should be that type's piece for that height", func(t *testing.T) {
					assert.Equal(t, fmt.Sprintf("botropolis/%s-%d", kind, n), buildingPiece(b))
				})
			})
		}
	}

	// Aria, 2026-10-18: a session whose transcript Claude Code deleted is
	// a ruin, abandoned and fallen apart, not a building of no height.
	for _, kind := range city.DistrictTypes {
		t.Run(fmt.Sprintf("when a %s district's session has lost its transcript", kind), func(t *testing.T) {
			b := &city.Building{Type: kind, Session: state.Session{ConversationGone: true, Storeys: 4}}

			t.Run("it should be that type's ruins", func(t *testing.T) {
				assert.Equal(t, fmt.Sprintf("botropolis/%s-ruins", kind), buildingPiece(b))
			})

			t.Run("and it is in the atlas", func(t *testing.T) {
				atlases, err := assets.LoadKits()
				require.NoError(t, err)
				_, ok := atlases[0].Sprite(buildingPiece(b), 0)
				assert.True(t, ok)
			})
		})
	}

	t.Run("when a session has no storeys yet", func(t *testing.T) {
		b := &city.Building{Type: city.OfficeDistrict}

		t.Run("it should be one storey", func(t *testing.T) {
			assert.Equal(t, "botropolis/office-1", buildingPiece(b))
		})
	})

	t.Run("when a district has no type yet", func(t *testing.T) {
		b := &city.Building{Session: state.Session{Storeys: 2}}

		t.Run("it should be residential", func(t *testing.T) {
			assert.Equal(t, "botropolis/residential-2", buildingPiece(b))
		})
	})

	t.Run("when every kind is looked up in the atlas", func(t *testing.T) {
		atlases, err := assets.LoadKits()
		require.NoError(t, err)
		require.NotEmpty(t, atlases)

		t.Run("it should be a piece the pipeline cut", func(t *testing.T) {
			var missing []string
			for _, kind := range city.DistrictTypes {
				for n := 1; n <= state.MaxStoreys; n++ {
					name := buildingPiece(&city.Building{Type: kind, Session: state.Session{Storeys: n}})
					if _, ok := atlases[0].Sprite(name, 0); !ok {
						missing = append(missing, name)
					}
				}
			}
			assert.Empty(t, missing, "add them to PIECES in render.py and re-run make sprites")
		})
	})
}
