package render

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/assets"
	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/state"
)

// Item 273, slice 2: a scripted run, shown, is a street stall in its
// district's own kind, and carries none of what storeys draw.
func TestStall(t *testing.T) {
	stall := func(kind city.DistrictType) *city.Building {
		b := kindBuilding(kind, 4)
		b.Session.Entrypoint = "sdk-cli"
		b.Session.Compactions, b.Session.Spire, b.Session.ContextPercent = 5, 3, 90
		return b
	}

	for kind, want := range map[city.DistrictType]string{
		city.ResidentialDistrict: "botropolis/residential-stall",
		city.ShoppingDistrict:    "botropolis/shopping-stall",
		city.OfficeDistrict:      "botropolis/office-stall",
		city.IndustrialDistrict:  "botropolis/industrial-stall",
	} {
		t.Run("when a scripted session stands in a "+string(kind)+" district", func(t *testing.T) {
			t.Run("it should be drawn as "+want, func(t *testing.T) {
				assert.Equal(t, want, buildingPiece(stall(kind)))
			})
		})
	}

	t.Run("when every stall and customer is looked up in the atlas", func(t *testing.T) {
		atlases, err := assets.LoadKits()
		require.NoError(t, err)
		require.NotEmpty(t, atlases)

		t.Run("it should be a piece the pipeline cut", func(t *testing.T) {
			var missing []string
			for _, name := range append(append([]string{}, kitCustomers...), stallPieces()...) {
				if _, ok := atlases[0].Sprite(name, 0); !ok {
					missing = append(missing, name)
				}
			}
			assert.Empty(t, missing, "add them to PIECES in render.py and re-run make sprites")
		})
	})

	t.Run("when a scripted session on the map has subagents in flight", func(t *testing.T) {
		queued := func(t *testing.T, view city.View) int {
			t.Helper()
			scene := city.NewScene(city.NewLayout())
			scene.Resize(800, 600)
			scene.SetShown(city.Shown{Scripted: true})
			run := kindBuilding(city.ShoppingDistrict, 1).Session
			run.ID, run.CWD, run.Entrypoint, run.SubagentsInFlight = "run", "/src/pipeline", "sdk-cli", 3
			scene.SetSnapshot(state.Snapshot{Sessions: []state.Session{run}})
			scene.SetView(view)
			g := &Game{scene: scene}
			var screen *ebiten.Image
			return len(g.queues(&screen, scene.City(), scene.Camera()))
		}

		t.Run("it should draw one customer per subagent", func(t *testing.T) {
			assert.Equal(t, 3, queued(t, city.ViewAttention))
		})

		t.Run("and the view hides the subagents, it should draw none", func(t *testing.T) {
			assert.Zero(t, queued(t, city.ViewSpend))
		})
	})

	t.Run("when an interactive session stands there", func(t *testing.T) {
		t.Run("it should be drawn as its storeys' building", func(t *testing.T) {
			assert.Equal(t, "botropolis/office-4", buildingPiece(kindBuilding(city.OfficeDistrict, 4)))
		})
	})

	t.Run("when a scripted session is packed", func(t *testing.T) {
		b := stall(city.OfficeDistrict)
		b.Packed = true

		t.Run("it should still be a crate", func(t *testing.T) {
			assert.Contains(t, crates, buildingPiece(b))
		})
	})

	g := kitGame(t)
	cam := city.NewCamera()

	t.Run("when a scripted session has compacted, risen a spire and filled its context", func(t *testing.T) {
		t.Run("it should draw no courses", func(t *testing.T) {
			assert.Zero(t, g.courseLift(cam, stall(city.OfficeDistrict)))
		})

		t.Run("it should raise no spire", func(t *testing.T) {
			assert.Zero(t, spireSegments(stall(city.OfficeDistrict)))
		})

		t.Run("it should set no skip at the kerb", func(t *testing.T) {
			assert.Zero(t, shownSkip(stall(city.OfficeDistrict), city.Job{}, false))
		})

		t.Run("it should light no windows", func(t *testing.T) {
			b := stall(city.OfficeDistrict)
			b.Lit = true
			_, lit := g.windowLights(cam, b, 1, 1)
			assert.False(t, lit)
		})
	})
}
