package measure

import (
	"fmt"
	"slices"
	"strings"
)

// Tally says what a run of measured ticks did and where: their median
// instructions, when every one was counted, and how many ended on each core
// (item 330). Instructions do not move with the core or the clock, so they
// tell a slower core from more work.
func Tally(costs []Cost) string {
	if len(costs) == 0 {
		return "no ticks"
	}
	var work string
	counted := make([]uint64, 0, len(costs))
	for _, c := range costs {
		if c.Counted {
			counted = append(counted, c.Instructions)
		}
	}
	if len(counted) == len(costs) {
		slices.Sort(counted)
		work = fmt.Sprintf("%.1f M instructions a tick (median of %d)", float64(counted[len(counted)/2])/1e6, len(costs))
	} else {
		ticks := "ticks"
		if len(costs) == 1 {
			ticks = "tick"
		}
		work = fmt.Sprintf("instructions not counted (%d %s)", len(costs), ticks)
	}
	on := map[int]int{}
	for _, c := range costs {
		on[c.EndedOn]++
	}
	cores := make([]int, 0, len(on))
	for core := range on {
		cores = append(cores, core)
	}
	slices.Sort(cores)
	where := make([]string, 0, len(cores))
	for _, core := range cores {
		name := fmt.Sprint(core)
		if core < 0 {
			name = "unknown"
		}
		where = append(where, fmt.Sprintf("%s×%d", name, on[core]))
	}
	return work + ", ended on cores " + strings.Join(where, " ")
}
