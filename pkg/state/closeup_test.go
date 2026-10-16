package state_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/state"
)

// Item 86, slice 4: closing up notes when, what was open, and what you
// pinned to start with next time; ids and times only.
func TestCloseUpRecord(t *testing.T) {
	at := time.Date(2026, 10, 16, 18, 0, 0, 0, time.UTC)
	path := func(t *testing.T) string { return filepath.Join(t.TempDir(), "closeup.json") }

	t.Run("when the desk is closed up", func(t *testing.T) {
		loader := state.NewLoader(t.TempDir(), alive).WithLedger(filepath.Join(t.TempDir(), "sessions.json")).WithCloseUp(path(t))
		require.NoError(t, loader.CloseUp([]string{"a", "b"}, []string{"b"}, at))
		got, err := loader.LastCloseUp()
		require.NoError(t, err)

		t.Run("it should record when", func(t *testing.T) {
			assert.Equal(t, at, got.At)
		})

		t.Run("it should record which sessions were open", func(t *testing.T) {
			assert.Equal(t, []string{"a", "b"}, got.Open)
		})

		t.Run("it should record which were pinned", func(t *testing.T) {
			assert.Equal(t, []string{"b"}, got.Pinned)
		})
	})

	t.Run("when it is saved and loaded", func(t *testing.T) {
		p := path(t)
		want := state.CloseUp{At: at, Open: []string{"a"}, Pinned: []string{"a"}}
		require.NoError(t, want.Save(p))
		got, err := state.LoadCloseUp(p)
		require.NoError(t, err)

		t.Run("it should keep all three", func(t *testing.T) {
			assert.Equal(t, want, got)
		})
	})

	t.Run("when it is closed up again", func(t *testing.T) {
		p := path(t)
		loader := state.NewLoader(t.TempDir(), alive).WithCloseUp(p)
		require.NoError(t, loader.CloseUp([]string{"a"}, nil, at))
		require.NoError(t, loader.CloseUp([]string{"c"}, []string{"c"}, at.Add(24*time.Hour)))
		got, err := state.LoadCloseUp(p)
		require.NoError(t, err)

		t.Run("the newer close-up should replace the older", func(t *testing.T) {
			assert.Equal(t, []string{"c"}, got.Open)
		})
	})

	t.Run("when the loader keeps no close-up", func(t *testing.T) {
		err := state.NewLoader(t.TempDir(), alive).CloseUp([]string{"a"}, nil, at)

		t.Run("it should refuse, saying why", func(t *testing.T) {
			assert.ErrorContains(t, err, "keeps no")
		})
	})
}
