package script

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	for name, cue := range map[string]Cue{
		"a key":               {At: 1, Key: "Tab"},
		"a held arrow":        {At: 1, Hold: "ArrowLeft", For: 0.5},
		"a point":             {At: 1, Point: []float64{10, 20}},
		"a target":            {At: 1, PointAt: "plant"},
		"a pan":               {At: 1, PanTo: "district:kiln", Over: 2},
		"a click":             {At: 1, Click: true},
		"a turn of wheel":     {At: 1, Wheel: 2, Over: 1},
		"a statistics prompt": {At: 1, ShowStatsPrompt: "off"},
		"a store notice":      {At: 1, ShowStoreNotice: "damaged"},
	} {
		t.Run("when a cue is "+name, func(t *testing.T) {
			t.Run("it should pass", func(t *testing.T) {
				assert.NoError(t, Validate([]Cue{cue}))
			})
		})
	}

	for name, cue := range map[string]Cue{
		"empty":                  {At: 1},
		"two things at once":     {At: 1, Key: "Tab", Click: true},
		"a point that is not xy": {At: 1, Point: []float64{10}},
		"a hold with no length":  {At: 1, Hold: "ArrowLeft"},
		"before the clip":        {At: -1, Click: true},
		"a statistics prompt that is neither off nor on":   {At: 1, ShowStatsPrompt: "maybe"},
		"a store notice that is neither newer nor damaged": {At: 1, ShowStoreNotice: "broken"},
		"a store notice and a click at once":               {At: 1, ShowStoreNotice: "newer", Click: true},
	} {
		t.Run("when a cue is "+name, func(t *testing.T) {
			t.Run("it should refuse it", func(t *testing.T) {
				assert.Error(t, Validate([]Cue{cue}))
			})
		})
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "script.json")
	cues := []Cue{{At: 2, Click: true}, {At: 0.5, PanTo: "plant", Over: 1}}
	require.NoError(t, Save(path, cues))
	got, err := Load(path)
	require.NoError(t, err)

	t.Run("when a script is saved and loaded", func(t *testing.T) {
		t.Run("it should come back as it went", func(t *testing.T) {
			assert.Equal(t, cues, got)
		})

		t.Run("it should play in time order", func(t *testing.T) {
			assert.Equal(t, 0.5, Sorted(got)[0].At)
		})
	})
}
