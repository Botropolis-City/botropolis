package app

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 274: sessions whose transcript is gone are a row in the panel,
// hidden by default.
func TestGoneSessionsSetting(t *testing.T) {
	t.Run("when the config is the default", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		cfg, err := config.New(config.NewViper())
		require.NoError(t, err)
		var row ui.Setting
		for _, s := range settingsFor(cfg).Items {
			if s.Key == config.KeyGoneSessions {
				row = s
			}
		}

		t.Run("it should show a deleted transcripts row reading hidden", func(t *testing.T) {
			assert.Equal(t, [2]string{"deleted transcripts", "hidden"}, [2]string{row.Label, row.Value})
		})

		t.Run("it should offer hidden and shown", func(t *testing.T) {
			assert.Equal(t, []string{"hidden", "shown"}, row.Options)
		})
	})

	for _, c := range []struct {
		value string
		want  bool
	}{{"shown", true}, {"hidden", false}} {
		t.Run("when the panel sets it to "+c.value, func(t *testing.T) {
			key, value := settingValue(ui.Setting{Key: config.KeyGoneSessions, Value: c.value})

			t.Run(fmt.Sprintf("it should save gone_sessions = %v", c.want), func(t *testing.T) {
				assert.Equal(t, []any{config.KeyGoneSessions, c.want}, []any{key, value})
			})
		})
	}
}
