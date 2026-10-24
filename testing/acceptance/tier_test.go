package acceptance_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/tools/acceptance-targets/targets"
)

// performanceTier marks a test that measures CPU or memory (item 226). It
// needs a quiet machine, so it runs only in make test-performance, which
// the performance manager and the release run, and skips everywhere else.
// Every other test is a behaviour test, which make test-acceptance runs in
// a process of its own, beside others. tools/acceptance-targets reads the
// marker, and the guard below works out which tests must carry it.
func performanceTier(t *testing.T) {
	t.Helper()
	if os.Getenv("BOTROPOLIS_PERFORMANCE") == "" {
		t.Skip("measures CPU or memory: set BOTROPOLIS_PERFORMANCE=1, or run make test-performance")
	}
}

const measurePackage = "botropolis.city/botropolis/pkg/measure"

func TestEveryAcceptanceTestIsInTheTierItNeeds(t *testing.T) {
	t.Run("when the acceptance package's files are read", func(t *testing.T) {
		tests, err := targets.ReadTiers(".", measurePackage)
		require.NoError(t, err)

		t.Run("it should find the package's tests", func(t *testing.T) {
			assert.Greater(t, len(tests), 50)
		})

		t.Run("it should find performance tests among them", func(t *testing.T) {
			assert.NotEmpty(t, targets.InTier(tests, targets.Performance))
		})

		names := make([]string, 0, len(tests))
		for name := range tests {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			test := tests[name]
			t.Run("and "+name+" is read", func(t *testing.T) {
				t.Run("it should call performanceTier(t) exactly when it measures CPU or memory", func(t *testing.T) {
					assert.Equal(t, test.Reaches, test.Tier, "%s is marked %s but reaches what a %s test measures", name, test.Tier, test.Reaches)
				})
			})
		}
	})

	t.Run("when a test that measures without the marker is added to a copy of the package", func(t *testing.T) {
		dir := t.TempDir()
		files, err := filepath.Glob("*.go")
		require.NoError(t, err)
		for _, f := range files {
			data, err := os.ReadFile(f)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(dir, f), data, 0o644))
		}
		added := "package acceptance_test\n\nimport \"testing\"\n\nfunc TestMeasuresUnmarked(t *testing.T) {\n\t_ = privateDirty(1)\n}\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "unmarked_test.go"), []byte(added), 0o644))
		tests, err := targets.ReadTiers(dir, measurePackage)
		require.NoError(t, err)

		t.Run("it should find that it needs the performance tier it is not in", func(t *testing.T) {
			assert.NotEqual(t, tests["TestMeasuresUnmarked"].Reaches, tests["TestMeasuresUnmarked"].Tier)
		})
	})
}
