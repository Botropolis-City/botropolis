package acceptance_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
)

// Item 167: the README named BOTROPOLIS_PARKED_DAYS for a year after the
// key became shuttered_days. Every variable it names must set a key.
func TestReadmeNamesRealVariables(t *testing.T) {
	keys := config.Keys(config.NewViper())
	named := regexp.MustCompile(`BOTROPOLIS_[A-Z_]+`).FindAllString(readRepoFile(t, "README.md"), -1)
	require.NotEmpty(t, named)

	for _, v := range slices.Compact(slices.Sorted(slices.Values(named))) {
		t.Run("when the README names "+v+", it should set a config key", func(t *testing.T) {
			assert.Contains(t, keys, strings.ToLower(strings.TrimPrefix(v, "BOTROPOLIS_")))
		})
	}
}
