package state_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

func TestHeft(t *testing.T) {
	t.Run("when a session used input, output, cache creation and cache reads", func(t *testing.T) {
		u := claude.Usage{Input: 1_000, Output: 2_000, CacheCreate: 3_000, CacheRead: 40_000}

		t.Run("it should count cache reads at a tenth", func(t *testing.T) {
			assert.Equal(t, int64(10_000), state.Heft(u))
		})
	})
}

func evenly(n int) []int64 {
	hefts := make([]int64, n)
	for i := range hefts {
		hefts[i] = int64(i+1) * 1_000
	}
	return hefts
}

func TestNewScale(t *testing.T) {
	at := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)

	t.Run("when the sample is empty", func(t *testing.T) {
		scale := state.NewScale(nil, at)

		t.Run("it should cut at the starter lines", func(t *testing.T) {
			assert.Equal(t, state.StarterCuts, scale.Cuts)
		})
	})

	t.Run("when the sample has twenty or more sessions", func(t *testing.T) {
		t.Run("and the hefts are spread evenly", func(t *testing.T) {
			scale := state.NewScale(evenly(100), at)

			t.Run("it should cut at the user's own fifths", func(t *testing.T) {
				assert.Equal(t, [4]int64{21_000, 41_000, 61_000, 81_000}, scale.Cuts)
			})
		})

		t.Run("and the history is skewed", func(t *testing.T) {
			var hefts []int64
			for i := range 25 {
				hefts = append(hefts, int64(10_000+i*100))
			}
			for i := range 25 {
				hefts = append(hefts, int64(1_000_000*(i+1)))
			}
			for i := range 50 {
				hefts = append(hefts, int64(20_000+i*1_000))
			}
			scale := state.NewScale(hefts, at)

			t.Run("it should put a typical session at three storeys", func(t *testing.T) {
				assert.Equal(t, 3, scale.Storeys(45_000))
			})

			t.Run("it should put only the heaviest fifth at five storeys", func(t *testing.T) {
				five := 0
				for _, h := range hefts {
					if scale.Storeys(h) == 5 {
						five++
					}
				}
				assert.Equal(t, 20, five)
			})
		})

		t.Run("and one session is a thousand times heavier than the rest", func(t *testing.T) {
			hefts := evenly(25)
			without := state.NewScale(hefts, at)
			with := state.NewScale(append(append([]int64{}, hefts...), 25_000_000), at)

			t.Run("it should leave every other session's storey unchanged", func(t *testing.T) {
				var a, b []int
				for _, h := range hefts {
					a, b = append(a, without.Storeys(h)), append(b, with.Storeys(h))
				}
				assert.Equal(t, a, b)
			})
		})
	})

	t.Run("when the sample has fewer than twenty sessions", func(t *testing.T) {
		hefts := make([]int64, 10)
		for i := range hefts {
			hefts[i] = 100_000_000
		}
		scale := state.NewScale(hefts, at)

		t.Run("it should cut between the starter lines and the user's own", func(t *testing.T) {
			between := true
			for i, c := range scale.Cuts {
				between = between && c > state.StarterCuts[i] && c < 100_000_000
			}
			assert.True(t, between)
		})

		t.Run("and the sample has ten sessions", func(t *testing.T) {
			t.Run("it should take each cut point halfway between them, on a log scale", func(t *testing.T) {
				var want [4]int64
				for i, s := range state.StarterCuts {
					want[i] = int64(math.Round(math.Sqrt(float64(s) * 100_000_000)))
				}
				assert.Equal(t, want, scale.Cuts)
			})
		})
	})
}

func TestScaleStoreys(t *testing.T) {
	scale := state.Scale{Cuts: [4]int64{100, 200, 300, 400}}
	cases := []struct {
		name string
		heft int64
		want int
	}{
		{"when the heft is below the first cut", 99, 1},
		{"when the heft is at the first cut", 100, 2},
		{"when the heft is at the second cut", 200, 3},
		{"when the heft is at the third cut", 300, 4},
		{"when the heft is at the last cut", 400, 5},
		{"when the heft is far above the last cut", 400_000, 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Run("it should be the right number of storeys", func(t *testing.T) {
				assert.Equal(t, c.want, scale.Storeys(c.heft))
			})
		})
	}
}

func TestScaleStale(t *testing.T) {
	at := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	scale := state.NewScale(evenly(30), at)

	t.Run("when the scale is less than a day old", func(t *testing.T) {
		t.Run("it should not be stale", func(t *testing.T) {
			assert.False(t, scale.Stale(at.Add(23*time.Hour)))
		})
	})

	t.Run("when it is a day old", func(t *testing.T) {
		t.Run("it should be stale", func(t *testing.T) {
			assert.True(t, scale.Stale(at.Add(24*time.Hour)))
		})
	})
}

func heftSessions(hefts []int64) []state.Session {
	sessions := make([]state.Session, len(hefts))
	for i, h := range hefts {
		sessions[i] = state.Session{ID: string(rune('a'+i%26)) + string(rune('a'+i/26)), Heft: h}
	}
	return sessions
}

func storeysOf(sessions []state.Session, id string) int {
	for _, s := range sessions {
		if s.ID == id {
			return s.Storeys
		}
	}
	return 0
}

func TestHeights(t *testing.T) {
	at := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	big := func() []int64 {
		hefts := evenly(40)
		for i := range hefts {
			hefts[i] *= 1_000
		}
		return hefts
	}

	t.Run("when no session has been read whole yet", func(t *testing.T) {
		h := state.NewHeights()
		sessions := heftSessions(big())
		h.Apply(sessions, false, at)

		t.Run("it should draw on the starter lines", func(t *testing.T) {
			assert.Equal(t, state.StarterCuts, h.Scale().Cuts)
		})

		t.Run("and the whole read then finishes", func(t *testing.T) {
			sessions := heftSessions(big())
			h.Apply(sessions, true, at.Add(time.Minute))

			t.Run("it should let a storey the starter lines gave fall to the user's own", func(t *testing.T) {
				assert.Equal(t, 1, storeysOf(sessions, "aa"))
			})
		})
	})

	t.Run("when the whole read has finished", func(t *testing.T) {
		h := state.NewHeights()
		sessions := heftSessions(big())
		h.Apply(sessions, true, at)

		t.Run("it should draw on the user's own scale", func(t *testing.T) {
			assert.Equal(t, state.NewScale(big(), at).Cuts, h.Scale().Cuts)
		})

		t.Run("it should set each session's storeys", func(t *testing.T) {
			assert.Equal(t, 5, storeysOf(sessions, "nb"))
		})
	})

	// Item 110.
	t.Run("when the sample is stored hefts rather than the sessions shown", func(t *testing.T) {
		h := state.NewHeights()
		stored := evenly(30)
		for i := range stored {
			stored[i] *= 50
		}
		h.ApplyOn(heftSessions(big()), stored, true, at)

		t.Run("it should cut the scale from the stored hefts", func(t *testing.T) {
			assert.Equal(t, state.NewScale(stored, at).Cuts, h.Scale().Cuts)
		})
	})

	t.Run("when a session's heft grows past a cut", func(t *testing.T) {
		h := state.NewHeights()
		sessions := heftSessions(big())
		h.Apply(sessions, true, at)
		sessions = heftSessions(big())
		sessions[0].Heft = h.Scale().Cuts[0]
		h.Apply(sessions, true, at.Add(time.Minute))

		t.Run("it should gain a storey", func(t *testing.T) {
			assert.Equal(t, 2, storeysOf(sessions, "aa"))
		})
	})

	t.Run("when the daily recompute raises the cut points", func(t *testing.T) {
		h := state.NewHeights()
		h.Apply(heftSessions(big()), true, at)
		heavier := big()
		for i := range heavier {
			heavier[i] *= 10
		}
		heavier[39] = big()[39]
		sessions := heftSessions(heavier)
		h.Apply(sessions, true, at.Add(25*time.Hour))

		t.Run("it should keep every storey already reached", func(t *testing.T) {
			assert.Equal(t, 5, storeysOf(sessions, "nb"))
		})
	})

	t.Run("when the scale was computed less than a day ago", func(t *testing.T) {
		t.Run("and the heaviest session doubles", func(t *testing.T) {
			h := state.NewHeights()
			h.Apply(heftSessions(big()), true, at)
			before := h.Scale()
			hefts := big()
			hefts[39] *= 2
			h.Apply(heftSessions(hefts), true, at.Add(time.Hour))

			t.Run("it should not recompute the scale", func(t *testing.T) {
				assert.Equal(t, before, h.Scale())
			})
		})
	})

	t.Run("when a session is packed up and then unpacked", func(t *testing.T) {
		h := state.NewHeights()
		sessions := heftSessions(big())
		sessions[39].Packed = true
		h.Apply(sessions, true, at)
		sessions = heftSessions(big())
		h.Apply(sessions, true, at.Add(time.Minute))

		t.Run("it should keep its storeys", func(t *testing.T) {
			assert.Equal(t, 5, storeysOf(sessions, "nb"))
		})
	})

	t.Run("when a session's transcript is gone", func(t *testing.T) {
		t.Run("it should be left out of the sample", func(t *testing.T) {
			h := state.NewHeights()
			sessions := heftSessions(big())
			gone := append(sessions, state.Session{ID: "gone", ConversationGone: true, Heft: 1})
			h.Apply(gone, true, at)
			assert.Equal(t, state.NewScale(big(), at).Sample, h.Scale().Sample)
		})

		t.Run("and it reached three storeys earlier in the run", func(t *testing.T) {
			h := state.NewHeights()
			sessions := heftSessions(big())
			sessions[20].ID = "later-gone"
			h.Apply(sessions, true, at)
			sessions = heftSessions(big())
			sessions[20] = state.Session{ID: "later-gone", ConversationGone: true}
			h.Apply(sessions, true, at.Add(time.Minute))

			t.Run("it should keep three", func(t *testing.T) {
				assert.Equal(t, 3, storeysOf(sessions, "later-gone"))
			})
		})

		t.Run("and it was never seen with a transcript", func(t *testing.T) {
			h := state.NewHeights()
			sessions := append(heftSessions(big()), state.Session{ID: "gone", ConversationGone: true})
			h.Apply(sessions, true, at)

			t.Run("it should draw at one storey", func(t *testing.T) {
				assert.Equal(t, 1, storeysOf(sessions, "gone"))
			})
		})
	})

	t.Run("when a session's heft is only a lower bound", func(t *testing.T) {
		h := state.NewHeights()
		sessions := append(heftSessions(big()), state.Session{ID: "partial", Heft: 1, HeftPartial: true})
		h.Apply(sessions, true, at)

		t.Run("it should be left out of the sample", func(t *testing.T) {
			assert.Equal(t, 40, h.Scale().Sample)
		})
	})
}

// Item 109, slice 5: past the top line, a spire, one segment per
// doubling.
func TestScaleSpire(t *testing.T) {
	scale := state.Scale{Cuts: [4]int64{100, 200, 300, 1_000}}
	cases := []struct {
		times float64
		want  int
	}{{0.5, 0}, {1, 0}, {1.99, 0}, {2, 1}, {4, 2}, {63, 5}, {64, 6}, {1000, 6}}
	for _, c := range cases {
		t.Run(fmt.Sprintf("when the heft is %g times the top line", c.times), func(t *testing.T) {
			t.Run(fmt.Sprintf("it should have %d segments", c.want), func(t *testing.T) {
				assert.Equal(t, c.want, scale.Spire(int64(c.times*1_000)))
			})
		})
	}
}

func TestHeightsSpire(t *testing.T) {
	at := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)

	t.Run("when the daily recompute raises the top line past a session's doubling", func(t *testing.T) {
		hefts := evenly(40)
		hefts[39] = 1_000_000
		h := state.NewHeights()
		first := heftSessions(hefts)
		h.Apply(first, true, at)
		heavier := evenly(40)
		for i := range heavier {
			heavier[i] *= 100
		}
		heavier[39] = 1_000_000
		sessions := heftSessions(heavier)
		h.Apply(sessions, true, at.Add(25*time.Hour))

		t.Run("it should keep its segments", func(t *testing.T) {
			assert.Equal(t, first[39].Spire, sessions[39].Spire)
		})

		t.Run("it should have had segments to keep", func(t *testing.T) {
			assert.Positive(t, first[39].Spire)
		})
	})
}
