package targets

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureTests = `package fixture_test

import "testing"

func TestPasses(t *testing.T) {}

func TestFails(t *testing.T) { t.Fatal("the wrong answer") }

func TestPanics(t *testing.T) { panic("the fixture panics") }
`

func TestRunPackage(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a test binary")
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/fixture\n\ngo 1.27\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fixture_test.go"), []byte(fixtureTests), 0o644))
	t.Setenv("GOWORK", "off")

	t.Run("when a package with a passing, a failing and a panicking test is run", func(t *testing.T) {
		var out bytes.Buffer
		code := RunPackage(dir, "example.com/fixture/measure", "", 2, time.Minute, &out)

		t.Run("it should exit non-zero", func(t *testing.T) {
			assert.NotEqual(t, 0, code)
		})

		t.Run("it should name the failing test", func(t *testing.T) {
			assert.Contains(t, out.String(), "FAIL TestFails")
		})

		t.Run("it should print what the failing test said", func(t *testing.T) {
			assert.Contains(t, out.String(), "the wrong answer")
		})

		t.Run("it should name the panicking test", func(t *testing.T) {
			assert.Contains(t, out.String(), "FAIL TestPanics")
		})

		t.Run("it should pass the passing test", func(t *testing.T) {
			assert.Contains(t, out.String(), "ok   TestPasses")
		})

		t.Run("it should count all three", func(t *testing.T) {
			assert.Contains(t, out.String(), "3 tests: 1 passed, 0 skipped, 2 failed")
		})
	})

	t.Run("when only the passing test is asked for", func(t *testing.T) {
		var out bytes.Buffer
		code := RunPackage(dir, "example.com/fixture/measure", "^TestPasses$", 2, time.Minute, &out)

		t.Run("it should exit 0", func(t *testing.T) {
			assert.Equal(t, 0, code, out.String())
		})

		t.Run("it should run that test alone", func(t *testing.T) {
			assert.Contains(t, out.String(), "1 tests: 1 passed")
		})
	})

	t.Run("when the pattern matches no test", func(t *testing.T) {
		var out bytes.Buffer
		code := RunPackage(dir, "example.com/fixture/measure", "^TestNothing$", 2, time.Minute, &out)

		t.Run("it should fail, saying it ran none", func(t *testing.T) {
			assert.NotEqual(t, 0, code, out.String())
		})
	})
}
