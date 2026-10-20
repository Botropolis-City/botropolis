package statedir_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/statedir"
)

// Item 165: a test run never reads or writes the state directory it
// started with, which is the developer's own.
func TestGuard(t *testing.T) {
	recovered := func(f func()) (r any) {
		defer func() { r = recover() }()
		f()
		return nil
	}

	t.Run("when a test resolves the store directory the run started with", func(t *testing.T) {
		r := recovered(func() { _ = statedir.Dir() })

		t.Run("it should panic", func(t *testing.T) {
			assert.NotNil(t, r)
		})

		t.Run("and it should name item 165 in the panic", func(t *testing.T) {
			assert.Contains(t, fmt.Sprint(r), "item 165")
		})
	})

	t.Run("when a test checks a path under the state directory the run started with", func(t *testing.T) {
		under := filepath.Join(statedir.Default(), "usage-probe")

		t.Run("it should panic", func(t *testing.T) {
			assert.Panics(t, func() { statedir.Check(under) })
		})
	})

	t.Run("when a test only resolves the default's name", func(t *testing.T) {
		t.Run("it should not panic", func(t *testing.T) {
			assert.NotPanics(t, func() { _ = statedir.Default() })
		})
	})

	t.Run("when a test has pointed XDG_STATE_HOME elsewhere", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", t.TempDir())

		t.Run("it should not panic", func(t *testing.T) {
			assert.NotPanics(t, func() { _ = statedir.Dir() })
		})
	})

	t.Run("when a test has pointed HOME elsewhere and XDG_STATE_HOME is unset", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("XDG_STATE_HOME", "")

		t.Run("it should not panic", func(t *testing.T) {
			assert.NotPanics(t, func() { _ = statedir.Dir() })
		})
	})

	t.Run("when a path only shares the real directory's prefix", func(t *testing.T) {
		sibling := statedir.Default() + "-other"

		t.Run("it should not panic", func(t *testing.T) {
			assert.NotPanics(t, func() { statedir.Check(sibling) })
		})
	})

	t.Run("when the process is not a test binary", func(t *testing.T) {
		dir := "/home/someone/.local/state/botropolis"

		t.Run("it should not panic", func(t *testing.T) {
			assert.NotPanics(t, func() { statedir.Guard(dir, dir, false) })
		})
	})
}
