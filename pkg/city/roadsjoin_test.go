package city_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 108, Aria 2026-10-18: "The road should always attach to any other
// existing road." Wherever two roads meet, they join with a T or a
// crossing.
func TestRoadsJoin(t *testing.T) {
	names := []string{"a", "b", "c", "d", "e", "f"}
	var cases [][][2]string
	var all [][2]string
	for i := range names {
		for j := i + 1; j < len(names); j++ {
			cases = append(cases, [][2]string{{names[i], names[j]}})
			all = append(all, [2]string{names[i], names[j]})
		}
	}
	cases = append(cases, all)
	opposite := map[int]int{city.DirN: city.DirS, city.DirS: city.DirN, city.DirE: city.DirW, city.DirW: city.DirE}
	steps := map[int]city.Cell{city.DirN: {Col: 0, Row: -1}, city.DirS: {Col: 0, Row: 1}, city.DirE: {Col: 1, Row: 0}, city.DirW: {Col: -1, Row: 0}}

	for _, roads := range cases {
		t.Run(fmt.Sprintf("when the roads are %v", roads), func(t *testing.T) {
			snap := snapshot(sixDistricts()...)
			snap.Roads = roadsBetween(roads...)
			c := city.Build(snap, city.NewLayout())
			laid := map[city.Cell]int{}
			for _, sc := range c.StreetCells {
				laid[sc.Cell] = sc.Mask
			}

			t.Run("it should join every two laid cells that touch, both ways", func(t *testing.T) {
				var apart []string
				for cell, mask := range laid {
					for dir, step := range steps {
						next, ok := laid[city.Cell{Col: cell.Col + step.Col, Row: cell.Row + step.Row}]
						if ok && (mask&dir == 0 || next&opposite[dir] == 0) {
							apart = append(apart, fmt.Sprintf("%v towards %d", cell, dir))
						}
					}
				}
				assert.Empty(t, apart)
			})

		})
	}
}
