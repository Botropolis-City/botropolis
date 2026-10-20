package acceptance_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// Item 167: every manual page make man writes is installed by the deb,
// the rpm and the Arch package, which nfpm builds from one config, and by
// the Homebrew formula.
func TestPackagesInstallManPages(t *testing.T) {
	dir := t.TempDir()
	out, err := exec.Command("go", "run", "../../tools/man", "-out", dir, "-date", "2026-10-20").CombinedOutput()
	require.NoError(t, err, "%s", out)
	pages, err := filepath.Glob(filepath.Join(dir, "*.gz"))
	require.NoError(t, err)
	require.NotEmpty(t, pages)

	var nfpm struct {
		Contents []struct{ Src, Dst string }
	}
	require.NoError(t, yaml.Unmarshal([]byte(readRepoFile(t, nfpmConfig)), &nfpm))

	for _, page := range pages {
		name := filepath.Base(page)
		section := filepath.Ext(strings.TrimSuffix(name, ".gz"))[1:]
		want := "/usr/share/man/man" + section + "/"
		t.Run("when nfpm packages "+name+", it should install it under "+want, func(t *testing.T) {
			installed := false
			for _, c := range nfpm.Contents {
				if ok, _ := filepath.Match(c.Src, "./bin/man/"+name); ok && c.Dst == want {
					installed = true
				}
			}
			assert.True(t, installed)
		})
	}

	brew := readRepoFile(t, brewTemplate)
	for _, install := range []string{"man1.install", "man5.install"} {
		t.Run("when Homebrew installs the formula, it should run "+install, func(t *testing.T) {
			assert.Contains(t, brew, install)
		})
	}
}
