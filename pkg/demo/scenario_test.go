package demo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func TestScenarioValidate(t *testing.T) {
	placed := []Placement{{Project: "beacon", State: "empty"}}

	t.Run("when a scenario places sessions and names each shot once", func(t *testing.T) {
		t.Run("it should pass", func(t *testing.T) {
			assert.NoError(t, Scenario{Sessions: placed, Shots: []Shot{{Name: "a"}, {Name: "b"}}}.Validate())
		})
	})

	for name, s := range map[string]Scenario{
		"places no sessions":      {Shots: []Shot{{Name: "a"}}},
		"has a shot with no name": {Sessions: placed, Shots: []Shot{{}}},
		"names two shots alike":   {Sessions: placed, Shots: []Shot{{Name: "a"}, {Name: "a"}}},
		"sets a pace below zero":  {Sessions: placed, Pace: Duration(-time.Second)},
	} {
		t.Run("when a scenario "+name, func(t *testing.T) {
			t.Run("it should refuse it, rather than film an empty city or overwrite a file", func(t *testing.T) {
				assert.Error(t, s.Validate())
			})
		})
	}
}

func TestDurationYAML(t *testing.T) {
	for raw, want := range map[string]time.Duration{
		"90s": 90 * time.Second, "3h": 3 * time.Hour, "2d": 48 * time.Hour, "1d6h": 30 * time.Hour,
	} {
		t.Run("when a scenario says "+raw, func(t *testing.T) {
			var d struct {
				Ago Duration `yaml:"ago"`
			}
			require.NoError(t, yaml.Unmarshal([]byte("ago: "+raw), &d))

			t.Run("it should read it as that long", func(t *testing.T) {
				assert.Equal(t, want, time.Duration(d.Ago))
			})
		})
	}
}

func TestScenarioYAML(t *testing.T) {
	var s Scenario
	require.NoError(t, yaml.Unmarshal([]byte(`
sessions:
  - {ref: tidepool/what-is-phase, state: working, ago: 1m, started: 3h}
groups:
  - [Driftwood ingest, Saltmarsh API]
pace: 30s
`), &s))

	t.Run("when a scenario sets pace", func(t *testing.T) {
		t.Run("it should parse it as a duration", func(t *testing.T) {
			assert.Equal(t, 30*time.Second, time.Duration(s.Pace))
		})
	})

	t.Run("when a placement gives started", func(t *testing.T) {
		t.Run("it should parse it as a duration", func(t *testing.T) {
			assert.Equal(t, 3*time.Hour, time.Duration(s.Sessions[0].Started))
		})
	})

	t.Run("when a scenario lists the groups it expects", func(t *testing.T) {
		t.Run("it should read each group as its members' titles", func(t *testing.T) {
			assert.Equal(t, [][]string{{"Driftwood ingest", "Saltmarsh API"}}, s.Groups)
		})
	})
}
