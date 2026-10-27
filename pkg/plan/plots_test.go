package plan_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/plan"
)

// The plaza had two deciders and no referee: the plan planted a bush or
// a planter on every cell of the rim, and pkg/city dropped the power
// plant, the hall and the library onto the same plaza afterwards from
// the plaza's own rectangle. Nothing made them agree, so planters grew
// out of the concrete and a bush floated beside the cooling tower.
//
// The plan reserves the plots now, and plants round them. Whoever puts a
// building on the plaza takes the plot the plan set aside.
func TestPlazaPlotsAreLeftClear(t *testing.T) {
	p := plan.Make(plan.Input{Districts: live(6)}, plan.NewMemory())
	require.NotZero(t, p.Plaza.Cols, "no plaza to test")

	plots := []plan.Block{p.Plots.Plant, p.Plots.Hall, p.Plots.Library, p.Plots.Clinic}

	// Item 280: the clinic, where what needs you stands, takes the
	// plaza's fourth corner.
	t.Run("when the clinic's plot is reserved", func(t *testing.T) {
		t.Run("it should take the plaza's bottom-right corner", func(t *testing.T) {
			assert.Equal(t, plan.Cell{Col: p.Plaza.Min.Col + p.Plaza.Cols - p.Plots.Clinic.Cols, Row: p.Plaza.Min.Row + p.Plaza.Rows - p.Plots.Clinic.Rows}, p.Plots.Clinic.Min)
		})

		t.Run("it should be one of the plots the planting keeps off", func(t *testing.T) {
			assert.Contains(t, p.Plots.All(), p.Plots.Clinic)
		})
	})

	t.Run("when the fountain is placed", func(t *testing.T) {
		t.Run("it should stand on no plot", func(t *testing.T) {
			assert.False(t, p.Plots.Taken(p.Fountain))
		})
	})

	t.Run("when the plaza is planted", func(t *testing.T) {
		t.Run("it should reserve a plot for each civic building", func(t *testing.T) {
			for _, b := range plots {
				require.Greater(t, b.Cols, 0)
				require.Greater(t, b.Rows, 0)
			}
		})

		t.Run("it should keep the plots inside the plaza", func(t *testing.T) {
			for _, b := range plots {
				require.True(t, p.Plaza.Contains(b.Min), "plot at %v is off the plaza", b.Min)
				last := plan.Cell{Col: b.Min.Col + b.Cols - 1, Row: b.Min.Row + b.Rows - 1}
				require.True(t, p.Plaza.Contains(last), "plot ending %v is off the plaza", last)
			}
		})

		t.Run("it should plant nothing on a reserved plot", func(t *testing.T) {
			for _, tree := range p.Trees {
				for _, b := range plots {
					require.False(t, b.Contains(tree.Cell),
						"%v planted on a reserved plot at %v", tree.Cell, b.Min)
				}
			}
		})

		t.Run("it should plant nothing on the fountain", func(t *testing.T) {
			for _, tree := range p.Trees {
				require.NotEqual(t, p.Fountain, tree.Cell)
			}
		})

		t.Run("it should still plant the rim it can", func(t *testing.T) {
			planted := 0
			for _, tree := range p.Trees {
				if p.Plaza.Contains(tree.Cell) {
					planted++
				}
			}
			assert.Greater(t, planted, 0, "the plaza lost all its planting")
		})
	})

	t.Run("when two plots are compared", func(t *testing.T) {
		t.Run("it should not put two buildings on the same ground", func(t *testing.T) {
			for i := range plots {
				for j := i + 1; j < len(plots); j++ {
					for col := plots[i].Min.Col; col < plots[i].Min.Col+plots[i].Cols; col++ {
						for row := plots[i].Min.Row; row < plots[i].Min.Row+plots[i].Rows; row++ {
							require.False(t, plots[j].Contains(plan.Cell{Col: col, Row: row}),
								"plots %d and %d overlap at %d,%d", i, j, col, row)
						}
					}
				}
			}
		})
	})
}

// Item 86: the depot is ground spoken for, as the yard before it was.
func TestNothingIsPlantedOnTheDepot(t *testing.T) {
	p := plan.Make(plan.Input{Districts: live(4), Towers: 2, DepotRows: 2}, plan.NewMemory())
	require.Positive(t, p.Depot.Rows)

	t.Run("when the plan has a depot", func(t *testing.T) {
		t.Run("it should plant nothing on it", func(t *testing.T) {
			for _, tree := range p.Trees {
				require.False(t, p.Depot.Contains(tree.Cell), "%v planted on the depot", tree.Cell)
			}
		})
	})
}

// Item 86: the depot's block is as wide as it asks for, from the plan's
// west edge, and never wider than the plan.
func TestDepotBlock(t *testing.T) {
	full := plan.Make(plan.Input{Districts: live(4), DepotRows: 1}, plan.NewMemory())

	t.Run("when the depot asks for three columns", func(t *testing.T) {
		p := plan.Make(plan.Input{Districts: live(4), DepotRows: 1, DepotCols: 3}, plan.NewMemory())

		t.Run("its block should be three columns wide", func(t *testing.T) {
			assert.Equal(t, 3, p.Depot.Cols)
		})

		t.Run("it should start at the plan's west edge", func(t *testing.T) {
			assert.Equal(t, full.Depot.Min.Col, p.Depot.Min.Col)
		})
	})

	t.Run("when it asks for more than the plan's width", func(t *testing.T) {
		p := plan.Make(plan.Input{Districts: live(4), DepotRows: 1, DepotCols: 999}, plan.NewMemory())

		t.Run("it should be the plan's width", func(t *testing.T) {
			assert.Equal(t, full.Depot.Cols, p.Depot.Cols)
		})
	})
}

// The class, not the instance: nothing may be planted on ground a
// building will stand on, wherever that ground is. The plaza was where
// it showed, but a district block is the same promise — the plan says a
// building goes there, so the plan must not also say a bush does.
func TestNothingIsPlantedOnBuildingGround(t *testing.T) {
	p := plan.Make(plan.Input{Districts: live(6)}, plan.NewMemory())

	t.Run("when the plan has finished placing everything", func(t *testing.T) {
		t.Run("it should plant nothing inside a district's block", func(t *testing.T) {
			for _, tree := range p.Trees {
				for root, b := range p.Blocks {
					require.False(t, b.Contains(tree.Cell),
						"%v planted inside district %s at %v", tree.Cell, root, b.Min)
				}
			}
		})

		t.Run("it should plant nothing on a tower's cell", func(t *testing.T) {
			for _, tree := range p.Trees {
				for _, tower := range p.Towers {
					require.NotEqual(t, tower, tree.Cell, "planted on a tower")
				}
			}
		})
	})
}
