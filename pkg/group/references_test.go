package group_test

import (
	"testing"

	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
)

func TestReferences(t *testing.T) {
	cases := []struct {
		scenario string
		text     string
		want     []string
	}{
		{"a ticket key", "pick up PROJ-123 today", []string{"PROJ-123"}},
		{"a ticket key at the end of a sentence", "this is for LOG-88.", []string{"LOG-88"}},
		{"a lower-case key", "pick up proj-123 today", nil},
		{"a standard named like a key", "UTF-8, SHA-256, ISO-8601 and RFC-9110", nil},
		{"a key inside a longer word", "xPROJ-123", nil},
		{"a pull-request url", "see https://github.com/acme/ratelimit/pull/41 please", []string{"acme/ratelimit#41"}},
		{"a merge-request url", "https://gitlab.com/acme/infra/-/merge_requests/310", []string{"acme/infra#310"}},
		{"a Depends-On line", "bump it\nDepends-On: acme/ratelimit v1.4.0\n", []string{"acme/ratelimit v1.4.0"}},
		{"the same key twice", "PROJ-123 and again PROJ-123", []string{"PROJ-123"}},
		{"nothing", "fix the test", nil},
	}
	for _, c := range cases {
		t.Run("when the text holds "+c.scenario, func(t *testing.T) {
			t.Run("it should find exactly its references", func(t *testing.T) {
				assert.Equal(t, c.want, group.References(c.text))
			})
		})
	}
}
