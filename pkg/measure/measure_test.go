package measure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCounters(t *testing.T) {
	t.Run("when a counter is bumped once", func(t *testing.T) {
		before := Read(TranscriptsOpened)
		Count(TranscriptsOpened)

		t.Run("it should read back one more than before", func(t *testing.T) {
			assert.Equal(t, before+1, Read(TranscriptsOpened))
		})
	})

	t.Run("when a counter is bumped by a number of bytes", func(t *testing.T) {
		before := Read(BytesRead)
		Add(BytesRead, 4096)

		t.Run("it should read back that many more", func(t *testing.T) {
			assert.Equal(t, before+4096, Read(BytesRead))
		})
	})

	t.Run("when a conversation is read", func(t *testing.T) {
		before := Take()
		Count(ConversationsRead)

		t.Run("it should count it apart from the transcripts opened", func(t *testing.T) {
			assert.Equal(t, Work{ConversationsRead: 1}, Take().Since(before))
		})
	})

	t.Run("when one counter is bumped", func(t *testing.T) {
		before := Take()
		Count(SessionScans)
		after := Take()

		t.Run("it should leave the others where they were", func(t *testing.T) {
			assert.Equal(t, Work{SessionScans: 1}, after.Since(before))
		})
	})
}
