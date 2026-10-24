package render

import (
	"fmt"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 159: the opening panel fills as the pages of the level in use go
// up; item 162 puts up only that level, 2 pages at the usual zoom and 3
// from zoom 1.75.
func TestKitProgress(t *testing.T) {
	for _, tc := range []struct {
		zoom  float64
		pages int
	}{
		{1, 2},
		{2, 3},
	} {
		t.Run(fmt.Sprintf("when the city opens at zoom %v", tc.zoom), func(t *testing.T) {
			k, err := loadKitsLazily()
			require.NoError(t, err)

			t.Run("and no page is up", func(t *testing.T) {
				t.Run(fmt.Sprintf("it should have none of %d up", tc.pages), func(t *testing.T) {
					done, total := k.progress(tc.zoom)
					assert.Equal(t, [2]int{0, tc.pages}, [2]int{done, total})
				})
			})

			t.Run("and one page is up", func(t *testing.T) {
				a := &k.atlases[k.index(tc.zoom)]
				a.pages = append(a.pages, (*ebiten.Image)(nil))

				t.Run("it should count it", func(t *testing.T) {
					done, _ := k.progress(tc.zoom)
					assert.Equal(t, 1, done)
				})
			})
		})
	}
}
