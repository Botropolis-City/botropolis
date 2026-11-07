package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 351: City Hall's board counts its groups in words that agree
// with the count, as the strip's do (item 171).
func TestBoardCount(t *testing.T) {
	for _, tc := range []struct {
		groups int
		want   string
	}{
		{1, "1 group"},
		{3, "3 groups"},
	} {
		t.Run("when the board has "+tc.want, func(t *testing.T) {
			t.Run("it should say "+tc.want, func(t *testing.T) {
				assert.Equal(t, tc.want, boardStatus(make([]boardRow, tc.groups)))
			})
		})
	}
}
