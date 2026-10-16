package helpers_test

import (
	"os"
	"path/filepath"
	"testing"

	"botropolis.city/botropolis/testing/helpers"
	"github.com/stretchr/testify/assert"
)

type recorder struct {
	testing.TB
	failed, skipped bool
}

func (r *recorder) Helper()               {}
func (r *recorder) Fatal(...any)          { r.failed = true }
func (r *recorder) Fatalf(string, ...any) { r.failed = true }
func (r *recorder) Skipf(string, ...any)  { r.skipped = true }

func TestFixtureHome(t *testing.T) {
	t.Run("when the fixture is missing", func(t *testing.T) {
		r := &recorder{TB: t}
		helpers.FixtureHome(r, "no-such-fixture")

		t.Run("it should fail the test", func(t *testing.T) {
			assert.True(t, r.failed)
		})

		t.Run("it should not skip it", func(t *testing.T) {
			assert.False(t, r.skipped)
		})
	})

	t.Run("when the fixture is the checked-in synthetic home", func(t *testing.T) {
		home := helpers.FixtureHome(t, "synthetic")

		t.Run("it should return a home holding a Claude directory", func(t *testing.T) {
			assert.DirExists(t, filepath.Join(home, ".claude"))
		})
	})
}

func TestMain(m *testing.M) {
	os.Exit(helpers.RunIsolated(m))
}

func TestHomeIsolation(t *testing.T) {
	helpers.AssertIsolated(t)
}
