package daemon_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/daemon"
	"botropolis.city/botropolis/pkg/find"
	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
)

// deepCopy is v with every slice, map and pointer it reaches copied, so
// a later write to v's does not show in it.
func deepCopy[T any](v T) T {
	var out T
	copyValue(reflect.ValueOf(&out).Elem(), reflect.ValueOf(v))
	return out
}

func copyValue(dst, src reflect.Value) {
	switch src.Kind() {
	case reflect.Slice:
		if src.IsNil() {
			return
		}
		dst.Set(reflect.MakeSlice(src.Type(), src.Len(), src.Len()))
		for i := 0; i < src.Len(); i++ {
			copyValue(dst.Index(i), src.Index(i))
		}
	case reflect.Map:
		if src.IsNil() {
			return
		}
		dst.Set(reflect.MakeMapWithSize(src.Type(), src.Len()))
		for it := src.MapRange(); it.Next(); {
			v := reflect.New(it.Value().Type()).Elem()
			copyValue(v, it.Value())
			dst.SetMapIndex(it.Key(), v)
		}
	case reflect.Pointer:
		if src.IsNil() {
			return
		}
		v := reflect.New(src.Elem().Type())
		copyValue(v.Elem(), src.Elem())
		dst.Set(v)
	case reflect.Struct:
		dst.Set(src)
		for i := 0; i < src.NumField(); i++ {
			if dst.Field(i).CanSet() {
				copyValue(dst.Field(i), src.Field(i))
			}
		}
	default:
		dst.Set(src)
	}
}

// keptLoads is a loader that keeps what it last handed the daemon, and
// a deep copy of it taken before anything else could see it.
type keptLoads struct {
	*state.Loader
	loaded, before []state.Session
}

func (k *keptLoads) Load(at time.Time) (state.Snapshot, error) {
	snapshot, err := k.Loader.Load(at)
	k.loaded, k.before = snapshot.Sessions, deepCopy(snapshot.Sessions)
	return snapshot, err
}

func sessionIn(t *testing.T, snapshot state.Snapshot, id string) state.Session {
	t.Helper()
	for _, s := range snapshot.Sessions {
		if s.ID == id {
			return s
		}
	}
	require.FailNow(t, "no such session", id)
	return state.Session{}
}

// Item 213: every rescan and every hook event copied the whole session
// slice to lay the hooks over it, 2 MB a rescan on a 2,000-session home
// and a quarter of what the daemon allocated. The view shares the loaded
// sessions until a hook has something to say about one of them.
func TestViewSharesTheLoadedSessions(t *testing.T) {
	t.Run("when a rescan has loaded sessions no hook has touched", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		require.NoError(t, d.Rescan())

		t.Run("it should make a view without allocating", func(t *testing.T) {
			assert.Zero(t, testing.AllocsPerRun(20, func() { d.Snapshot() }))
		})
	})

	t.Run("when a hook event changes a session after a view was handed out", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		require.NoError(t, d.Rescan())
		handed := d.Snapshot()
		was := deepCopy(handed.Sessions)
		d.Apply(claude.HookEvent{Name: claude.HookPreToolUse, SessionID: sid, ToolName: "Bash"}, now.Add(time.Hour))
		next := d.Snapshot()

		t.Run("it should leave the view handed out as it was", func(t *testing.T) {
			assert.Equal(t, was, handed.Sessions)
		})

		t.Run("it should show the hook's state in the next view", func(t *testing.T) {
			assert.Equal(t, state.Working, sessionIn(t, next, sid).State)
		})
	})

	t.Run("when a session had a hook event, and a later rescan's transcript has caught up with it", func(t *testing.T) {
		d := newDaemon(t, idleHome(t, "interactive"))
		require.NoError(t, d.Rescan())
		d.Apply(claude.HookEvent{Name: claude.HookPreToolUse, SessionID: sid, ToolName: "Bash"}, t0)
		require.NoError(t, d.Rescan())

		t.Run("it should still say the session is hooked", func(t *testing.T) {
			assert.True(t, sessionIn(t, d.Snapshot(), sid).Hooked)
		})

		t.Run("it should make a view without allocating", func(t *testing.T) {
			assert.Zero(t, testing.AllocsPerRun(20, func() { d.Snapshot() }))
		})
	})
}

// Item 213: sharing the loaded sessions is safe only while nothing that
// is handed a view writes to it, so every consumer is handed one here and
// the loaded sessions are held to a deep copy taken before any saw them.
func TestSharedViewsAreNotWrittenTo(t *testing.T) {
	t.Run("when a view shared with the loaded sessions is handed to every consumer, in the daemon and through its socket", func(t *testing.T) {
		home := idleHome(t, "interactive")
		loads := &keptLoads{Loader: state.NewLoader(home.Path, probes)}
		d := daemon.NewWith(loads, func() time.Time { return now })
		d.SetIndex(find.New(home.Path))
		require.NoError(t, d.Rescan())
		updates, cancel := d.Subscribe()
		t.Cleanup(cancel)
		require.NoError(t, d.Rescan())
		<-updates
		d.Snapshot()
		d.Events(time.Time{})
		_, err := d.Find(find.Query{Words: "ci queue", Limit: 5})
		require.NoError(t, err)
		sock := serve(t, d)
		client, err := proto.Dial(sock)
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Close() })
		streaming, err := proto.Dial(sock)
		require.NoError(t, err)
		t.Cleanup(func() { _ = streaming.Close() })
		_, err = client.Snapshot()
		require.NoError(t, err)
		sub, err := streaming.Subscribe(time.Time{})
		require.NoError(t, err)
		<-sub
		_, err = client.Events(time.Time{})
		require.NoError(t, err)
		_, err = client.Find(find.Query{Words: "ci queue", Limit: 5})
		require.NoError(t, err)

		t.Run("it should leave the loaded sessions as they were loaded", func(t *testing.T) {
			assert.Equal(t, loads.before, loads.loaded)
		})
	})
}
