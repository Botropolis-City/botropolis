package group_test

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/group"
)

// everyday are words a heavy user's sessions share without working
// together: each lands in about a sixth of them, under the fifth a word
// must stay under to count as rare (item 270).
var everyday = strings.Fields(`build tests failing parser config output server client cache logger
	handler request response module package release deploy script error branch commit review docs
	refactor layout render styles schema migration`)

// atScale is 1,600 sessions over 30 days in 600 directories, each with a
// few prompts of everyday words and a topic shared with a handful of
// others, and 800 more a minute apart that all name one ticket.
func atScale() []group.Member {
	r := rand.New(rand.NewSource(270))
	end := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	start := end.Add(-29 * 24 * time.Hour)
	var members []group.Member
	for s := range 1600 {
		at := start.Add(time.Duration(s) * 29 * 24 * time.Hour / 1601).Add(time.Duration(r.Intn(600)) * time.Second)
		topic := r.Intn(200)
		var prompts []string
		for range 2 + r.Intn(4) {
			prompts = append(prompts, fmt.Sprintf("please %s %s scale%dtopic%c",
				everyday[r.Intn(len(everyday))], everyday[r.Intn(len(everyday))], topic, 'a'+r.Intn(4)))
		}
		members = append(members, group.Member{ID: fmt.Sprintf("s%04d", s), Dir: fmt.Sprintf("/w/scale%03d", s%600),
			Started: at, Ended: at.Add(time.Duration(5+r.Intn(20)) * time.Minute), Prompts: prompts})
	}
	burst := end.Add(-3 * 24 * time.Hour)
	for s := range 800 {
		at := burst.Add(time.Duration(s) * 50 * time.Second)
		members = append(members, group.Member{ID: fmt.Sprintf("b%04d", s), Dir: fmt.Sprintf("/tmp/tmp.burst%04d", s%300),
			Started: at, Ended: at.Add(30 * time.Second), Prompts: []string{"please run the nightly check for TICKET-270"}})
	}
	return members
}

func TestCandidatesAtScale(t *testing.T) {
	members := atScale()

	t.Run("when 2,400 sessions sharing everyday words are grouped", func(t *testing.T) {
		dt := group.NewDetector()
		got := dt.Detect(members, group.Memory{})
		// The oracle predates naming the keys too common to be evidence
		// (item 271); everything else it finds, it finds as before.
		got.Common = nil
		t.Logf("%d pairs scored, %d weighed", dt.PairsScored(), dt.WordingWeighed())

		t.Run("it should find the groups the every-pair oracle finds", func(t *testing.T) {
			assert.Equal(t, group.DetectEveryPair(members, group.Memory{}), got)
		})

		t.Run("it should score fewer than 100,000 pairs", func(t *testing.T) {
			assert.Less(t, dt.PairsScored(), 100_000)
		})

		t.Run("it should weigh the wording of fewer than 100,000 pairs", func(t *testing.T) {
			assert.Less(t, dt.WordingWeighed(), 100_000)
		})
	})

	t.Run("when the index adds up the rarity two sessions share", func(t *testing.T) {
		sums := group.IndexSums(members)
		r := rand.New(rand.NewSource(2700))
		for range 200 {
			a, b := members[r.Intn(len(members))].ID, members[r.Intn(len(members))].ID
			if a == b {
				continue
			}
			sum, shared := sums(a, b)

			t.Run(fmt.Sprintf("for %s and %s, it should be what the wording weighs them at", a, b), func(t *testing.T) {
				assert.InDelta(t, shared, sum, 1e-9)
			})
		}
	})
}

func TestEverydayWordsAlone(t *testing.T) {
	at := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	// filler are sessions that make the two below rare in what they
	// share and everyday in nothing else.
	filler := func(n int) []group.Member {
		var out []group.Member
		for i := range n {
			out = append(out, group.Member{ID: fmt.Sprintf("f%03d", i), Dir: fmt.Sprintf("/w/f%03d", i),
				Started: at.Add(time.Duration(i+10) * 24 * time.Hour), Prompts: []string{fmt.Sprintf("please look at thing%03d", i)}})
		}
		return out
	}

	t.Run("when two sessions share only everyday words, enough to pass the wording threshold", func(t *testing.T) {
		members := append(filler(60),
			group.Member{ID: "a", Dir: "/w/one", Started: at, Prompts: []string{"migrate the ledger schema and backfill the reconciler"}},
			group.Member{ID: "b", Dir: "/w/two", Started: at.Add(3 * time.Hour), Prompts: []string{"backfill the reconciler after the ledger schema migration"}})
		got := group.NewDetector().Detect(members, group.Memory{})

		t.Run("it should still find what the every-pair oracle finds", func(t *testing.T) {
			assert.Equal(t, group.DetectEveryPair(members, group.Memory{}), got)
		})
	})

	t.Run("when two sessions share too few words to pass the threshold but start within the burst window", func(t *testing.T) {
		members := append(filler(60),
			group.Member{ID: "a", Dir: "/w/one", Branch: "fix-ledger", Started: at, Prompts: []string{"look at the ledger"}},
			group.Member{ID: "b", Dir: "/w/two", Branch: "fix-ledger", Started: at.Add(10 * time.Minute), Prompts: []string{"the ledger again"}})
		got := group.NewDetector().Detect(members, group.Memory{})

		t.Run("it should still score them for their timing and branch, as the every-pair oracle does", func(t *testing.T) {
			assert.Equal(t, group.DetectEveryPair(members, group.Memory{}), got)
		})
	})
}
