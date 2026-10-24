package targets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tierPackage = `package acceptance_test

import (
	"os"
	"testing"

	"botropolis.city/botropolis/pkg/measure"
)

func performanceTier(t *testing.T) { t.Helper() }

type meter struct{}

func newMeter() *meter { return &meter{} }

func work() measure.Work { return measure.Take() }

func plain() string { return "nothing measured" }

func findProcess() bool {
	_, err := os.ReadFile("/proc/1/comm")
	return err == nil
}

func TestCounted(t *testing.T) {
	_ = work()
}

func TestCPU(t *testing.T) {
	performanceTier(t)
	_ = newMeter().cpu()
}

func TestInstructions(t *testing.T) {
	performanceTier(t)
	_ = measure.CountInstructions.Load()
}

func TestBehaviour(t *testing.T) {
	_ = plain()
}

func TestSkipsFirst(t *testing.T) {
	if testing.Short() {
		t.Skip("long")
	}
	performanceTier(t)
	_ = newMeter().cpu()
}

func TestFindsAProcess(t *testing.T) {
	_ = findProcess()
}

func TestNamesALocalLikeAMeter(t *testing.T) {
	cpu := 1
	_ = cpu
}

func TestUnmarkedButMeasures(t *testing.T) {
	t.Run("deep down", func(t *testing.T) { _ = newMeter().cpu() })
}

func TestMarkedButMeasuresNothing(t *testing.T) {
	performanceTier(t)
	_ = plain()
}

func TestMain(m *testing.M) { os.Exit(m.Run()) }

func BenchmarkIgnored(b *testing.B) {}
`

const metersFile = `package acceptance_test

import "os"

func (m *meter) cpu() int64 {
	data, _ := os.ReadFile("/proc/self/stat")
	return int64(len(data))
}
`

func TestReadTiers(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tier_test.go"), []byte(tierPackage), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, MetersFile), []byte(metersFile), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "elsewhere_test.go"), []byte("//go:build never\n\npackage acceptance_test\n\nimport \"testing\"\n\nfunc TestElsewhere(t *testing.T) {}\n"), 0o644))
	tests, err := ReadTiers(dir, mod+"/pkg/measure")
	require.NoError(t, err)

	t.Run("when the package's test files are parsed", func(t *testing.T) {
		t.Run("it should find every top-level Test function this platform builds, and nothing else", func(t *testing.T) {
			assert.ElementsMatch(t, []string{"TestCounted", "TestCPU", "TestInstructions", "TestBehaviour", "TestSkipsFirst", "TestFindsAProcess", "TestNamesALocalLikeAMeter", "TestUnmarkedButMeasures", "TestMarkedButMeasuresNothing"}, keys(tests))
		})
	})

	for _, c := range []struct {
		when, test    string
		tier, reaches Tier
	}{
		{"and a test reads pkg/measure's work counters", "TestCounted", Behaviour, Behaviour},
		{"and a test calls performanceTier(t) and reaches a meter through a method", "TestCPU", Performance, Performance},
		{"and a test calls performanceTier(t) and reaches one of pkg/measure's meters", "TestInstructions", Performance, Performance},
		{"and a test calls no marker and measures nothing", "TestBehaviour", Behaviour, Behaviour},
		{"and a test skips before it calls performanceTier(t)", "TestSkipsFirst", Performance, Performance},
		{"and a test reads /proc outside the meters", "TestFindsAProcess", Behaviour, Behaviour},
		{"and a test names a local variable like a meter method", "TestNamesALocalLikeAMeter", Behaviour, Behaviour},
		{"and a test calls no marker but a subtest measures", "TestUnmarkedButMeasures", Behaviour, Performance},
		{"and a test calls performanceTier(t) but measures nothing", "TestMarkedButMeasuresNothing", Performance, Behaviour},
	} {
		t.Run(c.when, func(t *testing.T) {
			t.Run("it should read the tier its marker names", func(t *testing.T) {
				assert.Equal(t, c.tier, tests[c.test].Tier)
			})

			t.Run("it should work out the tier what it reaches needs", func(t *testing.T) {
				assert.Equal(t, c.reaches, tests[c.test].Reaches)
			})
		})
	}
}

func TestInTier(t *testing.T) {
	tests := map[string]Test{
		"TestB": {Tier: Performance},
		"TestA": {Tier: Performance},
		"TestC": {Tier: Behaviour},
	}

	t.Run("when the performance tier is asked for", func(t *testing.T) {
		t.Run("it should match exactly its tests, sorted", func(t *testing.T) {
			assert.Equal(t, "^(TestA|TestB)$", RunPattern(InTier(tests, Performance)))
		})
	})

	t.Run("when no test is asked for", func(t *testing.T) {
		t.Run("it should match nothing", func(t *testing.T) {
			assert.Equal(t, "^$", RunPattern(nil))
		})
	})

	t.Run("when RUN matches a performance test", func(t *testing.T) {
		names, err := Matching(InTier(tests, Performance), "^TestB$")
		require.NoError(t, err)

		t.Run("it should run that test alone", func(t *testing.T) {
			assert.Equal(t, []string{"TestB"}, names)
		})
	})

	t.Run("when RUN matches only tests outside the tier", func(t *testing.T) {
		_, err := Matching(InTier(tests, Performance), "^TestC$")

		t.Run("it should refuse, saying nothing in the tier matches", func(t *testing.T) {
			assert.ErrorContains(t, err, "matches no test")
		})
	})

	t.Run("when RUN is empty", func(t *testing.T) {
		names, err := Matching(InTier(tests, Performance), "")
		require.NoError(t, err)

		t.Run("it should run the whole tier", func(t *testing.T) {
			assert.Equal(t, []string{"TestA", "TestB"}, names)
		})
	})
}

func keys[V any](m map[string]V) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
