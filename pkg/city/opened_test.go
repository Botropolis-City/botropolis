package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// Item 159: until a snapshot opens the city, the window shows how far
// the first load has got instead of an empty island.
func TestSceneOpened(t *testing.T) {
	t.Run("when no snapshot has come", func(t *testing.T) {
		t.Run("it should not be open", func(t *testing.T) {
			assert.False(t, city.NewScene(city.NewLayout()).Opened())
		})
	})

	t.Run("when a snapshot has come", func(t *testing.T) {
		t.Run("it should be open", func(t *testing.T) {
			assert.True(t, scene(t).Opened())
		})
	})
}

// Item 202: a recording opens the city only once its history is read,
// so its first frame stands on the heights the rest will, and no film or
// frame test sees every building grow a storey mid-shot.
func TestSceneOpenedOnARead(t *testing.T) {
	waiting := func(t *testing.T) *city.Scene {
		t.Helper()
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetOpenOnRead(true)
		return s
	}
	unread, read := snapshot(), snapshot()
	read.HeftsRead = true

	t.Run("when the scene opens on a read history, and a snapshot's history is not read", func(t *testing.T) {
		s := waiting(t)
		s.SetSnapshot(unread)

		t.Run("it should not be open", func(t *testing.T) {
			assert.False(t, s.Opened())
		})

		t.Run("and then a snapshot whose history is read arrives", func(t *testing.T) {
			s.SetSnapshot(read)

			t.Run("it should be open", func(t *testing.T) {
				assert.True(t, s.Opened())
			})
		})
	})

	t.Run("when the scene opens on a read history, and it is opened anyway after an unread snapshot", func(t *testing.T) {
		s := waiting(t)
		s.SetSnapshot(unread)
		s.OpenAnyway()

		t.Run("it should be open", func(t *testing.T) {
			assert.True(t, s.Opened())
		})
	})

	t.Run("when it is opened anyway before any snapshot has come", func(t *testing.T) {
		s := waiting(t)
		s.OpenAnyway()

		t.Run("it should not be open", func(t *testing.T) {
			assert.False(t, s.Opened())
		})
	})

	for _, c := range []struct {
		name  string
		snaps []bool
		want  bool
	}{
		{"no snapshot has come", nil, false},
		{"an unread snapshot has come", []bool{false}, true},
		{"a read snapshot has come", []bool{false, true}, false},
	} {
		t.Run("when the scene opens on a read history, and "+c.name, func(t *testing.T) {
			s := waiting(t)
			for _, isRead := range c.snaps {
				if isRead {
					s.SetSnapshot(read)
				} else {
					s.SetSnapshot(unread)
				}
			}

			t.Run("it should say whether it is waiting for a read", func(t *testing.T) {
				assert.Equal(t, c.want, s.WaitingForRead())
			})
		})
	}

	t.Run("when the scene does not wait for a read, and a snapshot's history is not read", func(t *testing.T) {
		s := city.NewScene(city.NewLayout())
		s.SetSnapshot(unread)

		t.Run("it should be open", func(t *testing.T) {
			assert.True(t, s.Opened())
		})
	})
}
