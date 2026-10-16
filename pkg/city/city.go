package city

import (
	"fmt"
	"hash/fnv"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/plan"
	"botropolis.city/botropolis/pkg/state"
)

// World units are pixels at zoom 1; every size is a whole number of 16 px
// tiles so sprites land on tile boundaries at integer zooms.
const (
	Tile               = 16.0
	BuildingSize       = 3 * Tile
	BuildingGap        = Tile
	ParkedSize         = 2 * Tile
	ParkedGap          = Tile / 2
	YardGap            = Tile
	DistrictPadding    = 2 * Tile
	DistrictGap        = 4 * Tile
	districtColumns    = 3
	minBuildingColumns = 3
	minYardColumns     = 3
	worktreeSegment    = ".claude/worktrees"
)

type Building struct {
	Session   state.Session
	slot      int
	Rect      Rect
	Fill      float64
	Cranes    int
	Flags     int
	Merged    int
	Smoke     int
	Lit       bool
	Pulse     bool
	BoardedUp bool
	// Vacant is a plot with no building: a session nothing has been typed
	// into yet.
	Vacant bool
	// Hue is a parked session's container colour, one per project, an
	// index into ContainerHues.
	Hue int
	// Group is the group the session belongs to, if any (item 87).
	Group *state.Group
}

// ContainerHues names the container colours the storage yard has, in
// the order the kit's pieces come; a project keeps one for good.
var ContainerHues = []string{"red", "blue", "green"}

// projectHue picks a project's container colour from its root.
func projectHue(root string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(root))
	return int(h.Sum32() % uint32(len(ContainerHues)))
}

type District struct {
	Name      string
	Root      string
	columns   int
	size      Point
	Rect      Rect
	Buildings []*Building
	roads     []roadNote
	// Storage marks the one district that holds every parked session,
	// grouped by project.
	Storage bool
	Groups  []StorageGroup
	at      time.Time
}

type roadNote struct {
	other    string
	messages int
	out      bool
	files    int
}

type City struct {
	Districts   []*District
	Plant       Plant
	Towers      []*Tower
	Library     Library
	Hall        Hall
	Roads       []RoadLine
	Streets []Street
	// Corridors are the plan's whole avenue grid, and StreetCells the
	// road laid on it: only the cells a connection's route crosses, each
	// joined to the road beside it. A corridor without road is belt.
	Corridors   []StreetCell
	StreetCells []StreetCell
	RiverCells  []RiverCell
	Night       bool
	Time        time.Time

	// From the plan: the plaza and its fountain, every park block and
	// belt strip, the trees and lamps, and the map's edge.
	Plaza    Rect
	Fountain Rect
	Camps    []*Camp
	Parks    []Park
	Trees    []Tree
	// Rails is the freight loop as a closed polyline; Trains run it.
	Rails  []Point
	Trains []*Train
	Lamps  []Cell
	bounds Rect
	plan   plan.Plan
}

type Hit struct {
	District *District
	Building *Building
	Landmark Landmark
	Tower    *Tower
	Road     *RoadLine
	Beam     *Beam
	Line     *PowerLine
	Park     *Park
	Train    *Train
	Voyage   *Voyage
	// Courier is the refresh boat. It carries what pressing the refresh
	// key does, because Aria had not realised the key existed. Item 56.
	Courier *Courier
	// The movers. These five were the only objects on the map that could
	// not be asked what they meant, and three of them are vehicles, which
	// is most of why they blurred together. See movers.go.
	Worker   *Building
	Subagent *Building
	Smoke    *Building
	// Gauge is a usage boat on the river. It carries its reading rather
	// than pointing at something else, because the reading is a moment
	// in time and the boat is the only place it lives. Item 49.
	Gauge *GaugeHit
	Car   *CarHit
	Flag  *FlagHit
}

// CarHit is one car and the road it is driving, so it can name the pair
// of repos it stands for — which is the thing a car on a routed street
// cannot say by where it is.
type CarHit struct {
	Road *RoadLine
	From string
	To   string
}

// FlagHit is one flag on a roof and the pull request it stands for.
type FlagHit struct {
	Building *Building
	PR       claude.PR
}

// Near is the road, beam or power line within tolerance of p, nearest
// first, or nothing. A road is hovered along the avenues its traffic
// takes; only a road with no route falls back to its straight line.
func (c *City) Near(p Point, tolerance float64) Hit {
	best := tolerance
	var hit Hit
	routed := map[*RoadLine]bool{}
	for i := range c.Streets {
		street := &c.Streets[i]
		routed[street.Road] = true
		for k := 1; k < len(street.Path); k++ {
			if d := p.DistanceToSegment(street.Path[k-1], street.Path[k]); d < best {
				best, hit = d, Hit{Road: street.Road}
			}
		}
	}
	for i := range c.Roads {
		if routed[&c.Roads[i]] {
			continue
		}
		if d := p.DistanceToSegment(c.Roads[i].A, c.Roads[i].B); d < best {
			best, hit = d, Hit{Road: &c.Roads[i]}
		}
	}
	beams := c.Beams()
	for i := range beams {
		if d := p.DistanceToSegment(beams[i].From, beams[i].To); d < best {
			best, hit = d, Hit{Beam: &beams[i]}
		}
	}
	lines := c.PowerLines()
	for i := range lines {
		if d := p.DistanceToSegment(lines[i].From, lines[i].To); d < best {
			best, hit = d, Hit{Line: &lines[i]}
		}
	}
	return hit
}

type Card struct {
	Title   string
	Lines   []string
	Actions []string
	// Para, when set, is prose wrapped under the lines.
	Para string
	// Series, when set, is drawn as a sparkline under the lines.
	Series []float64
}

func ProjectRoot(cwd string) string {
	if cwd == "" {
		return ""
	}
	clean := filepath.Clean(cwd)
	if i := strings.Index(clean, "/"+worktreeSegment+"/"); i >= 0 {
		return clean[:i]
	}
	return clean
}

func Build(snapshot state.Snapshot, layout *Layout) *City {
	byRoot := map[string][]state.Session{}
	parked := map[string][]state.Session{}
	for _, s := range snapshot.Sessions {
		root := ProjectRoot(s.CWD)
		if layout.IsHidden(root) {
			continue
		}
		if s.State == state.Parked {
			parked[root] = append(parked[root], s)
			continue
		}
		byRoot[root] = append(byRoot[root], s)
	}
	roots := make([]string, 0, len(byRoot))
	for root := range byRoot {
		roots = append(roots, root)
	}
	sort.Slice(roots, func(i, j int) bool {
		return baseName(roots[i]) < baseName(roots[j])
	})

	city := &City{Time: snapshot.At}
	if len(byRoot)+len(parked) == 0 {
		city.placeLandmarks(snapshot)
		return city
	}
	for _, root := range roots {
		city.Districts = append(city.Districts, buildDistrict(root, byRoot[root], parked[root], layout))
	}
	in := planInput(city.Districts, len(snapshot.Servers))
	p := plan.Make(in, layout.memory())
	storage := buildStorage(parked, float64(p.SpanCols())*CellSize)
	if storage != nil {
		in.StorageRows = cellsNeeded(storage.size.Y)
		p = plan.Make(in, layout.memory())
	}
	city.plan = p
	city.bounds = cellRect(p.Bounds)
	for _, d := range city.Districts {
		d.placeOn(cellRect(p.Blocks[d.Root]))
		d.at = snapshot.At
	}
	if storage != nil {
		storage.placeOn(cellRect(p.Storage))
		city.Districts = append(city.Districts, storage)
	}
	for _, park := range append(append([]plan.Block(nil), p.Parks...), p.Belt...) {
		city.Parks = append(city.Parks, Park{Rect: cellRect(park)})
	}
	for _, t := range p.Trees {
		cell := toCell(t.Cell)
		centre := cell.Center()
		city.Trees = append(city.Trees, Tree{Cell: cell, Kind: t.Kind, Variant: t.Variant, At: Point{X: centre.X + t.DX*CellSize, Y: centre.Y + t.DY*CellSize}})
	}
	city.Rails = railPath(p.Rails)
	for _, l := range p.Lamps {
		city.Lamps = append(city.Lamps, toCell(l))
	}
	city.Plaza = cellRect(p.Plaza)
	city.Fountain = cellAt(p.Fountain)
	city.placeLandmarks(snapshot)
	city.placeRoads(snapshot.Roads)
	city.placeRiver()
	city.placeStreets()
	city.placeCamps(snapshot.Teams)
	city.placeGroups(snapshot.Groups)
	city.Trains = trains(city.Breakdown(LastDay))
	return city
}

func baseName(root string) string {
	return filepath.Base(root)
}

func buildDistrict(root string, live, parked []state.Session, layout *Layout) *District {
	district := &District{Name: baseName(root), Root: root}
	sort.Slice(live, func(i, j int) bool {
		if !live[i].StartedAt.Equal(live[j].StartedAt) {
			return live[i].StartedAt.Before(live[j].StartedAt)
		}
		return live[i].ID < live[j].ID
	})
	slots := layout.Slots(root, live, parked)
	district.columns = buildingColumns(maxOf(slots) + 1)
	used := min(maxOf(slots)+1, district.columns)
	rows := math.Ceil(float64(maxOf(slots)+1) / float64(district.columns))
	district.size = Point{
		X: 2*DistrictPadding + pitch(used, BuildingSize, BuildingGap),
		Y: 2*DistrictPadding + pitch(int(rows), BuildingSize, BuildingGap),
	}
	for _, s := range live {
		column, row := slots[s.ID]%district.columns, slots[s.ID]/district.columns
		at := Point{X: DistrictPadding + float64(column)*(BuildingSize+BuildingGap), Y: DistrictPadding + float64(row)*(BuildingSize+BuildingGap)}
		district.Buildings = append(district.Buildings, newBuilding(s, slots[s.ID], at, BuildingSize))
	}
	sort.Slice(district.Buildings, func(i, j int) bool {
		return district.Buildings[i].slot < district.Buildings[j].slot
	})
	return district
}

// pitch is the span of n cells of the given size with gaps between them.
func pitch(n int, size, gap float64) float64 {
	if n <= 0 {
		return 0
	}
	return float64(n)*size + float64(n-1)*gap
}

func maxOf(slots map[string]int) int {
	highest := -1
	for _, slot := range slots {
		if slot > highest {
			highest = slot
		}
	}
	return highest
}

func buildingColumns(slots int) int {
	columns := int(math.Ceil(math.Sqrt(float64(slots))))
	if columns < minBuildingColumns {
		columns = minBuildingColumns
	}
	return columns
}

func mergedPRs(s state.Session) int {
	n := 0
	for _, pr := range s.PRs {
		if pr.Merged() {
			n++
		}
	}
	return n
}

func newBuilding(s state.Session, slot int, at Point, size float64) *Building {
	return &Building{
		Session:   s,
		slot:      slot,
		Rect:      RectAt(at.X, at.Y, size, size),
		Fill:      math.Min(1, s.ContextPercent/100),
		Cranes:    s.SubagentsInFlight,
		Flags:     len(s.PRs),
		Merged:    mergedPRs(s),
		Smoke:     s.APIErrors,
		Lit:       state.Live(s.State),
		Pulse:     s.State == state.NeedsYou,
		BoardedUp: s.State == state.Parked,
		Vacant:    s.State == state.Empty,
		Hue:       projectHue(ProjectRoot(s.CWD)),
	}
}

// TowerLabelWidth is the screen room the longest tower name needs to the
// left of the tower column.
func (c *City) TowerLabelWidth() float64 {
	longest := 0
	for _, t := range c.Towers {
		longest = max(longest, len(t.Server.Name))
	}
	return float64(longest)*LabelCharWidth + TowerLabelMargin
}

func (c *City) Buildings() []*Building {
	var buildings []*Building
	for _, d := range c.Districts {
		buildings = append(buildings, d.Buildings...)
	}
	return buildings
}

// Bounds is the plan's edge: belt to belt, river included.
func (c *City) Bounds() Rect {
	return c.bounds
}

// Extent is what a fit should frame: the whole plan.
func (c *City) Extent() Rect {
	return c.bounds
}

// Plan is the plan the city was laid out on.
func (c *City) Plan() plan.Plan {
	return c.plan
}

func (c *City) At(p Point) Hit {
	if c.Plant.Rect.Area() > 0 && c.Plant.Rect.Contains(p) {
		return Hit{Landmark: LandmarkPlant}
	}
	for _, t := range c.Towers {
		if t.Rect.Contains(p) {
			return Hit{Landmark: LandmarkTower, Tower: t}
		}
	}
	if c.Library.Rect.Area() > 0 && c.Library.Rect.Contains(p) {
		return Hit{Landmark: LandmarkLibrary}
	}
	if c.Hall.Rect.Area() > 0 && c.Hall.Rect.Contains(p) {
		return Hit{Landmark: LandmarkHall}
	}
	if c.Fountain.Area() > 0 && c.Fountain.Contains(p) {
		return Hit{Landmark: LandmarkFountain}
	}
	if _, ok := c.River(CellOf(p)); ok {
		return Hit{Landmark: LandmarkWater}
	}
	for _, d := range c.Districts {
		if !d.Rect.Contains(p) {
			continue
		}
		hit := Hit{District: d}
		for _, b := range d.Buildings {
			if b.Rect.Contains(p) {
				hit.Building = b
				break
			}
		}
		return hit
	}
	for i := range c.Parks {
		if c.Parks[i].Rect.Contains(p) {
			return Hit{Landmark: LandmarkPark, Park: &c.Parks[i]}
		}
	}
	if c.Plaza.Area() > 0 && c.Plaza.Contains(p) {
		return Hit{Landmark: LandmarkPlaza}
	}
	return Hit{}
}

func (b *Building) Card(now time.Time) Card {
	s := b.Session
	title := s.Title
	if title == "" {
		title = state.ShortID(s.ID)
	}
	lines := []string{
		"state    " + string(s.State),
	}
	if g := b.Group; g != nil {
		lines = append(lines, "group    "+g.Name+", "+string(g.Move)+": "+g.Why)
		if s.GroupJoined != "" {
			lines = append(lines, "joined   "+s.GroupJoined)
		}
	}
	if b.Vacant {
		lines = append(lines, "empty    nothing typed yet; prune clears it after an hour")
	}
	lines = append(lines, "branch   "+s.Branch, "model    "+s.Model)
	if doing := s.Doing(); doing != "" {
		lines = append(lines, "doing    "+doing)
	}
	context := fmt.Sprintf("context  %s of %s (%s tokens)", format.Percent(s.ContextPercent), format.Tokens(float64(s.ContextWindow)), format.Tokens(float64(s.ContextTokens)))
	if s.Compactions > 0 {
		context += fmt.Sprintf(", compacted %dx, last %s", s.Compactions, s.LastCompactionAt.Local().Format("15:04"))
	}
	lines = append(lines, context,
		fmt.Sprintf("tokens   %s/h fresh, %s/h cached", format.Tokens(s.FreshTokensPerHour), format.Tokens(s.CacheReadPerHour)))
	subs := fmt.Sprintf("subs     %d of %d in flight", s.SubagentsInFlight, s.Subagents)
	if len(s.SubagentNames) > 0 {
		subs += ": " + strings.Join(s.SubagentNames, ", ")
	}
	lines = append(lines, subs)
	if len(s.PRs) > 0 {
		parts := make([]string, 0, len(s.PRs))
		for _, pr := range s.PRs {
			part := fmt.Sprintf("#%d %s", pr.Number, pr.Repository)
			if pr.State != "" && pr.State != claude.PROpen {
				part += " (" + pr.State + ")"
			}
			parts = append(parts, part)
		}
		lines = append(lines, "prs      "+strings.Join(parts, ", "))
	}
	if s.APIErrors > 0 {
		lines = append(lines, fmt.Sprintf("errors   %d api, last %s", s.APIErrors, s.LastErrorAt.Local().Format("15:04")))
	}
	if s.Note != "" {
		lines = append(lines, "note     "+s.Note)
	}
	if s.Team != "" {
		role := "lead"
		if s.Agent != "" {
			role = s.Agent
		}
		lines = append(lines, "team     "+s.Team+" ("+role+")")
	}
	telemetry := "telemetry hooks"
	if !s.Hooked {
		telemetry = "telemetry files only; no hook events seen (no wire)"
	}
	lines = append(lines, telemetry,
		"age      "+format.Age(now.Sub(s.StartedAt))+", idle "+format.Age(now.Sub(s.LastActivity)))
	return Card{Title: title, Lines: lines, Series: b.Series(LastDay, now)}
}

// DistrictOf is the district a building stands in.
func (c *City) DistrictOf(b *Building) *District {
	for _, d := range c.Districts {
		for _, other := range d.Buildings {
			if other == b {
				return d
			}
		}
	}
	return nil
}

// Busy reports whether anything in the district is awake: the rule for
// showing its name plate without a hover.
func (d *District) Busy() bool {
	for _, b := range d.Buildings {
		if b.Lit {
			return true
		}
	}
	return false
}

func (d *District) Card() Card {
	if d.Storage {
		return d.storageCard()
	}
	var fresh, cached float64
	for _, b := range d.Buildings {
		fresh += b.Session.FreshTokensPerHour
		cached += b.Session.CacheReadPerHour
	}
	var prs int
	var active time.Duration
	for _, b := range d.Buildings {
		prs += len(b.Session.PRs)
		active += b.Session.LastActivity.Sub(b.Session.StartedAt)
	}
	lines := []string{
		fmt.Sprintf("sessions %d", len(d.Buildings)),
		fmt.Sprintf("active   %s across them", format.Age(active)),
		fmt.Sprintf("tokens   %s/h fresh, %s/h cached", format.Tokens(fresh), format.Tokens(cached)),
		fmt.Sprintf("prs      %d", prs),
		"path     " + d.Root,
	}
	if len(d.roads) > 0 {
		parts := make([]string, 0, len(d.roads))
		for _, r := range d.roads {
			direction := "to"
			if !r.out {
				direction = "from"
			}
			parts = append(parts, fmt.Sprintf("%s %s %s", roadTraffic(r.messages, r.files), direction, r.other))
		}
		lines = append(lines, "roads    "+strings.Join(parts, ", "))
	}
	return Card{Title: d.Name, Lines: lines, Series: d.Series(LastDay, d.at)}
}
