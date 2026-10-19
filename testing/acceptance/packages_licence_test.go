package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

// The binaries embed Inter (OFL-1.1) and Kenney's atlases (CC0-1.0), so every
// package must declare both beside MIT (item 121). tools/check-packages reads
// the fields; these run it against the repository and against mutated copies.

const (
	nfpmConfig   = "packaging/nfpm.yaml"
	brewTemplate = "packaging/brew/botropolis.rb.in"
)

var allLicences = []string{"MIT", "OFL-1.1", "CC0-1.0"}

// licenceField is how one packaging file declares a set of licences.
type licenceField struct {
	name   string
	file   string
	prefix string
	write  func(licences []string, join string) string
}

var licenceFields = []licenceField{
	{"nfpm", nfpmConfig, "license:", func(ls []string, join string) string {
		return "license: " + strings.Join(ls, " "+join+" ")
	}},
	{"Homebrew", brewTemplate, "license ", func(ls []string, join string) string {
		quoted := make([]string, len(ls))
		for i, l := range ls {
			quoted[i] = `"` + l + `"`
		}
		key := "all_of"
		if join == "OR" {
			key = "any_of"
		}
		return "  license " + key + ": [" + strings.Join(quoted, ", ") + "]"
	}},
}

func runLicenceCheck(t *testing.T, root string) (string, error) {
	t.Helper()
	out, err := exec.Command(helpers.Tool(t, "check-packages"), "--licences", "--root", root).CombinedOutput()
	return string(out), err
}

// packagingCopy writes both packaging files into a temporary tree, each with
// its licence line replaced by the given one ("" removes it).
func packagingCopy(t *testing.T, lines map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range licenceFields {
		var kept []string
		for _, l := range strings.Split(readRepoFile(t, f.file), "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), f.prefix) {
				if lines[f.name] != "" {
					kept = append(kept, lines[f.name])
				}
				continue
			}
			kept = append(kept, l)
		}
		dst := filepath.Join(root, f.file)
		require.NoError(t, os.MkdirAll(filepath.Dir(dst), 0o755))
		require.NoError(t, os.WriteFile(dst, []byte(strings.Join(kept, "\n")), 0o644))
	}
	return root
}

func declaring(licences []string, join string) map[string]string {
	lines := map[string]string{}
	for _, f := range licenceFields {
		lines[f.name] = f.write(licences, join)
	}
	return lines
}

func without(drop string) []string {
	var kept []string
	for _, l := range allLicences {
		if l != drop {
			kept = append(kept, l)
		}
	}
	return kept
}

func TestCheckPackagesLicences(t *testing.T) {
	t.Run("when the repository's own nfpm and Homebrew fields are checked", func(t *testing.T) {
		out, err := runLicenceCheck(t, repoRoot(t))

		t.Run("it should pass", func(t *testing.T) {
			assert.NoError(t, err, out)
		})
	})

	t.Run("when both fields declare MIT, OFL-1.1 and CC0-1.0", func(t *testing.T) {
		out, err := runLicenceCheck(t, packagingCopy(t, declaring(allLicences, "AND")))

		t.Run("it should pass", func(t *testing.T) {
			assert.NoError(t, err, out)
		})
	})

	for _, f := range licenceFields {
		for _, licence := range allLicences {
			t.Run("when the "+f.name+" field drops "+licence, func(t *testing.T) {
				lines := declaring(allLicences, "AND")
				lines[f.name] = f.write(without(licence), "AND")
				out, err := runLicenceCheck(t, packagingCopy(t, lines))

				t.Run("it should fail", func(t *testing.T) {
					assert.Error(t, err, out)
				})

				t.Run("it should name the file and the licence", func(t *testing.T) {
					assert.Regexp(t, regexp.QuoteMeta(f.file)+`.*`+regexp.QuoteMeta(licence), out)
				})
			})
		}

		t.Run("when the "+f.name+" field joins the licences with OR", func(t *testing.T) {
			lines := declaring(allLicences, "AND")
			lines[f.name] = f.write(allLicences, "OR")
			out, err := runLicenceCheck(t, packagingCopy(t, lines))

			t.Run("it should fail", func(t *testing.T) {
				assert.Error(t, err, out)
			})
		})

		t.Run("when the "+f.name+" file has no licence field", func(t *testing.T) {
			lines := declaring(allLicences, "AND")
			lines[f.name] = ""
			out, err := runLicenceCheck(t, packagingCopy(t, lines))

			t.Run("it should fail", func(t *testing.T) {
				assert.Error(t, err, out)
			})
		})
	}
}
