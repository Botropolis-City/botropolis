package harness_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/harness"
	"botropolis.city/botropolis/pkg/state"
)

// scanning is a harness partway through a long read of its history.
type scanning struct {
	fake
	scan *state.ScanProgress
}

func (s scanning) Progress() *state.ScanProgress { return s.scan }

// Item 159.
func TestMultiProgress(t *testing.T) {
	scan := &state.ScanProgress{Done: 16 << 20, Known: 64 << 20}
	reading := scanning{fake: fake{name: "claude", snapshot: state.Snapshot{Scan: scan}}, scan: scan}
	quiet := fake{name: "codex"}

	t.Run("when one of two harnesses is reading its history", func(t *testing.T) {
		m := harness.NewMulti(quiet, reading)
		snapshot, err := m.Load(now)
		require.NoError(t, err)

		t.Run("it should put that progress on the merged snapshot", func(t *testing.T) {
			assert.Equal(t, scan, snapshot.Scan)
		})

		t.Run("it should report it as the city's progress", func(t *testing.T) {
			assert.Equal(t, scan, m.Progress())
		})
	})

	t.Run("when no harness is reading", func(t *testing.T) {
		t.Run("it should report none", func(t *testing.T) {
			assert.Nil(t, harness.NewMulti(quiet).Progress())
		})
	})
}
