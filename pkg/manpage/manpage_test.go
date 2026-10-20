package manpage_test

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/manpage"
)

var date = time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC)

func tree() *cobra.Command {
	run := func(*cobra.Command, []string) error { return nil }
	root := &cobra.Command{Use: "tool", Short: "A tool", RunE: run}
	goCmd := &cobra.Command{Use: "go", Short: "Go", RunE: run}
	goCmd.AddCommand(&cobra.Command{Use: "fast", Short: "Go fast", RunE: run})
	root.AddCommand(goCmd,
		&cobra.Command{Use: "old", Short: "Old", Deprecated: "use go", RunE: run},
		&cobra.Command{Use: "secret", Short: "Secret", Hidden: true, RunE: run},
	)
	root.InitDefaultHelpCmd()
	return root
}

func page(t *testing.T, path string) string {
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

func TestWrite(t *testing.T) {
	t.Run("when the pages of a command tree are written", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, manpage.Write(dir, date, tree()))

		for _, name := range []string{"tool.1.gz", "tool-go.1.gz", "tool-go-fast.1.gz"} {
			t.Run("it should write "+name, func(t *testing.T) {
				assert.FileExists(t, filepath.Join(dir, name))
			})
		}

		for _, name := range []string{"tool-old.1.gz", "tool-secret.1.gz", "tool-help.1.gz"} {
			t.Run("it should not write "+name, func(t *testing.T) {
				assert.NoFileExists(t, filepath.Join(dir, name))
			})
		}

		t.Run("it should date the page as given", func(t *testing.T) {
			assert.Contains(t, page(t, filepath.Join(dir, "tool.1.gz")), "Oct 2026")
		})

		t.Run("it should not credit the generator", func(t *testing.T) {
			assert.NotContains(t, page(t, filepath.Join(dir, "tool.1.gz")), "Auto generated")
		})
	})

	t.Run("when the same tree is written twice", func(t *testing.T) {
		first, second := t.TempDir(), t.TempDir()
		require.NoError(t, manpage.Write(first, date, tree()))
		require.NoError(t, manpage.Write(second, date, tree()))
		a, err := os.ReadFile(filepath.Join(first, "tool.1.gz"))
		require.NoError(t, err)
		b, err := os.ReadFile(filepath.Join(second, "tool.1.gz"))
		require.NoError(t, err)

		t.Run("it should write the same bytes, so packages build reproducibly", func(t *testing.T) {
			assert.Equal(t, a, b)
		})
	})
}

func TestWriteConfig(t *testing.T) {
	docs := []config.KeyDoc{
		{Key: "liveliness", Help: "how much the city moves", Default: "steady", Env: "BOTROPOLIS_LIVELINESS"},
		{Key: "terminal", Help: "the terminal to attach in", Env: "BOTROPOLIS_TERMINAL"},
	}
	dir := t.TempDir()
	require.NoError(t, manpage.WriteConfig(dir, date, docs))
	text := page(t, filepath.Join(dir, "botropolis-config.5.gz"))

	t.Run("it should be section 5", func(t *testing.T) {
		assert.Contains(t, text, `.TH "BOTROPOLIS-CONFIG" "5"`)
	})

	for _, want := range []string{"liveliness", "how much the city moves", "steady", "BOTROPOLIS_LIVELINESS", "terminal", "BOTROPOLIS_TERMINAL"} {
		t.Run("it should name "+want, func(t *testing.T) {
			assert.Contains(t, text, want)
		})
	}

	t.Run("it should say how to set a key for one run", func(t *testing.T) {
		assert.Contains(t, text, `\-\-set key=value`)
	})
}
