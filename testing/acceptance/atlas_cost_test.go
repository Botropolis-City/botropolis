package acceptance_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

// tools/atlas-cost.py stopped at a NameError for six days and nothing noticed,
// because nothing ran it (item 229). This is its caller.
//
// The tool reads render.py, buildings.py and the shipped manifests in a
// subprocess, which the Go test cache cannot see, so readAtlasInputs opens
// them here to put them in the cache key.

type atlasZoom struct {
	name  string
	pages int
}

func readAtlasInputs(t *testing.T) []atlasZoom {
	t.Helper()
	root := filepath.Join("..", "..")
	for _, f := range []string{"tools/atlas-cost.py", "tools/render-sprites/render.py", "tools/render-sprites/buildings.py"} {
		_, err := os.ReadFile(filepath.Join(root, f))
		require.NoError(t, err)
	}
	manifests, err := filepath.Glob(filepath.Join(root, "pkg", "assets", "kits", "kits-z*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, manifests, "no shipped atlas manifests")
	zooms := make([]atlasZoom, 0, len(manifests))
	for _, m := range manifests {
		data, err := os.ReadFile(m)
		require.NoError(t, err)
		var manifest struct {
			Pages []string `json:"pages"`
		}
		require.NoError(t, json.Unmarshal(data, &manifest))
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(m), "kits-"), ".json")
		zooms = append(zooms, atlasZoom{name: name, pages: len(manifest.Pages)})
	}
	return zooms
}

func runAtlasCost(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("python3", append([]string{helpers.Tool(t, "atlas-cost.py")}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestAtlasCost(t *testing.T) {
	zooms := readAtlasInputs(t)

	t.Run("when atlas-cost runs on the repository's own render.py", func(t *testing.T) {
		out, err := runAtlasCost(t)

		t.Run("it should exit 0", func(t *testing.T) {
			assert.NoError(t, err, out)
		})

		for _, z := range zooms {
			t.Run("and the zoom is "+z.name, func(t *testing.T) {
				t.Run("it should say its page count replays the shipped atlas", func(t *testing.T) {
					assert.Contains(t, out, fmt.Sprintf("%s: %d pages (replays the shipped atlas)", z.name, z.pages))
				})
			})
		}

		t.Run("it should never say MISMATCH", func(t *testing.T) {
			assert.NotContains(t, out, "MISMATCH")
		})

		t.Run("it should price the whole atlas over as many pages as are shipped", func(t *testing.T) {
			shipped := 0
			for _, z := range zooms {
				shipped += z.pages
			}
			priced := -1
			if m := regexp.MustCompile(`MB over (\d+) pages`).FindStringSubmatch(out); m != nil {
				priced, _ = strconv.Atoi(m[1])
			}
			assert.Equal(t, shipped, priced, out)
		})
	})

	t.Run("when atlas-cost runs with --drop-unused", func(t *testing.T) {
		out, err := runAtlasCost(t, "--drop-unused")

		t.Run("it should exit 0", func(t *testing.T) {
			assert.NoError(t, err, out)
		})
	})
}
