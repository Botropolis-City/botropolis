package proto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/proto"
)

// Item 152: whether a daemon is this build's release and protocol, which
// a restart waits for.
func TestSame(t *testing.T) {
	for _, c := range []struct {
		scenario string
		a, b     proto.Peer
		same     bool
	}{
		{"when release and protocol match", proto.Peer{Release: "0.2.3", Protocol: 1}, proto.Peer{Release: "0.2.3", Protocol: 1}, true},
		{"when an unstated protocol meets protocol 1", proto.Peer{Release: "0.2.3"}, proto.Peer{Release: "0.2.3", Protocol: 1}, true},
		{"when the releases differ", proto.Peer{Release: "0.2.2", Protocol: 1}, proto.Peer{Release: "0.2.3", Protocol: 1}, false},
		{"when the protocols differ", proto.Peer{Release: "0.2.3", Protocol: 2}, proto.Peer{Release: "0.2.3", Protocol: 1}, false},
		{"when one is from before the exchange", proto.Peer{}, proto.Peer{Release: "0.2.3", Protocol: 1}, false},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			t.Run("it should say whether they are the same", func(t *testing.T) {
				assert.Equal(t, c.same, proto.Same(c.a, c.b))
			})
		})
	}
}
