package group_test

import (
	"strings"
	"testing"

	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
)

const plan = `Plan for the headers change. First add the remaining and reset headers to the limiter
response in the library, with tests for both the fixed window and the sliding window. Then release
the library as a minor version and bump it in the service, where the middleware reads the new
fields and copies them onto every response. Finally the gateway configuration needs the headers
allowed through, and the dashboards should chart how often clients hit the limit after the change.`

func TestPasted(t *testing.T) {
	t.Run("when two sessions paste the same long text with small edits", func(t *testing.T) {
		edited := strings.Replace(plan, "minor version", "patch version", 1) + "\nThanks."

		t.Run("it should match", func(t *testing.T) {
			assert.True(t, group.SamePaste(plan, edited))
		})
	})

	t.Run("when the pasted texts differ", func(t *testing.T) {
		other := `Notes on the log shipping work. The agent writes to a local buffer and a sidecar
forwards batches to the collector every few seconds, retrying with backoff when the collector is
unavailable. We need to cap the buffer on disk, drop the oldest batches first, and count what was
dropped so the operator can see it. The collector's schema also changed, so the field names move.`

		t.Run("it should not match", func(t *testing.T) {
			assert.False(t, group.SamePaste(plan, other))
		})
	})

	t.Run("when the text is short", func(t *testing.T) {
		t.Run("it should not count as pasted", func(t *testing.T) {
			assert.False(t, group.SamePaste("run the tests again please", "run the tests again please"))
		})
	})
}
