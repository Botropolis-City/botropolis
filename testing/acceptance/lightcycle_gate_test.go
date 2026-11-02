//go:build linux

package acceptance_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 329: on a shared runner the light cycle reports its settled memory
// and does not gate, as the stress city does; the reference machine gates.
func TestTheLightCycleGate(t *testing.T) {
	cases := []struct {
		when  string
		env   map[string]string
		gates bool
	}{
		{"when CI is not set", map[string]string{}, true},
		{"when CI is set", map[string]string{"CI": "true"}, false},
	}
	for _, c := range cases {
		t.Run(c.when, func(t *testing.T) {
			gate := stressGateFor(func(k string) string { return c.env[k] })

			t.Run("and the second night settles 8 MiB over the first", func(t *testing.T) {
				r := &barRecorder{}
				holdCycle(r, gate, "night", "images", 100<<20, 108<<20)

				t.Run(fmt.Sprintf("it should fail: %v", c.gates), func(t *testing.T) {
					assert.Equal(t, c.gates, r.failed)
				})

				if !c.gates {
					t.Run("it should log both readings and the margin", func(t *testing.T) {
						assert.Contains(t, strings.Join(r.logs, "\n"), "the second night's images settled at 108 MiB against the first's 100 MiB, the margin 4 MiB")
					})
				}
			})

			t.Run("and the second night settles within 4 MiB of the first", func(t *testing.T) {
				r := &barRecorder{}
				holdCycle(r, gate, "night", "images", 100<<20, 103<<20)

				t.Run("it should not fail", func(t *testing.T) {
					assert.False(t, r.failed)
				})
			})
		})
	}
}
