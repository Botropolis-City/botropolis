package acceptance_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readRepoFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", path))
	require.NoError(t, err)
	return string(data)
}

func defaultKits(t *testing.T) []string {
	t.Helper()
	m := regexp.MustCompile(`(?m)^default_kits=\(([^)]*)\)`).FindStringSubmatch(readRepoFile(t, "tools/fetch-kits"))
	require.NotNil(t, m, "tools/fetch-kits has no default_kits")
	return strings.Fields(m[1])
}

func kitName(slug string) string {
	words := strings.Split(slug, "-")
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func thanksSection(t *testing.T) string {
	t.Helper()
	readme := readRepoFile(t, "README.md")
	start := strings.Index(readme, "[Kenney](https://kenney.nl)")
	require.NotEqual(t, -1, start, "README.md does not credit Kenney")
	end := strings.Index(readme[start:], "\n## ")
	require.NotEqual(t, -1, end)
	return readme[start : start+end]
}

func TestReadmeCredits(t *testing.T) {
	t.Run("when the README's thanks are read beside tools/fetch-kits's default kits", func(t *testing.T) {
		thanks := strings.Join(strings.Fields(thanksSection(t)), " ")
		kits := defaultKits(t)

		for _, slug := range kits {
			name := kitName(slug)
			t.Run("it should name the "+name, func(t *testing.T) {
				assert.Contains(t, thanks, name)
			})
		}

		t.Run("and every kit it names is checked against the defaults", func(t *testing.T) {
			fetched := map[string]bool{}
			for _, slug := range kits {
				fetched[kitName(slug)] = true
			}
			var unfetched []string
			for _, name := range regexp.MustCompile(`City Kit [A-Z]\w+|\b[A-Z]\w+ Kit\b`).FindAllString(thanks, -1) {
				if name != "City Kit" && !fetched[name] {
					unfetched = append(unfetched, name)
				}
			}
			t.Run("it should name no kit the pipeline does not fetch", func(t *testing.T) {
				assert.Empty(t, unfetched)
			})
		})

		t.Run("it should say the session buildings are modelled in tools/render-sprites/buildings.py", func(t *testing.T) {
			assert.Contains(t, thanks, "`tools/render-sprites/buildings.py`")
		})

		t.Run("it should thank Inter under the SIL Open Font License", func(t *testing.T) {
			assert.Regexp(t, `\[Inter\]\(https://rsms\.me/inter/\).*SIL Open Font License`, thanks)
		})
	})
}
