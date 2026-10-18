package claude_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/statedir"
)

func TestPrivateRunsStartNoClaude(t *testing.T) {
	runs := []struct {
		name string
		run  func(bin string)
	}{
		{"a usage probe", func(bin string) { claude.RefreshUtilization(context.Background(), bin) }},
		{"a summary", func(bin string) { _, _ = claude.Summarise(context.Background(), bin, "User: hello") }},
	}
	for _, private := range []bool{false, true} {
		for _, r := range runs {
			name := "when the state is the person's own, and " + r.name + " is asked for"
			want := "it should run the CLI"
			if private {
				name = "when the state is private, and " + r.name + " is asked for"
				want = "it should not run the CLI"
			}
			t.Run(name, func(t *testing.T) {
				t.Setenv("HOME", t.TempDir())
				t.Setenv("XDG_STATE_HOME", t.TempDir())
				t.Setenv("TMPDIR", t.TempDir())
				if private {
					cleanup, err := statedir.MakePrivate("")
					require.NoError(t, err)
					t.Cleanup(func() { _ = cleanup() })
				}
				bin, out := fakeClaude(t, `cat > /dev/null; touch "$OUT.ran"; echo done`)
				r.run(bin)

				t.Run(want, func(t *testing.T) {
					if private {
						assert.NoFileExists(t, out+".ran")
					} else {
						assert.FileExists(t, out+".ran")
					}
				})
			})
		}
	}
}
