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
		// Item 76: a poster is chosen from a clip's own frames.
		"sets poster_at on a still": {Sessions: placed, Shots: []Shot{{Name: "a", PosterAt: ptr(1.0)}}},
		"sets poster_at past the clip's end": {Sessions: placed,
			Shots: []Shot{{Name: "a", Record: &Recording{Seconds: 4}, PosterAt: ptr(4.5)}}},
		"loops a still": {Sessions: placed, Shots: []Shot{{Name: "a", Loop: &Loop{Crossfade: 1}}}},
		"crossfades half its loop or more": {Sessions: placed,
			Shots: []Shot{{Name: "a", Record: &Recording{Seconds: 4}, Loop: &Loop{Crossfade: 2}}}},
		"loops with no crossfade": {Sessions: placed,
			Shots: []Shot{{Name: "a", Record: &Recording{Seconds: 4}, Loop: &Loop{}}}},
		"budgets a still": {Sessions: placed, Shots: []Shot{{Name: "a", MaxMB: 3}}},
		"budgets a clip without listing its formats": {Sessions: placed,
			Shots: []Shot{{Name: "a", Record: &Recording{Seconds: 4}, MaxMB: 3}}},
		"budgets a GIF": {Sessions: placed,
			Shots: []Shot{{Name: "a", Record: &Recording{Seconds: 4}, MaxMB: 3, Formats: []string{"mp4", "gif"}}}},
		"crops a still": {Sessions: placed, Shots: []Shot{{Name: "a", Crops: []string{"4:5"}}}},
		"crops to something that is not W:H": {Sessions: placed,
			Shots: []Shot{{Name: "a", Record: &Recording{Seconds: 4}, Formats: []string{"mp4"}, Crops: []string{"tall"}}}},
		"crops wider than its window": {Sessions: placed,
			Shots: []Shot{{Name: "a", Record: &Recording{Seconds: 4}, Formats: []string{"mp4"}, Crops: []string{"21:9"}}}},
		"crops a GIF": {Sessions: placed,
			Shots: []Shot{{Name: "a", Record: &Recording{Seconds: 4}, Crops: []string{"4:5"}}}},
		"sets poster_at below zero": {Sessions: placed,
			Shots: []Shot{{Name: "a", Record: &Recording{Seconds: 4}, PosterAt: ptr(-1.0)}}},
	} {
		t.Run("when a scenario "+name, func(t *testing.T) {
			t.Run("it should refuse it, rather than film an empty city or overwrite a file", func(t *testing.T) {
				assert.Error(t, s.Validate())
			})
		})
	}
}

func TestPosterAtYAML(t *testing.T) {
	var shot Shot
	require.NoError(t, yaml.Unmarshal([]byte("{name: day, record: {seconds: 54}, poster_at: 12.5}"), &shot))

	t.Run("when a shot sets poster_at", func(t *testing.T) {
		t.Run("it should read it in seconds of video", func(t *testing.T) {
			assert.Equal(t, ptr(12.5), shot.PosterAt)
		})
	})
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
