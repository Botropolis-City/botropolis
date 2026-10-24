package targets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const reachPackage = `package acceptance_test

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	setup()
	os.Exit(m.Run())
}

func setup() {
	_ = "setup body"
}

// helper is shared.
func helper() int {
	return 1 // helper body
}

const answer = 42 // answer line

type rig struct{}

func (r rig) open() int {
	return 2 // method body
}

func TestA(t *testing.T) {
	_ = helper()
}

func TestB(t *testing.T) {
	_ = answer
}

func TestC(t *testing.T) {
	_ = "C body"
}

func TestD(t *testing.T) {
	_ = rig{}.open()
}
`

func lineOf(t *testing.T, text, marker string) int {
	t.Helper()
	i := strings.Index(text, marker)
	require.GreaterOrEqual(t, i, 0, marker)
	return strings.Count(text[:i], "\n") + 1
}

func TestReachingChange(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "reach_test.go"), []byte(reachPackage), 0o644))
	at := func(marker string) []LineRange {
		l := lineOf(t, reachPackage, marker)
		return []LineRange{{From: l, To: l}}
	}

	for _, c := range []struct {
		when, marker string
		tests        []string
	}{
		{"when a line in a test's own body changed", `"C body"`, []string{"TestC"}},
		{"when a line in a helper changed", "helper body", []string{"TestA"}},
		{"when a package-level constant changed", "answer line", []string{"TestB"}},
		{"when a method changed", "method body", []string{"TestD"}},
		{"when only the imports changed", `"os"`, nil},
		{"when only a comment between declarations changed", "helper is shared", nil},
	} {
		t.Run(c.when, func(t *testing.T) {
			tests, why := ReachingChange(dir, "reach_test.go", at(c.marker))

			t.Run("it should be able to say", func(t *testing.T) {
				assert.Empty(t, why)
			})

			t.Run("it should name exactly the tests that reach it", func(t *testing.T) {
				assert.Equal(t, c.tests, tests)
			})
		})
	}

	for _, c := range []struct{ when, marker string }{
		{"when TestMain itself changed", "setup()\n\tos.Exit"},
		{"when a function TestMain calls changed", "setup body"},
	} {
		t.Run(c.when, func(t *testing.T) {
			_, why := ReachingChange(dir, "reach_test.go", at(c.marker))

			t.Run("it should not choose, saying TestMain reaches it", func(t *testing.T) {
				assert.Contains(t, why, "TestMain")
			})
		})
	}

	t.Run("when a file that is not Go changed", func(t *testing.T) {
		_, why := ReachingChange(dir, "fixture.json", []LineRange{{From: 1, To: 1}})

		t.Run("it should not choose, naming the file", func(t *testing.T) {
			assert.Contains(t, why, "fixture.json")
		})
	})
}

func TestChangedLines(t *testing.T) {
	diff := "diff --git a/testing/acceptance/a_test.go b/testing/acceptance/a_test.go\n" +
		"index 1..2 100644\n" +
		"--- a/testing/acceptance/a_test.go\n" +
		"+++ b/testing/acceptance/a_test.go\n" +
		"@@ -10,2 +10,3 @@ func TestA(t *testing.T) {\n" +
		"-old\n-old\n+new\n+new\n+new\n" +
		"@@ -40 +41,0 @@ func helper() {\n" +
		"-gone\n" +
		"diff --git a/testing/acceptance/b_test.go b/testing/acceptance/b_test.go\n" +
		"--- a/testing/acceptance/b_test.go\n" +
		"+++ b/testing/acceptance/b_test.go\n" +
		"@@ -5 +5 @@\n" +
		"-x\n+y\n"
	got := ChangedLines(diff)

	t.Run("when a diff with -U0 hunks is read", func(t *testing.T) {
		t.Run("it should give each file the new lines each hunk covers, a deletion as the lines either side of it", func(t *testing.T) {
			assert.Equal(t, map[string][]LineRange{
				"testing/acceptance/a_test.go": {{From: 10, To: 12}, {From: 41, To: 42}},
				"testing/acceptance/b_test.go": {{From: 5, To: 5}},
			}, got)
		})
	})
}
