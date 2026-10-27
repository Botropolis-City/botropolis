package control_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/control"
)

func TestPaste(t *testing.T) {
	for _, c := range []struct {
		goos, tool string
		argv       []string
	}{
		{"linux", "wl-paste", []string{"wl-paste", "--no-newline", "--type", "text"}},
		{"linux", "xclip", []string{"xclip", "-o", "-selection", "clipboard"}},
		{"linux", "xsel", []string{"xsel", "--clipboard", "--output"}},
		{"darwin", "pbpaste", []string{"pbpaste"}},
	} {
		t.Run("when "+c.tool+" is the clipboard tool on "+c.goos+"'s PATH", func(t *testing.T) {
			runner := &fakeRunner{stdout: "/w/api"}
			_, err := control.New(runner, control.DesktopFor(c.goos), env(nil), onPath(c.tool)).Paste(context.Background())
			require.NoError(t, err)

			t.Run("it should read the clipboard through that tool's output form", func(t *testing.T) {
				assert.Equal(t, []call{{"", c.argv}}, runner.reads)
			})
		})
	}

	t.Run("when wl-paste and xclip are both on the path", func(t *testing.T) {
		runner := &fakeRunner{stdout: "/w/api"}
		_, err := control.New(runner, linux, env(nil), onPath("wl-paste", "xclip")).Paste(context.Background())
		require.NoError(t, err)

		t.Run("it should read through the one copying would use", func(t *testing.T) {
			assert.Equal(t, "wl-paste", runner.reads[0].argv[0])
		})
	})

	t.Run("when the clipboard holds text", func(t *testing.T) {
		got, err := control.New(&fakeRunner{stdout: "/w/api"}, linux, env(nil), onPath("xclip")).Paste(context.Background())
		require.NoError(t, err)

		t.Run("it should return it", func(t *testing.T) {
			assert.Equal(t, "/w/api", got)
		})
	})

	t.Run("when no clipboard tool is on the path", func(t *testing.T) {
		_, err := control.New(&fakeRunner{}, linux, env(nil), onPath()).Paste(context.Background())

		t.Run("it should say which to install", func(t *testing.T) {
			assert.EqualError(t, err, "nothing to paste: no clipboard tool found: install wl-clipboard, xclip or xsel")
		})
	})

	t.Run("when a clipboard tool is on the path", func(t *testing.T) {
		runner := &fakeRunner{stdout: "x"}
		asked := time.Now()
		_, err := control.New(runner, linux, env(nil), onPath("xclip")).Paste(context.Background())
		require.NoError(t, err)

		t.Run("it should hand the tool a deadline no more than two seconds away", func(t *testing.T) {
			assert.WithinDuration(t, asked.Add(control.CopyTimeout), runner.readDeadline, 100*time.Millisecond)
		})
	})

	t.Run("when the tool does not answer before its deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		_, err := control.New(&fakeRunner{readHangs: true}, linux, env(nil), onPath("xclip")).Paste(ctx)

		t.Run("it should say the clipboard did not answer", func(t *testing.T) {
			assert.EqualError(t, err, "the clipboard did not answer within 2s (xclip)")
		})
	})

	t.Run("when the tool prints nothing", func(t *testing.T) {
		_, err := control.New(&fakeRunner{}, linux, env(nil), onPath("xclip")).Paste(context.Background())

		t.Run("it should say the clipboard holds no text", func(t *testing.T) {
			assert.EqualError(t, err, "the clipboard holds no text")
		})
	})
}
