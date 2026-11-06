package render

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 352: the footer's status line has no callers. Every passing
// message is a notice, a prompt is a dialog and progress is on the
// button, so nothing writes to the old line, and it is gone.
func TestSetStatusHasNoCallers(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	call := regexp.MustCompile(`\bSetStatus\(`)
	var found []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		text, err := os.ReadFile(f)
		require.NoError(t, err)
		for i, line := range strings.Split(string(text), "\n") {
			if call.MatchString(line) {
				found = append(found, fmt.Sprintf("%s:%d", f, i+1))
			}
		}
	}

	t.Run("it should find no call to SetStatus, nor SetStatus itself, in pkg/render", func(t *testing.T) {
		assert.Empty(t, found)
	})
}
