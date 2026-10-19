package proto_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/proto"
)

// Items 149 and 152: the client and the daemon exchange versions, and
// any difference is refused, naming both.
func TestVersionCheck(t *testing.T) {
	cases := []struct {
		scenario string
		daemon   proto.Peer
		ours     proto.Peer
		refusal  string
		newer    bool
	}{
		{"when both are the same release", proto.Peer{Release: "1.0.0", Protocol: 1}, proto.Peer{Release: "1.0.0", Protocol: 1}, "", false},
		{"when both are dev builds", proto.Peer{Release: "dev", Protocol: 1}, proto.Peer{Release: "dev", Protocol: 1}, "", false},
		{"when the daemon is an older release", proto.Peer{Release: "0.2.2", Protocol: 1}, proto.Peer{Release: "0.2.3", Protocol: 1},
			"the Botropolis city and its daemon must be running the same version (daemon 0.2.2, city 0.2.3)", false},
		{"when the daemon is a newer release", proto.Peer{Release: "0.2.10", Protocol: 1}, proto.Peer{Release: "0.2.9", Protocol: 1},
			"the Botropolis city and its daemon must be running the same version (daemon 0.2.10, city 0.2.9)", true},
		{"when the daemon is from before the exchange", proto.Peer{}, proto.Peer{Release: "0.2.3", Protocol: 1},
			"the Botropolis city and its daemon must be running the same version (daemon older than 0.2.3, city 0.2.3)", false},
		{"when the releases cannot be ordered", proto.Peer{Release: "dev", Protocol: 1}, proto.Peer{Release: "0.2.3", Protocol: 1},
			"the Botropolis city and its daemon must be running the same version (daemon dev, city 0.2.3)", false},
		{"when the daemon speaks an older protocol", proto.Peer{Release: "0.2.2", Protocol: 1}, proto.Peer{Release: "0.3.0", Protocol: 2},
			"the Botropolis city and its daemon must be running the same version (daemon 0.2.2, city 0.3.0)", false},
		{"when the daemon speaks a newer protocol", proto.Peer{Release: "0.3.0", Protocol: 2}, proto.Peer{Release: "0.2.2", Protocol: 1},
			"the Botropolis city and its daemon must be running the same version (daemon 0.3.0, city 0.2.2)", true},
		{"when one release speaks two protocols", proto.Peer{Release: "dev", Protocol: 2}, proto.Peer{Release: "dev", Protocol: 1},
			"the Botropolis city and its daemon must be running the same version (daemon dev protocol 2, city dev protocol 1)", true},
	}
	for _, c := range cases {
		t.Run(c.scenario, func(t *testing.T) {
			err := proto.Compare(c.daemon, c.ours)

			if c.refusal == "" {
				t.Run("it should not refuse", func(t *testing.T) {
					assert.NoError(t, err)
				})
				return
			}

			var mismatch *proto.Mismatch
			isMismatch := errors.As(err, &mismatch)

			t.Run("it should refuse, naming both", func(t *testing.T) {
				assert.EqualError(t, err, c.refusal)
			})

			t.Run("it should refuse with a mismatch", func(t *testing.T) {
				assert.True(t, isMismatch)
			})

			if isMismatch {
				t.Run("it should say whether the daemon is ahead", func(t *testing.T) {
					assert.Equal(t, c.newer, mismatch.Newer())
				})
			}
		})
	}

	t.Run("when a request and a response carry a version", func(t *testing.T) {
		peer := proto.Peer{Release: "0.2.3", Protocol: 1}
		var buf bytes.Buffer
		require.NoError(t, proto.Write(&buf, proto.Request{Op: proto.OpSnapshot, Peer: peer}))
		require.NoError(t, proto.Write(&buf, proto.Response{Peer: peer}))
		var request proto.Request
		var response proto.Response
		decoder := proto.NewDecoder(&buf)
		require.NoError(t, decoder.Decode(&request))
		require.NoError(t, decoder.Decode(&response))

		t.Run("it should round-trip the request's", func(t *testing.T) {
			assert.Equal(t, peer, request.Peer)
		})

		t.Run("it should round-trip the response's", func(t *testing.T) {
			assert.Equal(t, peer, response.Peer)
		})
	})
}
