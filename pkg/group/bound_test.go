package group_test

import (
	"fmt"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
)

// stagedHistory is a long history at the scale of item 270's laptop,
// all invented: 1,600 sessions in 600 directories, most of them scripted
// runs a minute apart with one boilerplate prompt, and among them ten
// pieces of work of four sessions each, tied by a reference or a team.
func stagedHistory() ([]group.Member, [][]string) {
	var ms []group.Member
	for i := 0; i < 1400; i++ {
		m := member(fmt.Sprintf("run-%04d", i), fmt.Sprintf("/w/scratch%03d", i%560), time.Duration(i)*time.Minute,
			"summarize the changes in this directory and report back")
		m.Ended = m.Started.Add(40 * time.Second)
		ms = append(ms, m)
	}
	var real [][]string
	for k := 0; k < 10; k++ {
		var ids []string
		for n := 0; n < 4; n++ {
			id := fmt.Sprintf("work-%d-%d", k, n)
			at := time.Duration(k)*2*time.Hour + time.Duration(n)*30*time.Second
			m := member(id, fmt.Sprintf("/w/project%02d", k*4+n), at, fmt.Sprintf("carry on with TASK-%d", 100+k))
			if k%2 == 1 {
				m.Prompts = []string{"follow the lead"}
				m.Team = fmt.Sprintf("crew-%d", k)
			}
			ms = append(ms, m)
			ids = append(ids, id)
		}
		real = append(real, ids)
	}
	for i := 0; i < 160; i++ {
		ms = append(ms, member(fmt.Sprintf("idle-%03d", i), fmt.Sprintf("/w/project%02d", i%40),
			time.Duration(i)*7*time.Minute+13*time.Second, "look around"))
	}
	return ms, real
}

func TestDetectBound(t *testing.T) {
	t.Run("when more sessions than the bound are chained by references each pair shares", func(t *testing.T) {
		result := detect(group.Memory{}, sharing(group.MaxMembers+5, func(i int, m *group.Member) {
			m.Prompts = []string{fmt.Sprintf("REF-%d then REF-%d", i, i+1)}
		})...)

		t.Run("it should make no group larger than the bound", func(t *testing.T) {
			assert.Equal(t, group.MaxMembers, largestGroup(result))
		})

		t.Run("it should report the session that would have joined past the bound", func(t *testing.T) {
			var capped []string
			for _, c := range result.Capped {
				capped = append(capped, c.Session)
			}
			assert.Equal(t, []string{fmt.Sprintf("s%04d", group.MaxMembers)}, capped)
		})

		t.Run("it should leave that session out of every group", func(t *testing.T) {
			_, ok := result.GroupOf(fmt.Sprintf("s%04d", group.MaxMembers))
			assert.False(t, ok)
		})
	})

	t.Run("when a staged history like item 270's is grouped", func(t *testing.T) {
		ms, real := stagedHistory()
		result := detect(group.Memory{}, ms...)

		t.Run("it should make no group larger than the bound", func(t *testing.T) {
			assert.LessOrEqual(t, largestGroup(result), group.MaxMembers)
		})

		t.Run("it should find each piece of work as a group of its own", func(t *testing.T) {
			var found [][]string
			for _, ids := range real {
				g, _ := result.GroupOf(ids[0])
				var in []string
				for _, m := range g.Members {
					in = append(in, m.Session)
				}
				found = append(found, in)
			}
			assert.Equal(t, real, found)
		})
	})
}
