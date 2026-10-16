package state_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/state"
)

func TestRung(t *testing.T) {
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	day := 24 * time.Hour

	t.Run("when the session is live", func(t *testing.T) {
		for _, st := range []state.State{state.NeedsYou, state.Working, state.Waiting, state.Unattended, state.Empty} {
			t.Run("and it is "+string(st), func(t *testing.T) {
				s := state.Session{State: st, LastActivity: now.Add(-90 * day)}

				t.Run("it should be on the live rung", func(t *testing.T) {
					assert.Equal(t, state.RungLive, s.Rung(now, state.DefaultThresholds))
				})
			})
		}
	})

	t.Run("when the session is stopped", func(t *testing.T) {
		for _, thresholds := range []state.Thresholds{
			state.DefaultThresholds,
			{Shuttered: 3 * day, Overgrown: 10 * day},
		} {
			cases := []struct {
				name  string
				quiet time.Duration
				want  state.Rung
			}{
				{"and it was last active less than the shuttered threshold ago", thresholds.Shuttered - time.Minute, state.RungResting},
				{"and it was last active exactly the shuttered threshold ago", thresholds.Shuttered, state.RungShuttered},
				{"and it was last active between the two thresholds", (thresholds.Shuttered + thresholds.Overgrown) / 2, state.RungShuttered},
				{"and it was last active exactly the overgrown threshold ago", thresholds.Overgrown, state.RungOvergrown},
				{"and it was last active long after the overgrown threshold", 4 * thresholds.Overgrown, state.RungOvergrown},
			}
			t.Run("and the thresholds are "+thresholds.Shuttered.String()+" and "+thresholds.Overgrown.String(), func(t *testing.T) {
				for _, c := range cases {
					t.Run(c.name, func(t *testing.T) {
						s := state.Session{State: state.Resting, LastActivity: now.Add(-c.quiet)}

						t.Run("it should be "+string(c.want), func(t *testing.T) {
							assert.Equal(t, c.want, s.Rung(now, thresholds))
						})
					})
				}
			})
		}

		t.Run("and its last activity is unknown", func(t *testing.T) {
			s := state.Session{State: state.Resting}

			t.Run("it should be resting", func(t *testing.T) {
				assert.Equal(t, state.RungResting, s.Rung(now, state.DefaultThresholds))
			})
		})

		t.Run("and its last activity is in the future", func(t *testing.T) {
			s := state.Session{State: state.Resting, LastActivity: now.Add(time.Hour)}

			t.Run("it should be resting", func(t *testing.T) {
				assert.Equal(t, state.RungResting, s.Rung(now, state.DefaultThresholds))
			})
		})
	})

	t.Run("when the defaults are read", func(t *testing.T) {
		t.Run("it should shutter after a week", func(t *testing.T) {
			assert.Equal(t, 7*day, state.DefaultThresholds.Shuttered)
		})

		t.Run("it should overgrow after thirty days", func(t *testing.T) {
			assert.Equal(t, 30*day, state.DefaultThresholds.Overgrown)
		})
	})
}
