package group_test

import (
	"fmt"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
)

// sharing is n sessions three hours apart in directories of their own,
// each made to share something by with.
func sharing(n int, with func(i int, m *group.Member)) []group.Member {
	var ms []group.Member
	for i := 0; i < n; i++ {
		m := member(fmt.Sprintf("s%04d", i), fmt.Sprintf("/w/dir%04d", i), time.Duration(i)*3*time.Hour, "go")
		with(i, &m)
		ms = append(ms, m)
	}
	return ms
}

func TestDetectCommonKeys(t *testing.T) {
	t.Run("when more sessions than the bound share one key, and nothing else ties them", func(t *testing.T) {
		cases := []struct {
			scenario string
			with     func(i int, m *group.Member)
			common   []string
		}{
			{"and the key is a reference", func(_ int, m *group.Member) { m.Prompts = []string{"on PROJ-42"} },
				[]string{"reference PROJ-42"}},
			{"and the key is a team", func(_ int, m *group.Member) { m.Team = "crew" },
				[]string{"team crew"}},
			{"and the keys are a branch and a file", func(_ int, m *group.Member) {
				m.Branch, m.Files = "feat/kiln", []string{"/w/lib/kiln.go"}
			}, []string{"branch feat/kiln", "file /w/lib/kiln.go"}},
		}
		for _, c := range cases {
			t.Run(c.scenario, func(t *testing.T) {
				result := detect(group.Memory{}, sharing(group.MaxMembers+1, c.with)...)

				t.Run("it should make no group", func(t *testing.T) {
					assert.Empty(t, result.Groups)
				})

				t.Run("it should report the key as common", func(t *testing.T) {
					assert.Equal(t, c.common, result.Common)
				})
			})
		}
	})

	t.Run("when exactly the bound share one reference", func(t *testing.T) {
		result := detect(group.Memory{}, sharing(group.MaxMembers, func(_ int, m *group.Member) { m.Prompts = []string{"on PROJ-42"} })...)

		t.Run("it should make one group of the bound", func(t *testing.T) {
			assert.Equal(t, []int{group.MaxMembers}, groupSizes(result))
		})
	})

	t.Run("when more sessions than the bound share one reference, and two of them also share a reference of their own", func(t *testing.T) {
		result := detect(group.Memory{}, sharing(group.MaxMembers+1, func(i int, m *group.Member) {
			m.Prompts = []string{"on PROJ-42"}
			if i < 2 {
				m.Prompts = append(m.Prompts, "and LOG-7")
			}
		})...)

		t.Run("it should group those two", func(t *testing.T) {
			g, _ := result.GroupOf("s0000")
			joined, ok := result.GroupOf("s0001")
			assert.True(t, ok && g.ID == joined.ID && len(g.Members) == 2)
		})
	})

	t.Run("when one detector sees a reference shared by the bound, and then by one session more", func(t *testing.T) {
		on := func(_ int, m *group.Member) { m.Prompts = []string{"on PROJ-42"} }
		d := group.NewDetector()
		d.Detect(append(fillers(), sharing(group.MaxMembers, on)...), group.Memory{})
		more := append(fillers(), sharing(group.MaxMembers+1, on)...)

		t.Run("it should find what a fresh detector finds", func(t *testing.T) {
			assert.Equal(t, group.Detect(more, group.Memory{}), d.Detect(more, group.Memory{}))
		})
	})

	t.Run("when one detector sees a reference shared by the bound, and then one more of the same sessions mentions it", func(t *testing.T) {
		upTo := func(n int) func(i int, m *group.Member) {
			return func(i int, m *group.Member) {
				if i < n {
					m.Prompts = []string{"on PROJ-42"}
				}
			}
		}
		d := group.NewDetector()
		d.Detect(append(fillers(), sharing(group.MaxMembers+1, upTo(group.MaxMembers))...), group.Memory{})
		more := append(fillers(), sharing(group.MaxMembers+1, upTo(group.MaxMembers+1))...)

		t.Run("it should find what a fresh detector finds", func(t *testing.T) {
			assert.Equal(t, group.Detect(more, group.Memory{}), d.Detect(more, group.Memory{}))
		})
	})
}

func groupSizes(result group.Result) []int {
	var sizes []int
	for _, g := range result.Groups {
		sizes = append(sizes, len(g.Members))
	}
	return sizes
}

func TestDetectorScoringCost(t *testing.T) {
	t.Run("when 800 of 2,400 sessions share one reference and nothing else, hours apart", func(t *testing.T) {
		ms := sharing(2400, func(i int, m *group.Member) {
			if i%3 == 0 {
				m.Prompts = []string{"carry on with PROJ-800"}
			}
		})
		d := group.NewDetector()
		d.Detect(ms, group.Memory{})

		t.Run("it should score fewer than 2,400 pairs", func(t *testing.T) {
			assert.Less(t, d.PairsScored(), 2400)
		})

		t.Run("it should keep fewer than 2,400 ties", func(t *testing.T) {
			assert.Less(t, d.PairsKept(), 2400)
		})
	})
}
