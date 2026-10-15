package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The documentation structure is checked by tools/check-docs, which make lint
// invokes. That dependency is one line in a Makefile with nothing guarding it:
// delete it and six checks go quiet at once behind a green gate. Asserting the
// dependency from inside the thing it invokes would be circular, and grepping
// the Makefile is a string match on a build file, so this is the other answer —
// a second, independent way in. The checks then survive losing either caller.
//
// This repo has already paid for the difference: tools/shrink-pngs sat in the
// Makefile for fourteen revisions while the path that actually ran bypassed it,
// at 6.6 MB of binary. A guard that is not invoked cannot fail.
//
// readDocs exists because a cached test is a test that did not run, and that is
// the same sentence again. Go keys the test cache on files the test binary
// itself opens; check-docs reads the documentation in a subprocess, which the
// cache cannot see. So with an orphaned file present this test reported
// "ok (cached)" while check-docs failed on the same tree — verified. Opening
// every file the script inspects puts them in the key, so any documentation
// change re-runs this rather than replaying a pass taken against a different
// tree. It is not a redundant read; it is the cache key.
func readDocs(t *testing.T) int {
	t.Helper()
	root := filepath.Join("..", "..")
	seen := 0
	for _, dir := range []string{".", "design", "rules"} {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			t.Fatalf("reading %s: %v", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			if _, err := os.ReadFile(filepath.Join(root, dir, e.Name())); err != nil {
				t.Fatalf("reading %s/%s: %v", dir, e.Name(), err)
			}
			seen++
		}
	}
	return seen
}

func TestDocumentationStructure(t *testing.T) {
	t.Run("when the repository's documentation is checked", func(t *testing.T) {
		readDocs(t)
		out, err := exec.Command("../../tools/check-docs").CombinedOutput()

		t.Run("and the structure holds", func(t *testing.T) {
			t.Run("it should report no problems", func(t *testing.T) {
				assert.NoError(t, err, string(out))
			})
		})
	})
}
