package acceptance_test

import (
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/app"
)

func gunzipped(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	r, err := gzip.NewReader(f)
	require.NoError(t, err)
	text, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(text)
}

// commandPages are the pages a tree needs: one per command help lists.
func commandPages(cmd *cobra.Command) []string {
	pages := []string{strings.ReplaceAll(cmd.CommandPath(), " ", "-") + ".1.gz"}
	for _, c := range cmd.Commands() {
		if c.IsAvailableCommand() {
			pages = append(pages, commandPages(c)...)
		}
	}
	return pages
}

// Item 167: every command botropolis lists has a manual page, and so do
// the daemon, the hook and the config file, built by make man.
func TestManPages(t *testing.T) {
	dir := t.TempDir()
	out, err := exec.Command("go", "run", "../../tools/man", "-out", dir, "-date", "2026-10-20").CombinedOutput()
	require.NoError(t, err, "%s", out)
	root, err := app.NewRoot()
	require.NoError(t, err)

	pages := append(commandPages(root), "botropolisd.1.gz", "botropolis-hook.1.gz", "botropolis-config.5.gz")
	for _, page := range pages {
		t.Run("when the pages are built, it should write "+page, func(t *testing.T) {
			assert.FileExists(t, filepath.Join(dir, page))
		})
	}

	t.Run("when a command takes an argument in angle brackets", func(t *testing.T) {
		t.Run("it should keep it in the synopsis", func(t *testing.T) {
			assert.Contains(t, gunzipped(t, filepath.Join(dir, "botropolis-attach.1.gz")), "attach <id>")
		})
	})

	for _, page := range []string{"botropolis-attach.1.gz", "botropolisd.1.gz", "botropolis-config.5.gz"} {
		t.Run("when "+page+" is built, it should not name the machine it was built on", func(t *testing.T) {
			text := gunzipped(t, filepath.Join(dir, page))
			assert.False(t, strings.Contains(text, os.Getenv("HOME")) || strings.Contains(text, "/run/user/"), "%s", text)
		})
	}

	t.Run("when the pages are built, it should write no page help does not list", func(t *testing.T) {
		written, err := filepath.Glob(filepath.Join(dir, "*.gz"))
		require.NoError(t, err)
		assert.Len(t, written, len(pages))
	})
}
