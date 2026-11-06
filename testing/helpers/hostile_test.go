package helpers_test

import (
	"fmt"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/testing/helpers"
)

// Item 263: make test-unit runs under this config, so a key it leaves at
// its default is a key no test is guarded against.
func TestHostileConfig(t *testing.T) {
	t.Run("when the hostile config is read", func(t *testing.T) {
		v := viper.New()
		v.SetConfigFile(helpers.HostileConfig())
		require.NoError(t, v.ReadInConfig())

		for _, doc := range config.Docs() {
			t.Run("it should set "+doc.Key, func(t *testing.T) {
				assert.True(t, v.IsSet(doc.Key))
			})

			t.Run("it should set "+doc.Key+" to something other than its default", func(t *testing.T) {
				assert.NotEqual(t, doc.Default, fmt.Sprint(v.Get(doc.Key)))
			})
		}

		// Item 350: a binding the hostile config sets must load, or every
		// test under it reads a city that refused to start.
		t.Run("it should bind every key without a clash", func(t *testing.T) {
			cfg := config.Config{Keys: v.GetStringMapString(config.KeySettings), JournalKey: v.GetString(config.KeyJournal),
				TriageKey: v.GetString(config.KeyTriage), StreamKey: v.GetString(config.KeyStream), CityPlanKey: v.GetString(config.KeyCityPlan),
				LabelsKey: v.GetString(config.KeyLabelsToggle), ExplainKey: v.GetString(config.KeyExplain), NewSessionKey: v.GetString(config.KeyNewSession),
				FindKey: v.GetString(config.KeyFind), FindCopyKey: v.GetString(config.KeyFindCopy), FindShowKey: v.GetString(config.KeyFindShow)}
			_, err := config.LoadBindings(cfg, "linux", func(string) bool { return true })
			assert.NoError(t, err)
		})
	})
}
