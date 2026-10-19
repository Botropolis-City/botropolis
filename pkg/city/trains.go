package city

import (
	"fmt"
	"hash/fnv"
	"math"

	"botropolis.city/botropolis/pkg/plan"
	"botropolis.city/botropolis/pkg/stats"
)

// The train is the ledger: one train per model on the freight loop,
// with a wagon per unit of cost over the day, so the length of each
// train is that model's share of the spend. Decided 2026-09-18; a wagon
// stands for dollars rather than tokens since item 116, because tokens
// of different models and counters are not priced alike. Trains run only
// on a priced breakdown, since only the store says which model sent
// each message.

// MaxWagons is the longest train; the wagon unit grows to keep it.
const MaxWagons = 6

// Train is one model's spend over the window, on the loop.
type Train struct {
	Model    string
	Counters stats.Counters
	Cost     stats.Cost
	// Unit is the dollars one wagon stands for.
	Unit   float64
	Wagons int
	Window Window
	// Hue picks the locomotive and wagon colour, one of ContainerHues.
	Hue int
}

// TrainUnit is the dollars one wagon stands for: the smallest 1-2-5
// step from a cent that keeps the dearest train to MaxWagons.
func TrainUnit(maxUSD float64) float64 {
	steps := []float64{1, 2, 5}
	for k := 0; ; k++ {
		for _, s := range steps {
			unit := s * math.Pow10(k-2)
			if maxUSD <= unit*MaxWagons*(1+1e-9) {
				return unit
			}
		}
	}
}

func trains(b Breakdown) []*Train {
	if !b.Priced || len(b.ByModel) == 0 {
		return nil
	}
	unit := TrainUnit(b.ByModel[0].Cost.High)
	out := make([]*Train, 0, len(b.ByModel))
	for _, share := range b.ByModel {
		if share.Cost.High <= 0 {
			continue
		}
		wagons := int(math.Ceil(share.Cost.High/unit - 1e-9))
		if wagons < 1 {
			wagons = 1
		}
		out = append(out, &Train{
			Model: share.Key, Counters: share.Counters, Cost: share.Cost, Unit: unit, Wagons: wagons,
			Window: b.Window, Hue: modelHue(share.Key),
		})
	}
	return out
}

func modelHue(model string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(model))
	return int(h.Sum32() % uint32(len(ContainerHues)))
}

func (t *Train) Card() Card {
	return Card{Title: t.Model, Lines: []string{
		stats.Headline(t.Cost, t.Counters.Messages) + " in " + t.Window.String(),
		stats.CountersLine(t.Counters),
		fmt.Sprintf("wagons   %d, one per $%.2f", t.Wagons, t.Unit),
	}}
}

// railPath is the loop as a closed polyline through its cells' centres.
func railPath(cells []plan.Cell) []Point {
	if len(cells) == 0 {
		return nil
	}
	path := make([]Point, 0, len(cells)+1)
	for _, c := range cells {
		path = append(path, toCell(c).Center())
	}
	return append(path, path[0])
}
