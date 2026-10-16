package group_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/group"
)

// Item 90: a word that wrongly joined two sessions can be muted.
func TestMuteWord(t *testing.T) {
	t.Run("when a word is muted", func(t *testing.T) {
		t.Run("memory should hold it", func(t *testing.T) {
			assert.Equal(t, []string{"kiln"}, group.Memory{}.WithMuted("kiln").Muted)
		})
	})

	t.Run("when it is muted twice", func(t *testing.T) {
		t.Run("memory should hold it once", func(t *testing.T) {
			assert.Equal(t, []string{"kiln"}, group.Memory{}.WithMuted("kiln").WithMuted("Kiln").Muted)
		})
	})

	t.Run("when a pair is joined only because a word strengthens their timing", func(t *testing.T) {
		pair := func() []group.Member {
			return []group.Member{
				member("a", "/w/lib", 0, "glaze the kiln shelves", "kiln firing schedule"),
				member("b", "/w/api", 10*time.Minute, "kiln temperature curve", "kiln shelves again"),
			}
		}
		g, ok := detect(group.Memory{}, pair()...).GroupOf("b")
		require.True(t, ok, "the word should join them before it is muted")
		require.Equal(t, group.SignalWording, g.Member("b").Reason.Signal)

		t.Run("and it is muted", func(t *testing.T) {
			_, joined := detect(group.Memory{}.WithMuted("kiln").WithMuted("shelves"), pair()...).GroupOf("b")

			t.Run("they should no longer join", func(t *testing.T) {
				assert.False(t, joined)
			})
		})
	})
}
