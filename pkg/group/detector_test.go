package group_test

import (
	"testing"

	"botropolis.city/botropolis/pkg/group"
	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
)

func membersOf(history []helpers.HistoryPrompt) []group.Member {
	index := map[string]int{}
	var ms []group.Member
	for _, p := range history {
		i, ok := index[p.Session]
		if !ok {
			i = len(ms)
			index[p.Session] = i
			ms = append(ms, group.Member{ID: p.Session, Dir: p.Dir, Started: p.At})
		}
		ms[i].Prompts = append(ms[i].Prompts, p.Text)
	}
	return ms
}

func withPrompt(ms []group.Member, i int, text string) []group.Member {
	out := append([]group.Member(nil), ms...)
	out[i].Prompts = append(append([]string(nil), out[i].Prompts...), text)
	return out
}

func TestDetector(t *testing.T) {
	t.Run("when a detector has grouped a synthetic history", func(t *testing.T) {
		base := membersOf(helpers.SyntheticHistory(158, 60, 12, t0))
		grouped := func(memory group.Memory) *group.Detector {
			d := group.NewDetector()
			d.Detect(base, memory)
			return d
		}

		t.Run("and one session sends a prompt that changes none of its edges", func(t *testing.T) {
			next := withPrompt(base, 7, "please zzqfirstword")
			d := grouped(group.Memory{})
			before := d.PairsScored()
			got := d.Detect(next, group.Memory{})
			scored := d.PairsScored() - before

			t.Run("it should score only that session's pairs", func(t *testing.T) {
				assert.LessOrEqual(t, scored, len(base)-1)
			})

			t.Run("it should find the same result as detecting from scratch", func(t *testing.T) {
				assert.Equal(t, group.Detect(next, group.Memory{}), got)
			})
		})

		changes := []struct {
			scenario string
			members  []group.Member
			memory   group.Memory
		}{
			{
				scenario: "and one session sends a prompt whose word changes the rarity of a word others use",
				members:  withPrompt(base, 7, "please topicaworda topicbwordb topiccwordc"),
			},
			{
				scenario: "and a session joins",
				members: append(append([]group.Member(nil), base...), group.Member{ID: "joined", Dir: "/w/repo03",
					Started: base[len(base)-1].Started, Prompts: []string{"please topicdworda"}}),
			},
			{
				scenario: "and a session leaves",
				members:  append(append([]group.Member(nil), base[:20]...), base[21:]...),
			},
			{
				scenario: "and memory mutes a word",
				members:  base,
				memory:   group.Memory{Muted: []string{"topicaworda", "topicbworda"}},
			},
			{
				scenario: "and memory marks a pair not related",
				members:  base,
				memory:   group.Memory{NotRelated: []group.Pair{{base[3].ID, base[11].ID}, {base[5].ID, base[6].ID}}},
			},
			{
				scenario: "and memory moves a session",
				members:  base,
				memory:   group.Memory{Moves: []group.Move{{Session: base[9].ID, Group: ""}}},
			},
			{
				scenario: "and memory confirms a session",
				members:  base,
				memory:   group.Memory{Confirmed: []string{base[12].ID, base[30].ID}},
			},
		}
		for _, c := range changes {
			t.Run(c.scenario, func(t *testing.T) {
				d := grouped(group.Memory{})
				got := d.Detect(c.members, c.memory)

				t.Run("it should find the same result as detecting from scratch", func(t *testing.T) {
					assert.Equal(t, group.Detect(c.members, c.memory), got)
				})
			})
		}
	})

	t.Run("when a history is replayed prompt by prompt", func(t *testing.T) {
		history := helpers.SyntheticHistory(1580, 24, 8, t0)
		d := group.NewDetector()
		memory := group.Memory{}
		firstDiffering := -1
		for step := 1; step <= len(history) && firstDiffering < 0; step++ {
			members := membersOf(history[:step])
			got := d.Detect(members, memory)
			want := group.Detect(members, memory)
			if !assert.ObjectsAreEqual(want, got) {
				firstDiffering = step
			}
			memory = memory.Remember(want)
		}

		t.Run("it should find the same result as from scratch at every step", func(t *testing.T) {
			assert.Equal(t, -1, firstDiffering)
		})
	})
}

func BenchmarkDetectAfterAPrompt(b *testing.B) {
	base := membersOf(helpers.SyntheticHistory(158, 100, 40, t0))
	next := withPrompt(base, 50, "please zzqfirstword")
	d := group.NewDetector()
	d.Detect(base, group.Memory{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		members := base
		if i%2 == 0 {
			members = next
		}
		d.Detect(members, group.Memory{})
	}
}
