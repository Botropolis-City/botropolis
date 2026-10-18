package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

// The website's install commands link /releases/latest/download/<fixed name>,
// which only works if every release carries its packages under names that do
// not change. tools/release-aliases makes those copies in the release workflow;
// this runs it as the workflow does. helpers.Tool reads the script first so it
// is in the test cache's key: a cached test is a test that did not run.
func runAliases(t *testing.T, version string, files ...string) (string, []byte, error) {
	t.Helper()
	releaseAliases := helpers.Tool(t, "release-aliases")
	dir := t.TempDir()
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), []byte(f), 0o644))
	}
	out, err := exec.Command(releaseAliases, version, dir).CombinedOutput()
	return dir, out, err
}

func TestReleaseAliases(t *testing.T) {
	cases := []struct{ versioned, fixed string }{
		{"botropolis_0.1.5_amd64.deb", "botropolis_amd64.deb"},
		{"botropolis_0.1.5_arm64.deb", "botropolis_arm64.deb"},
		{"botropolis-0.1.5-1.x86_64.rpm", "botropolis.x86_64.rpm"},
		{"botropolis-0.1.5-1.aarch64.rpm", "botropolis.aarch64.rpm"},
		{"botropolis-0.1.5-1-x86_64.pkg.tar.zst", "botropolis-x86_64.pkg.tar.zst"},
		{"botropolis-0.1.5-1-aarch64.pkg.tar.zst", "botropolis-aarch64.pkg.tar.zst"},
		{"botropolis-0.1.5-linux-amd64.tar.gz", "botropolis-linux-amd64.tar.gz"},
		{"botropolis-0.1.5-linux-arm64.tar.gz", "botropolis-linux-arm64.tar.gz"},
	}

	t.Run("when a release holds every package format", func(t *testing.T) {
		var files []string
		for _, c := range cases {
			files = append(files, c.versioned)
		}
		dir, out, err := runAliases(t, "0.1.5", files...)
		require.NoError(t, err, string(out))

		for _, c := range cases {
			t.Run("and it holds "+c.versioned, func(t *testing.T) {
				t.Run("it should copy it to "+c.fixed, func(t *testing.T) {
					got, err := os.ReadFile(filepath.Join(dir, c.fixed))
					require.NoError(t, err)
					assert.Equal(t, c.versioned, string(got))
				})
			})
		}
	})

	t.Run("when a version's dots could match any character", func(t *testing.T) {
		_, out, err := runAliases(t, "0.1.5", "botropolis_0x1x5_amd64.deb")

		t.Run("it should refuse a name that does not hold the version", func(t *testing.T) {
			assert.Error(t, err, string(out))
		})
	})

	t.Run("when a file does not name the version", func(t *testing.T) {
		_, out, err := runAliases(t, "0.1.5", "botropolis_0.1.4_amd64.deb")

		t.Run("it should refuse", func(t *testing.T) {
			assert.Error(t, err)
		})

		t.Run("it should name the file", func(t *testing.T) {
			assert.Contains(t, string(out), "botropolis_0.1.4_amd64.deb")
		})
	})

	t.Run("when two files would share a fixed name", func(t *testing.T) {
		_, out, err := runAliases(t, "0.1.5", "botropolis_0.1.5_amd64.deb", "botropolis_0.1.5-2_amd64.deb")

		t.Run("it should refuse", func(t *testing.T) {
			assert.Error(t, err, string(out))
		})
	})

	t.Run("when SHA256SUMS is in the directory", func(t *testing.T) {
		dir, out, err := runAliases(t, "0.1.5", "botropolis_0.1.5_amd64.deb", "SHA256SUMS")
		require.NoError(t, err, string(out))

		t.Run("it should leave it alone", func(t *testing.T) {
			entries, _ := os.ReadDir(dir)
			assert.Len(t, entries, 3)
		})
	})
}
