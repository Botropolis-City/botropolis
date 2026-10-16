package daemon_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/proto"
)

func TestServeFind(t *testing.T) {
	t.Run("when a client sends find", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		client, err := proto.Dial(serve(t, d))
		require.NoError(t, err)
		defer func() { _ = client.Close() }()
		hits, err := client.Find(find.Query{Words: "ci queue", Limit: 5})
		require.NoError(t, err)

		t.Run("it should answer with the index's hits", func(t *testing.T) {
			var ids []string
			for _, h := range hits {
				ids = append(ids, h.SessionID)
			}
			assert.Equal(t, []string{sid}, ids)
		})
	})
}
