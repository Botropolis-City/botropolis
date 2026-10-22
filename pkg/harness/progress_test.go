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

// loadingHarness is a harness partway through a load.
type loadingHarness struct {
	fake
	load state.LoadProgress
}

func (l loadingHarness) LoadProgress() state.LoadProgress { return l.load }

// Item 159.
func TestMultiLoadProgress(t *testing.T) {
	t.Run("when two harnesses are loading and a third cannot say", func(t *testing.T) {
		m := harness.NewMulti(
			loadingHarness{fake: fake{name: "claude"}, load: state.LoadProgress{Done: 40, Total: 120}},
			loadingHarness{fake: fake{name: "codex"}, load: state.LoadProgress{Done: 5, Total: 10}},
			fake{name: "other"},
		)

		t.Run("it should add up the ones that can", func(t *testing.T) {
			assert.Equal(t, state.LoadProgress{Done: 45, Total: 130}, m.LoadProgress())
		})
	})
}

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
