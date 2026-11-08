package group_test

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
)

// generatedPlacements is a city of sessions in teams, with moves, starts
// and stored groups pointing at one another's groups, from seed: the ids
// a group can take are those detection and teams make, and a few stored.
func generatedPlacements(seed uint64) (group.Memory, []group.Member) {
	r := rand.New(rand.NewPCG(seed, 364))
	n := 4 + r.IntN(7)
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("s%d", i)
	}
	pool := []string{"g-s0", "g-s1", "g-s2"}
	for _, id := range ids {
		pool = append(pool, group.NewGroupID(id))
	}
	pick := func() string { return pool[r.IntN(len(pool))] }
	refs := []string{"PROJ-1", "PROJ-2", "LOG-3"}
	var leads []string
	ms := make([]group.Member, 0, n)
	for i, id := range ids {
		prompt := "alone on " + id
		if r.IntN(2) == 0 {
			prompt = refs[r.IntN(len(refs))] + " work"
		}
		m := member(id, "/w/"+id, time.Duration(i)*time.Hour, prompt)
		switch {
		case len(leads) > 0 && r.IntN(3) == 0:
			m.Lead = leads[r.IntN(len(leads))]
			m.Team = "team-" + m.Lead
		case r.IntN(3) == 0:
			leads = append(leads, id)
			m.Team = "team-" + id
		}
		if r.IntN(4) == 0 {
			m.StartedFrom = pick()
		}
		if r.IntN(5) == 0 {
			m.Ended = m.Started.Add(30 * time.Minute)
		}
		ms = append(ms, m)
	}
	var memory group.Memory
	for _, id := range pool {
		if r.IntN(3) != 0 {
			continue
		}
		s := group.Stored{ID: id, Name: "stored " + id}
		for _, m := range ids {
			if r.IntN(3) == 0 {
				s.Members = append(s.Members, m)
			}
		}
		memory.Groups = append(memory.Groups, s)
	}
	for range r.IntN(5) {
		to := pick()
		if r.IntN(5) == 0 {
			to = ""
		}
		memory.Moves = append(memory.Moves, group.Move{Session: ids[r.IntN(n)], Group: to})
	}
	if len(leads) > 0 && r.IntN(4) == 0 {
		gone := leads[r.IntN(len(leads))]
		ms = slices.DeleteFunc(ms, func(m group.Member) bool { return m.ID == gone })
	}
	return memory, ms
}

// Item 364: however moves, starts, teams and stored groups point at one
// another, no two groups share an id.
func TestUniqueGroupIDs(t *testing.T) {
	t.Run("when placements are generated from seeds 1 to 1000, with teams, moves, starts and stored groups pointing at each other's groups", func(t *testing.T) {
		for seed := uint64(1); seed <= 1000; seed++ {
			memory, ms := generatedPlacements(seed)
			result := detect(memory, ms...)
			var dup []string
			seen := map[string]bool{}
			for _, g := range result.Groups {
				if seen[g.ID] {
					dup = append(dup, g.ID)
				}
				seen[g.ID] = true
			}
			t.Run(fmt.Sprintf("seed %d, it should return no two groups with one id", seed), func(t *testing.T) {
				assert.Empty(t, dup)
			})
		}
	})
}
