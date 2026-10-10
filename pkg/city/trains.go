package city

import (
	"fmt"
	"hash/fnv"
	"math"

	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/plan"
)

// The train is the ledger: one train per model on the freight loop,
// with a wagon per unit of tokens spent over the day, so the length of
// each train is that model's share of the spend. Decided 2026-09-18.

// MaxWagons is the longest train; the wagon unit grows to keep it.
const MaxWagons = 6

// Train is one model's spend over the window, on the loop.
type Train struct {
	Model     string
	Tokens    int64
	Unit      int64
	Wagons    int
	CostUSD   float64
	CostKnown bool
	Window    Window
	// Hue picks the locomotive and wagon colour, one of ContainerHues.
	Hue int
}

// TrainUnit is the tokens one wagon stands for: the smallest 1-2-5 step
// from a thousand that keeps the biggest train to MaxWagons.
func TrainUnit(maxTokens int64) int64 {
	steps := []int64{1, 2, 5}
	for k := 0; ; k++ {
		for _, s := range steps {
			unit := s * int64(math.Pow10(3+k))
			if maxTokens <= unit*MaxWagons {
				return unit
			}
		}
	}
}

func trains(b Breakdown) []*Train {
	if len(b.ByModel) == 0 {
		return nil
	}
	unit := TrainUnit(b.ByModel[0].Tokens)
	out := make([]*Train, 0, len(b.ByModel))
	for _, share := range b.ByModel {
		if share.Tokens <= 0 {
			continue
		}
		wagons := int((share.Tokens + unit - 1) / unit)
		if wagons < 1 {
			wagons = 1
		}
		out = append(out, &Train{
			Model: share.Key, Tokens: share.Tokens, Unit: unit, Wagons: wagons,
			CostUSD: share.CostUSD, CostKnown: share.CostKnown, Window: b.Window, Hue: modelHue(share.Key),
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
		fmt.Sprintf("tokens   %s in %s", format.Tokens(float64(t.Tokens)), t.Window),
		fmt.Sprintf("wagons   %d, one per %s tokens", t.Wagons, format.Tokens(float64(t.Unit))),
		"cost     " + format.USD(t.CostUSD, t.CostKnown) + " (pro-rated)",
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
