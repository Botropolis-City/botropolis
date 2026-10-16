package group_test

import (
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
)

func TestName(t *testing.T) {
	t.Run("when the members share a reference", func(t *testing.T) {
		result := detect(group.Memory{},
			member("a", "/w/lib", 0, "add rate-limit headers to the limiter for PROJ-123"),
			member("b", "/w/api", 3*time.Hour, "PROJ-123: read the rate-limit headers"))
		g, _ := result.GroupOf("a")

		t.Run("it should be named by the reference and the most distinctive words", func(t *testing.T) {
			assert.Equal(t, "PROJ-123 · rate-limit headers", g.Name)
		})

		t.Run("it should be anchored on the reference", func(t *testing.T) {
			assert.Equal(t, "PROJ-123", g.Anchor)
		})
	})

	t.Run("when the members share no reference", func(t *testing.T) {
		result := detect(group.Memory{},
			member("a", "/w/lib", 0, "add rate-limit headers to the limiter"),
			member("b", "/w/api", 5*time.Minute, "read the rate-limit headers"))
		g, _ := result.GroupOf("a")

		t.Run("it should be named by the most distinctive words alone", func(t *testing.T) {
			assert.Equal(t, "rate-limit headers", g.Name)
		})
	})

	t.Run("when the group was renamed", func(t *testing.T) {
		result := detect(group.Memory{Groups: []group.Stored{{ID: "g1", Name: "Headers", Renamed: true, Members: []string{"a"}}}},
			member("a", "/w/lib", 0, "PROJ-123"),
			member("b", "/w/api", 3*time.Hour, "PROJ-123"))
		g, _ := result.GroupOf("a")

		t.Run("it should keep the new name", func(t *testing.T) {
			assert.Equal(t, "Headers", g.Name)
		})
	})
}
