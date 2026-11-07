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

	// Item 361: stations are not drawn; the line is the group's.
	t.Run("when it is a group", func(t *testing.T) {
		t.Run("it should say what a group is", func(t *testing.T) {
			assert.Contains(t, city.KindGroup.About(), "A group is the sessions working on one thing, however many projects they cross, drawn as a line between their districts by default.")
		})

		t.Run("it should not mention a station", func(t *testing.T) {
			assert.NotContains(t, city.KindGroup.Is()+city.KindGroup.About(), "station")
		})

		t.Run("it should list the kerb as related", func(t *testing.T) {
			assert.Contains(t, city.KindGroup.Related(), city.KindKerb)
		})
	})

	t.Run("when it is a kerb", func(t *testing.T) {
		t.Run("it should say it runs round a group's buildings", func(t *testing.T) {
			assert.Contains(t, city.KindKerb.About(), "runs round a group's buildings")
		})
	})

	for _, k := range []city.Kind{city.KindProject, city.KindHall} {
		t.Run("when it is "+k.Name(), func(t *testing.T) {
			t.Run("it should name the group as related", func(t *testing.T) {
				assert.Contains(t, k.Related(), city.KindGroup)
			})
		})
	}

	// Item 255: a group's line carries nothing.
	t.Run("when it is a van", func(t *testing.T) {
		t.Run("it should not say a carriage runs a group's line", func(t *testing.T) {
			assert.NotContains(t, city.KindVan.About(), "carriage")
		})
	})

	t.Run("for every hit the map can return", func(t *testing.T) {
		a := session("a", cinders, state.Working)
		a.PRs = []claude.PR{{Number: 1, Repository: "o/r"}}
		a.Group = "g"
		mate := session("b", botropolis, state.Working)
		mate.Group = "g"
		s := city.NewScene(city.NewLayout())
		s.SetClock(func() time.Time { return now }, time.UTC)
		s.Resize(800, 600)
		s.SetSnapshot(state.Snapshot{At: now, Sessions: []state.Session{a, mate},
			Groups: []state.Group{{ID: "g", Name: "G", Members: []string{"a", "b"}}}})
		c := s.City()
		b := buildingOf(t, s, "a")
		require.NotEmpty(t, c.Lines)
		hits := map[city.Kind]city.Hit{
			city.KindSession:  {Building: b, District: c.DistrictOf(b)},
			city.KindProject:  {District: c.DistrictOf(b)},
			city.KindPlant:    {Landmark: city.LandmarkPlant},
			city.KindLibrary:  {Landmark: city.LandmarkLibrary},
			city.KindHall:     {Landmark: city.LandmarkHall},
			city.KindClinic:   {Landmark: city.LandmarkClinic},
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
			city.KindGroup:    {Station: &city.StationHit{Line: &c.Lines[0]}},
			// Item 89.
			city.KindInspector: {Building: b, Inspector: b},
			city.KindDelivery:  {Delivery: &city.Delivery{PR: a.PRs[0]}},
			city.KindVan:       {Traffic: &city.Traffic{Cargo: city.CargoMessage}},
			// Item 255.
			city.KindBoxTruck:  {Traffic: &city.Traffic{Cargo: city.CargoFiles}},
			city.KindMotorbike: {Traffic: &city.Traffic{Cargo: city.CargoVisit}},
		}

		// Item 361: a kerb cannot be pointed at.
		unpointable := []city.Kind{city.KindKerb}

		t.Run("it should have a hit for every kind that can be pointed at", func(t *testing.T) {
			assert.Len(t, hits, len(city.Kinds)-len(unpointable))
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
