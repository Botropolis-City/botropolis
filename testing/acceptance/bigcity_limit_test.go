package acceptance_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Item 303: the release runner holds the first map to its own limit.
func TestTheFirstMapLimit(t *testing.T) {
	cases := []struct {
		when  string
		env   map[string]string
		limit time.Duration
	}{
		{"when CI is not set", map[string]string{}, 5 * time.Second},
		{"when CI is true", map[string]string{"CI": "true"}, 9 * time.Second},
	}
	for _, c := range cases {
		t.Run(c.when, func(t *testing.T) {
			limit, why := firstMapLimit(func(k string) string { return c.env[k] })

			t.Run("it should hold the first frame to "+c.limit.String(), func(t *testing.T) {
				assert.Equal(t, c.limit, limit)
			})

			t.Run("it should name the limit in its reason", func(t *testing.T) {
				assert.Contains(t, why, c.limit.String())
			})
		})
	}
}
