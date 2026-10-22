package claude_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
)

// Item 160: the probe used to start every MCP server the user has
// configured, and the budget for background helpers says one at a time.

func TestRefreshUtilization(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	t.Run("when the probe runs", func(t *testing.T) {
		bin, out := fakeClaude(t, `printf '%s\n' "$@" > "$OUT.args"`)
		claude.RefreshUtilization(context.Background(), bin)
		args, err := os.ReadFile(out + ".args")
		require.NoError(t, err)

		t.Run("it should run print mode on /usage with no user settings and no MCP servers", func(t *testing.T) {
			assert.Equal(t, "-p\n/usage\n--setting-sources\n\n--strict-mcp-config\n", string(args))
		})
	})
}

type helperRun struct {
	name string
	run  func(ctx context.Context, bin string)
}

var helperRuns = []helperRun{
	{"usage probe", func(ctx context.Context, bin string) { claude.RefreshUtilization(ctx, bin) }},
	{"summary", func(ctx context.Context, bin string) { _, _ = claude.Summarise(ctx, bin, "User: hello") }},
}

// heldClaude is a fake that says it started and then holds its slot
// until the test releases it.
func heldClaude(t *testing.T) (bin, out string) {
	return fakeClaude(t, `touch "$OUT.started"; while [ ! -e "$OUT.release" ]; do sleep 0.01; done; cat > /dev/null; echo done`)
}

func started(out string) bool {
	_, err := os.Stat(out + ".started")
	return err == nil
}

func release(t *testing.T, out string) {
	require.NoError(t, os.WriteFile(out+".release", nil, 0o644))
}

func hold(t *testing.T, first helperRun) (out string, done chan struct{}) {
	bin, out := heldClaude(t)
	done = make(chan struct{})
	go func() { first.run(context.Background(), bin); close(done) }()
	require.Eventually(t, func() bool { return started(out) }, 5*time.Second, 10*time.Millisecond)
	return out, done
}

func TestOneHelperAtATime(t *testing.T) {
	for _, first := range helperRuns {
		t.Run("when a "+first.name+" is running", func(t *testing.T) {
			for _, second := range helperRuns {
				t.Run("and a "+second.name+" is asked for", func(t *testing.T) {
					t.Setenv("HOME", t.TempDir())
					t.Setenv("XDG_STATE_HOME", t.TempDir())
					firstOut, firstDone := hold(t, first)

					bin, out := heldClaude(t)
					release(t, out)
					secondDone := make(chan struct{})
					go func() { second.run(context.Background(), bin); close(secondDone) }()
					time.Sleep(300 * time.Millisecond)
					startedWhileHeld := started(out)
					release(t, firstOut)
					<-firstDone
					<-secondDone

					t.Run("it should not start claude until the first has finished", func(t *testing.T) {
						assert.Equal(t, []bool{false, true}, []bool{startedWhileHeld, started(out)})
					})
				})
			}
		})
	}

	t.Run("when a helper is running", func(t *testing.T) {
		for _, second := range helperRuns {
			t.Run("and a "+second.name+" is asked for with a context that ends first", func(t *testing.T) {
				t.Setenv("HOME", t.TempDir())
				t.Setenv("XDG_STATE_HOME", t.TempDir())
				firstOut, firstDone := hold(t, helperRuns[0])

				bin, out := heldClaude(t)
				release(t, out)
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()
				second.run(ctx, bin)
				startedAtAll := started(out)
				release(t, firstOut)
				<-firstDone

				t.Run("it should give up without starting claude", func(t *testing.T) {
					assert.False(t, startedAtAll)
				})
			})
		}
	})
}
