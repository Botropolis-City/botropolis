//go:build linux

package acceptance_test

import (
	"fmt"
	"strings"
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

type barRecorder struct {
	failed bool
	logs   []string
}

func (r *barRecorder) Helper()               {}
func (r *barRecorder) Errorf(string, ...any) { r.failed = true }
func (r *barRecorder) Logf(format string, args ...any) {
	r.logs = append(r.logs, fmt.Sprintf(format, args...))
}

// Item 305: on a shared runner the stress city's bars report and do not
// gate; the reference machine's quiet run is their gate.
func TestTheStressCityGate(t *testing.T) {
	cases := []struct {
		when  string
		env   map[string]string
		gates bool
		mode  string
	}{
		{"when CI is not set", map[string]string{}, true, "gates"},
		{"when CI is set", map[string]string{"CI": "true"}, false, "reports and does not gate"},
	}
	for _, c := range cases {
		t.Run(c.when, func(t *testing.T) {
			gate := stressGateFor(func(k string) string { return c.env[k] })

			t.Run(fmt.Sprintf("it should gate: %v", c.gates), func(t *testing.T) {
				assert.Equal(t, c.gates, gate.gates)
			})

			t.Run("it should say so in its mode", func(t *testing.T) {
				assert.Contains(t, gate.mode, c.mode)
			})

			t.Run("and a measure is over its bar", func(t *testing.T) {
				r := &barRecorder{}
				holdToBar(r, gate, false, "took %v, the bar %v", 22*time.Millisecond, 16*time.Millisecond)

				t.Run(fmt.Sprintf("it should fail: %v", c.gates), func(t *testing.T) {
					assert.Equal(t, c.gates, r.failed)
				})

				if !c.gates {
					t.Run("it should log the measure", func(t *testing.T) {
						assert.Contains(t, strings.Join(r.logs, "\n"), "took 22ms, the bar 16ms")
					})
				}
			})

			t.Run("and a measure is within its bar", func(t *testing.T) {
				r := &barRecorder{}
				holdToBar(r, gate, true, "took %v", 15*time.Millisecond)

				t.Run("it should not fail", func(t *testing.T) {
					assert.False(t, r.failed)
				})
			})

			t.Run("and the city drew no frame within the window", func(t *testing.T) {
				r := &barRecorder{}
				holdToFrames(r, gate, nil)

				t.Run(fmt.Sprintf("it should fail: %v", c.gates), func(t *testing.T) {
					assert.Equal(t, c.gates, r.failed)
				})

				if !c.gates {
					t.Run("it should log that nothing was measured", func(t *testing.T) {
						assert.Contains(t, strings.Join(r.logs, "\n"), "no frames measured, reported and not gated: the runner drew none within the window")
					})
				}
			})

			t.Run("and the city drew frames within the window", func(t *testing.T) {
				r := &barRecorder{}
				holdToFrames(r, gate, []float64{15.2})

				t.Run("it should not fail", func(t *testing.T) {
					assert.False(t, r.failed)
				})
			})
		})
	}
}
