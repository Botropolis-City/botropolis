package proto_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/proto"
)

// Item 149: the client and the daemon exchange versions. A release
// apart is said once and works; a protocol apart is refused.
func TestVersionCheck(t *testing.T) {
	cases := []struct {
		scenario string
		daemon   proto.Peer
		ours     proto.Peer
		notice   string
		refusal  string
	}{
		{"when both are the same release", proto.Peer{Release: "1.0.0", Protocol: 1}, proto.Peer{Release: "1.0.0", Protocol: 1}, "", ""},
		{"when both are dev builds", proto.Peer{Release: "dev", Protocol: 1}, proto.Peer{Release: "dev", Protocol: 1}, "", ""},
		{"when the daemon is an older release of the same protocol", proto.Peer{Release: "0.2.2", Protocol: 1}, proto.Peer{Release: "0.2.3", Protocol: 1},
			"daemon is 0.2.2, this is 0.2.3: restart it to update", ""},
		{"when the daemon is a newer release of the same protocol", proto.Peer{Release: "0.2.10", Protocol: 1}, proto.Peer{Release: "0.2.9", Protocol: 1},
			"daemon is 0.2.10, this is 0.2.9: restart this to update", ""},
		{"when the daemon is from before the exchange", proto.Peer{}, proto.Peer{Release: "0.2.3", Protocol: 1},
			"daemon is older than 0.2.3: restart it to update", ""},
		{"when the releases cannot be ordered", proto.Peer{Release: "dev", Protocol: 1}, proto.Peer{Release: "0.2.3", Protocol: 1},
			"daemon is dev, this is 0.2.3: restart it to update", ""},
		{"when the daemon speaks an older protocol", proto.Peer{Release: "0.2.2", Protocol: 1}, proto.Peer{Release: "0.3.0", Protocol: 2},
			"", "restart the daemon: it is 0.2.2, this is 0.3.0"},
		{"when the daemon is from before the exchange and the protocol has moved on", proto.Peer{}, proto.Peer{Release: "0.3.0", Protocol: 2},
			"", "restart the daemon: it is older than 0.3.0"},
		{"when the daemon speaks a newer protocol", proto.Peer{Release: "0.3.0", Protocol: 2}, proto.Peer{Release: "0.2.2", Protocol: 1},
			"", "this is 0.2.2 and the daemon is 0.3.0: restart this"},
	}
	for _, c := range cases {
		t.Run(c.scenario, func(t *testing.T) {
			notice, err := proto.Compare(c.daemon, c.ours)

			t.Run("it should give the right notice", func(t *testing.T) {
				assert.Equal(t, c.notice, notice)
			})

			if c.refusal == "" {
				t.Run("it should not refuse", func(t *testing.T) {
					assert.NoError(t, err)
				})
			} else {
				t.Run("it should refuse, saying what to restart", func(t *testing.T) {
					assert.EqualError(t, err, c.refusal)
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

	t.Run("when the same notice is reported twice", func(t *testing.T) {
		var heard []string
		restore := proto.OnNotice(func(n string) { heard = append(heard, n) })
		defer restore()
		proto.Report("daemon is 0.2.2, this is 0.2.3: restart it to update")
		proto.Report("daemon is 0.2.2, this is 0.2.3: restart it to update")

		t.Run("it should be heard once", func(t *testing.T) {
			assert.Len(t, heard, 1)
		})
	})
}
