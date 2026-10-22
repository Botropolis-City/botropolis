package group_test

import (
	"fmt"
	"math/rand"
	"runtime"
	"strings"
	"testing"
	"time"

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

func keptByAFreshDetector(members []group.Member, memory group.Memory) int {
	d := group.NewDetector()
	d.Detect(members, memory)
	return d.PairsKept()
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

			t.Run("it should find the same result as scoring every pair", func(t *testing.T) {
				assert.Equal(t, group.DetectEveryPair(next, group.Memory{}), got)
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
				scenario: "and the older half of the sessions age out",
				members:  append([]group.Member(nil), base[len(base)/2:]...),
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

				t.Run("it should find the same result as scoring every pair", func(t *testing.T) {
					assert.Equal(t, group.DetectEveryPair(c.members, c.memory), got)
				})

				t.Run("it should keep the pairs a fresh detector keeps", func(t *testing.T) {
					assert.Equal(t, keptByAFreshDetector(c.members, c.memory), d.PairsKept())
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
			want := group.DetectEveryPair(members, memory)
			if !assert.ObjectsAreEqual(want, got) {
				firstDiffering = step
			}
			memory = memory.Remember(want)
		}

		t.Run("it should find the same result as scoring every pair at every step", func(t *testing.T) {
			assert.Equal(t, -1, firstDiffering)
		})
	})

	t.Run("when two sessions far apart in time are joined by one signal", func(t *testing.T) {
		long := strings.Repeat("the plan is to move the parser into its own module and test each rule ", 5)
		x := group.Member{ID: "x", Dir: "/w/x", Started: t0.Add(10 * 24 * time.Hour), Prompts: []string{"please look at this"}}
		y := group.Member{ID: "y", Dir: "/w/y", Started: t0.Add(12 * 24 * time.Hour), Prompts: []string{"please look at that"}}
		signals := []struct {
			signal string
			join   func(x, y *group.Member)
		}{
			{"a visit to the other's directory while it ran", func(x, y *group.Member) {
				x.Visited = []group.Visit{{Dir: "/w/y", From: y.Started, To: y.Started.Add(time.Hour)}}
			}},
			{"the other's visit to its directory while it ran", func(x, y *group.Member) {
				x.Ended = y.Started.Add(2 * time.Hour)
				y.Visited = []group.Visit{{Dir: "/w/x", From: y.Started, To: y.Started.Add(time.Hour)}}
			}},
			{"a visit to a directory above the other's", func(x, y *group.Member) {
				y.Dir = "/w/y/deep/er"
				x.Visited = []group.Visit{{Dir: "/w/y", From: y.Started, To: y.Started.Add(time.Hour)}}
			}},
			{"a visit to a directory below the other's", func(x, y *group.Member) {
				x.Visited = []group.Visit{{Dir: "/w/y/deep", From: y.Started, To: y.Started.Add(time.Hour)}}
			}},
			{"a reference", func(x, y *group.Member) {
				x.Prompts = append(x.Prompts, "see PROJ-4242")
				y.Prompts = append(y.Prompts, "PROJ-4242 again")
			}},
			{"a team", func(x, y *group.Member) { x.Team, y.Team = "crew", "crew" }},
			{"a parent", func(x, y *group.Member) { y.Parent = "x" }},
			{"a message", func(x, y *group.Member) { y.Messaged = []string{"x"} }},
			{"a near-duplicate paste", func(x, y *group.Member) {
				x.Pasted = []string{long}
				y.Pasted = []string{strings.Replace(long, "parser", "lexer", 1)}
			}},
			{"starts within a minute", func(x, y *group.Member) { y.Started = x.Started.Add(40 * time.Second) }},
			{"a start within a minute of the other's close", func(x, y *group.Member) {
				x.Ended = y.Started.Add(-30 * time.Second)
			}},
			{"wording", func(x, y *group.Member) {
				x.Prompts = append(x.Prompts, "zebrafoo quokkabar")
				y.Prompts = append(y.Prompts, "quokkabar zebrafoo")
			}},
			{"a branch", func(x, y *group.Member) { x.Branch, y.Branch = "feat/shared", "feat/shared" }},
			{"branches naming one ticket", func(x, y *group.Member) { x.Branch, y.Branch = "proj-77-a", "fix/PROJ-77" }},
			{"a file", func(x, y *group.Member) { x.Files, y.Files = []string{"/w/x/a.go"}, []string{"/w/x/a.go"} }},
			{"starts within a burst", func(x, y *group.Member) { y.Started = x.Started.Add(9 * time.Minute) }},
			{"a start soon after the other's close", func(x, y *group.Member) {
				x.Ended = y.Started.Add(-7 * time.Minute)
			}},
		}
		for _, s := range signals {
			t.Run("and the signal is "+s.signal, func(t *testing.T) {
				a, b := x, y
				s.join(&a, &b)
				members := append(fillers(), a, b)
				want := group.EdgesEveryPair(members, group.Memory{})

				t.Run("scoring every pair should find an edge", func(t *testing.T) {
					assert.NotEmpty(t, want)
				})

				t.Run("it should find the edges scoring every pair finds", func(t *testing.T) {
					assert.Equal(t, want, group.DetectEdges(group.NewDetector(), members, group.Memory{}))
				})
			})
		}
	})

	t.Run("when one detector is given histories mixing every signal, one seed after another", func(t *testing.T) {
		d := group.NewDetector()
		for seed := int64(1); seed <= 40; seed++ {
			members, memory := mixedHistory(seed)
			got := group.DetectEdges(d, members, memory)

			t.Run(fmt.Sprintf("and the seed is %d", seed), func(t *testing.T) {
				t.Run("it should find the edges scoring every pair finds", func(t *testing.T) {
					assert.Equal(t, group.EdgesEveryPair(members, memory), got)
				})
			})
		}
	})

	t.Run("when a history mixing every signal changes one session at a time", func(t *testing.T) {
		members, memory := mixedHistory(198)
		d := group.NewDetector()
		d.Detect(members, memory)
		r := rand.New(rand.NewSource(198))
		firstDiffering := -1
		for step := 0; step < 60 && firstDiffering < 0; step++ {
			others, _ := mixedHistory(int64(1000 + step))
			i := r.Intn(len(members))
			members = append([]group.Member(nil), members...)
			for _, o := range others {
				if o.ID == members[i].ID {
					members[i] = o
				}
			}
			if !assert.ObjectsAreEqual(group.EdgesEveryPair(members, memory), group.DetectEdges(d, members, memory)) {
				firstDiffering = step
			}
		}

		t.Run("it should find the edges scoring every pair finds at every step", func(t *testing.T) {
			assert.Equal(t, -1, firstDiffering)
		})
	})
}

// mixedHistory is forty sessions drawn from small pools of directories,
// words, branches, teams, files and pastes, started close enough
// together that every signal turns up, with memory marking pairs not
// related and moving a session.
func mixedHistory(seed int64) ([]group.Member, group.Memory) {
	r := rand.New(rand.NewSource(seed))
	pick := func(xs ...string) string { return xs[r.Intn(len(xs))] }
	dirs := []string{"/w/a", "/w/a/sub", "/w/b", "/w/b/deep/er", "/w/c", "/x"}
	words := strings.Fields("alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima mike PROJ-7 PROJ-9")
	paste := strings.Repeat("first move the store behind an interface then test the loader with a fake store ", 4)
	ids := make([]string, 40)
	for i := range ids {
		ids[i] = fmt.Sprintf("m%02d", i)
	}
	at := t0
	var ms []group.Member
	for i, id := range ids {
		at = at.Add([]time.Duration{20 * time.Second, 5 * time.Minute, 12 * time.Minute, 3 * time.Hour}[r.Intn(4)])
		m := group.Member{ID: id, Dir: pick(dirs...), Started: at,
			Branch: pick("", "main", "feat/PROJ-1-x", "PROJ-1-fix", "feat/y", "feat/y", "z")}
		if r.Intn(2) == 0 {
			m.Ended = at.Add(time.Duration(r.Intn(40)) * time.Minute)
		}
		for p := 1 + r.Intn(3); p > 0; p-- {
			m.Prompts = append(m.Prompts, "please "+pick(words...)+" "+pick(words...)+" "+pick(words...))
		}
		switch r.Intn(6) {
		case 0:
			m.Pasted = []string{paste}
		case 1:
			m.Pasted = []string{strings.Replace(paste, "store", "cache", 1)}
		}
		m.Team = pick("", "", "", "t1", "t2")
		if i > 0 && r.Intn(6) == 0 {
			m.Parent = ids[r.Intn(i)]
		}
		if r.Intn(6) == 0 {
			m.Messaged = []string{ids[r.Intn(len(ids))]}
		}
		if r.Intn(4) == 0 {
			m.Visited = []group.Visit{{Dir: pick(dirs...), From: at, To: at.Add(time.Duration(r.Intn(90)) * time.Minute)}}
		}
		if r.Intn(4) == 0 {
			m.Files = []string{pick("/w/a/x.go", "/w/b/y.go", "/w/c/z.go")}
		}
		ms = append(ms, m)
	}
	memory := group.Memory{
		NotRelated: []group.Pair{{ids[r.Intn(40)], ids[r.Intn(40)]}, {ids[r.Intn(40)], ids[r.Intn(40)]}},
		Moves:      []group.Move{{Session: ids[r.Intn(40)], Group: ""}},
	}
	r.Shuffle(len(ms), func(i, j int) { ms[i], ms[j] = ms[j], ms[i] })
	return ms, memory
}

// tiesPerSession is the most sessions any one session is plausibly tied
// to: a topic worked across a few dozen sessions in a month, and those
// started in the same quarter hour. The pair cache keeps each tie once,
// so it holds at most half as many pairs per session.
const tiesPerSession = 40

func TestPairCache(t *testing.T) {
	t.Run("when 2,000 small sessions over 30 days are grouped", func(t *testing.T) {
		members := membersOf(helpers.ManySessions(198, 2000, t0))
		d := group.NewDetector()
		d.Detect(members, group.Memory{})

		t.Run("it should keep at most half as many pairs per session as a session has ties", func(t *testing.T) {
			assert.LessOrEqual(t, d.PairsKept(), len(members)*tiesPerSession/2)
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

// BenchmarkDetectAfterAPromptAmongMany is a prompt in one of a heavy
// user's 2,000 sessions: what a settled daemon pays per prompt (item 198).
func BenchmarkDetectAfterAPromptAmongMany(b *testing.B) {
	base := membersOf(helpers.ManySessions(198, 2000, t0))
	next := withPrompt(base, 1000, "please zzqfirstword")
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

// BenchmarkPairsKept reports what the detector keeps between runs
// against the number of sessions: pairs per session and live heap per
// session should stay about flat as the history grows (item 198).
func BenchmarkPairsKept(b *testing.B) {
	for _, n := range []int{250, 500, 1000, 2000} {
		b.Run(fmt.Sprintf("sessions=%d", n), func(b *testing.B) {
			members := membersOf(helpers.ManySessions(198, n, t0))
			var kept int
			var live uint64
			for i := 0; i < b.N; i++ {
				var before, after runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&before)
				d := group.NewDetector()
				d.Detect(members, group.Memory{})
				runtime.GC()
				runtime.ReadMemStats(&after)
				kept, live = d.PairsKept(), after.HeapAlloc-min(before.HeapAlloc, after.HeapAlloc)
				runtime.KeepAlive(d)
			}
			b.ReportMetric(float64(kept)/float64(n), "pairs/session")
			b.ReportMetric(float64(live)/float64(n), "live-B/session")
		})
	}
}
