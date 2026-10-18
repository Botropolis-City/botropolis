package harness_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/harness"
	"botropolis.city/botropolis/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fake struct {
	name     string
	snapshot state.Snapshot
	err      error
	dirs     []string
}

func (f fake) Name() string                           { return f.name }
func (f fake) Load(time.Time) (state.Snapshot, error) { return f.snapshot, f.err }
func (f fake) WatchDirs() []string                    { return f.dirs }

var now = time.Date(2026, time.September, 17, 8, 0, 0, 0, time.UTC)

func TestMulti(t *testing.T) {
	a := fake{name: "claude", dirs: []string{"/a"}, snapshot: state.Snapshot{
		Sessions: []state.Session{{ID: "1", State: state.Working}},
		Servers:  []state.Server{{Name: "atlassian"}},
		Power:    state.Power{CostUSD: 1.5, CostKnown: true, Fresh: 10, Cached: 100, ByModel: map[string]claude.Usage{"opus": {Output: 5}}},
		Stats:    &claude.Stats{TotalSessions: 513},
		Teams:    []claude.Team{{Name: "review", LeadSessionID: "1"}},
		Groups:   []state.Group{{ID: "g1", Members: []string{"1"}}},
	}}
	b := fake{name: "codex", dirs: []string{"/b"}, snapshot: state.Snapshot{
		Sessions: []state.Session{{ID: "2", State: state.Resting, Harness: "codex"}},
		Power:    state.Power{CostUSD: 0.5, Fresh: 1, ByModel: map[string]claude.Usage{"opus": {Output: 1}, "gpt": {Output: 7}}},
	}}

	t.Run("when two harnesses are merged", func(t *testing.T) {
		snapshot, err := harness.NewMulti(a, b).Load(now)
		require.NoError(t, err)

		t.Run("it should concatenate sessions and tag them with their harness", func(t *testing.T) {
			require.Len(t, snapshot.Sessions, 2)
			assert.Equal(t, "claude", snapshot.Sessions[0].Harness)
			assert.Equal(t, "codex", snapshot.Sessions[1].Harness)
		})

		t.Run("it should add the power up across harnesses", func(t *testing.T) {
			assert.InDelta(t, 2.0, snapshot.Power.CostUSD, 1e-9)
			assert.Equal(t, int64(6), snapshot.Power.ByModel["opus"].Output)
			assert.Equal(t, int64(7), snapshot.Power.ByModel["gpt"].Output)
		})

		t.Run("it should know the cost when either harness does", func(t *testing.T) {
			assert.True(t, snapshot.Power.CostKnown)
		})

		t.Run("it should keep the servers", func(t *testing.T) {
			assert.Len(t, snapshot.Servers, 1)
		})

		t.Run("it should keep the teams", func(t *testing.T) {
			require.Len(t, snapshot.Teams, 1)
			assert.Equal(t, "review", snapshot.Teams[0].Name)
		})

		t.Run("it should keep the groups", func(t *testing.T) {
			assert.Equal(t, []state.Group{{ID: "g1", Members: []string{"1"}}}, snapshot.Groups)
		})

		t.Run("it should keep the stats rollup", func(t *testing.T) {
			require.NotNil(t, snapshot.Stats)
			assert.Equal(t, 513, snapshot.Stats.TotalSessions)
		})

		t.Run("it should stamp the snapshot with now", func(t *testing.T) {
			assert.Equal(t, now, snapshot.At)
		})

		t.Run("it should union the watch directories", func(t *testing.T) {
			assert.Equal(t, []string{"/a", "/b"}, harness.NewMulti(a, b).WatchDirs())
		})
	})

	t.Run("when neither harness knows the cost", func(t *testing.T) {
		snapshot, err := harness.NewMulti(b, b).Load(now)
		require.NoError(t, err)

		t.Run("it should not know it either", func(t *testing.T) {
			assert.False(t, snapshot.Power.CostKnown)
		})
	})

	t.Run("when a harness fails", func(t *testing.T) {
		_, err := harness.NewMulti(a, fake{name: "broken", err: errors.New("boom")}).Load(now)

		t.Run("it should return the error", func(t *testing.T) {
			assert.ErrorContains(t, err, "boom")
		})
	})
}

// filled builds a value of v's type with every field set to something
// other than its zero value, so a merge that drops a field shows.
func filled(v reflect.Value) {
	switch v.Kind() {
	case reflect.String:
		v.SetString("x")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1.5)
	case reflect.Slice:
		elem := reflect.New(v.Type().Elem()).Elem()
		filled(elem)
		v.Set(reflect.Append(reflect.MakeSlice(v.Type(), 0, 1), elem))
	case reflect.Map:
		key := reflect.New(v.Type().Key()).Elem()
		filled(key)
		elem := reflect.New(v.Type().Elem()).Elem()
		filled(elem)
		m := reflect.MakeMap(v.Type())
		m.SetMapIndex(key, elem)
		v.Set(m)
	case reflect.Pointer:
		p := reflect.New(v.Type().Elem())
		filled(p.Elem())
		v.Set(p)
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(time.Time{}) {
			v.Set(reflect.ValueOf(now))
			return
		}
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				filled(v.Field(i))
			}
		}
	}
}

func TestMultiKeepsEveryField(t *testing.T) {
	t.Run("when one harness's snapshot goes through the merge", func(t *testing.T) {
		var in state.Snapshot
		filled(reflect.ValueOf(&in).Elem())
		out, err := harness.NewMulti(fake{name: "claude", snapshot: in}).Load(now)
		require.NoError(t, err)
		// At is the merge's own moment and Power.Since its own window; every
		// other exported field must come through untouched.
		in.At, out.At = time.Time{}, time.Time{}
		in.Power.Since, out.Power.Since = time.Time{}, time.Time{}
		// Storeys and spires are the merge's own too: it draws every
		// harness's sessions on one scale (item 109).
		for _, snap := range []*state.Snapshot{&in, &out} {
			for i := range snap.Sessions {
				snap.Sessions[i].Storeys, snap.Sessions[i].Spire = 0, 0
			}
		}

		for i := 0; i < reflect.TypeOf(in).NumField(); i++ {
			field := reflect.TypeOf(in).Field(i)
			t.Run("it should keep "+field.Name, func(t *testing.T) {
				assert.Equal(t, reflect.ValueOf(in).Field(i).Interface(), reflect.ValueOf(out).Field(i).Interface())
			})
		}
	})
}

// Item 109: every harness's sessions are drawn on one scale.
func TestMultiHeights(t *testing.T) {
	hefts := func(base int64, n int, harness string) []state.Session {
		var sessions []state.Session
		for i := range n {
			sessions = append(sessions, state.Session{ID: harness + string(rune('a'+i)), Harness: harness, Heft: base * int64(i+1)})
		}
		return sessions
	}
	claudeSessions, codexSessions := hefts(1_000, 20, "claude"), hefts(1_000_000, 20, "codex")
	var all []int64
	for _, s := range append(append([]state.Session{}, claudeSessions...), codexSessions...) {
		all = append(all, s.Heft)
	}

	t.Run("when a Codex session and a Claude session share the snapshot", func(t *testing.T) {
		a := fake{name: "claude", snapshot: state.Snapshot{Sessions: claudeSessions, HeftsRead: true}}
		b := fake{name: "codex", snapshot: state.Snapshot{Sessions: codexSessions, HeftsRead: true}}
		snapshot, err := harness.NewMulti(a, b).Load(now)
		require.NoError(t, err)

		t.Run("it should put both on one scale", func(t *testing.T) {
			scale := state.NewScale(all, now)
			var want, got []int
			for _, s := range snapshot.Sessions {
				want, got = append(want, scale.Storeys(s.Heft)), append(got, s.Storeys)
			}
			assert.Equal(t, want, got)
		})
	})

	t.Run("when one harness has not yet read its hefts whole", func(t *testing.T) {
		a := fake{name: "claude", snapshot: state.Snapshot{Sessions: claudeSessions}}
		b := fake{name: "codex", snapshot: state.Snapshot{Sessions: codexSessions, HeftsRead: true}}
		snapshot, err := harness.NewMulti(a, b).Load(now)
		require.NoError(t, err)

		t.Run("it should not report the hefts read", func(t *testing.T) {
			assert.False(t, snapshot.HeftsRead)
		})

		t.Run("it should draw on the starter lines", func(t *testing.T) {
			assert.Equal(t, state.Scale{Cuts: state.StarterCuts}.Storeys(codexSessions[0].Heft), snapshot.Sessions[20].Storeys)
		})
	})

	// Item 110.
	t.Run("when one harness gives stored hefts and another does not", func(t *testing.T) {
		var stored []int64
		for i := range 30 {
			stored = append(stored, int64(i+1)*50_000)
		}
		a := fake{name: "claude", snapshot: state.Snapshot{Sessions: claudeSessions, ScaleHefts: stored, HeftsRead: true}}
		b := fake{name: "codex", snapshot: state.Snapshot{Sessions: codexSessions, HeftsRead: true}}
		snapshot, err := harness.NewMulti(a, b).Load(now)
		require.NoError(t, err)

		t.Run("it should cut from the stored hefts plus the other's sessions", func(t *testing.T) {
			sample := append([]int64{}, stored...)
			for _, s := range codexSessions {
				sample = append(sample, s.Heft)
			}
			scale := state.NewScale(sample, now)
			var want, got []int
			for _, s := range snapshot.Sessions {
				want, got = append(want, scale.Storeys(s.Heft)), append(got, s.Storeys)
			}
			assert.Equal(t, want, got)
		})
	})
}
