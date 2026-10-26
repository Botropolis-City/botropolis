package city_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

func TestKinds(t *testing.T) {
	t.Run("for every kind", func(t *testing.T) {
		for _, k := range city.Kinds {
			t.Run("when it is "+k.Name(), func(t *testing.T) {
				t.Run("it should have a name and a one-line description", func(t *testing.T) {
					assert.True(t, k.Name() != "" && k.Is() != "" && !strings.Contains(k.Is(), "\n"))
				})
			})
		}
	})

	t.Run("when it is city hall", func(t *testing.T) {
		t.Run("it should say how to make a group", func(t *testing.T) {
			assert.Contains(t, city.KindHall.About(), "Move to group on a session's card, then new group…, makes a group.")
		})
	})

	t.Run("for every hit the map can return", func(t *testing.T) {
		a := session("a", cinders, state.Working)
		a.PRs = []claude.PR{{Number: 1, Repository: "o/r"}}
		a.Group = "g"
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(state.Snapshot{At: now, Sessions: []state.Session{a, session("b", botropolis, state.Working)},
			Groups: []state.Group{{ID: "g", Name: "G", Members: []string{"a"}}}})
		c := s.City()
		b := buildingOf(t, s, "a")
		require.NotEmpty(t, c.Lines)
		hits := map[city.Kind]city.Hit{
			city.KindSession:  {Building: b, District: c.DistrictOf(b)},
			city.KindProject:  {District: c.DistrictOf(b)},
			city.KindPlant:    {Landmark: city.LandmarkPlant},
			city.KindLibrary:  {Landmark: city.LandmarkLibrary},
			city.KindHall:     {Landmark: city.LandmarkHall},
			city.KindTower:    {Landmark: city.LandmarkTower, Tower: &city.Tower{}},
			city.KindRiver:    {Landmark: city.LandmarkWater},
			city.KindFountain: {Landmark: city.LandmarkFountain},
			city.KindPlaza:    {Landmark: city.LandmarkPlaza},
			city.KindRoad:     {Road: &city.RoadLine{From: c.DistrictOf(b), To: c.DistrictOf(b)}},
			city.KindBeam:     {Beam: &city.Beam{Building: b, Tower: &city.Tower{}}},
			city.KindWire:     {Line: &city.PowerLine{Building: b}},
			city.KindTrain:    {Train: &city.Train{}},
			city.KindTug:      {Voyage: &city.Voyage{}},
			city.KindLaunch:   {Courier: &city.Courier{}},
			city.KindDrone:    {Subagent: b},
			city.KindSmoke:    {Smoke: b},
			city.KindGauge:    {Gauge: &city.GaugeHit{}},
			city.KindCar:      {Car: &city.CarHit{Road: &city.RoadLine{}}},
			city.KindFlag:     {Flag: &city.FlagHit{PR: a.PRs[0]}},
			city.KindStation:  {Station: &city.StationHit{Line: &c.Lines[0]}},
			// Item 89.
			city.KindInspector: {Building: b, Inspector: b},
			city.KindDelivery:  {Delivery: &city.Delivery{PR: a.PRs[0]}},
			city.KindVan:       {Message: &city.Message{}},
		}

		t.Run("it should have a hit for every kind", func(t *testing.T) {
			assert.Len(t, hits, len(city.Kinds))
		})

		for k, hit := range hits {
			t.Run("when it is "+k.Name(), func(t *testing.T) {
				t.Run("it should name the kind of what is under it", func(t *testing.T) {
					assert.Equal(t, k, city.KindOf(hit))
				})

				t.Run("its card should start with that kind's line", func(t *testing.T) {
					card, ok := s.CardFor(hit)
					require.True(t, ok)
					assert.True(t, strings.HasPrefix(card.Kind, k.Name()), card.Kind)
				})
			})
		}
	})
}
