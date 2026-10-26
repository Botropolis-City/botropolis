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
	"botropolis.city/botropolis/pkg/stats"
)

// World units are pixels at zoom 1; every size is a whole number of 16 px
// tiles so sprites land on tile boundaries at integer zooms.
const (
	Tile               = 16.0
	BuildingSize       = 3 * Tile
	BuildingGap        = Tile
	DistrictPadding    = 2 * Tile
	DistrictGap        = 4 * Tile
	districtColumns    = 3
	minBuildingColumns = 3
	worktreeSegment    = ".claude/worktrees"
)

type Building struct {
	Session state.Session
	// Cost is the session's cost from the statistics store, and Priced
	// whether the store had one (item 116).
	Cost   stats.Cost
	Priced bool
	slot   int
	Rect   Rect
	Fill   float64
	Cranes int
	Flags  int
	Merged int
	Smoke  int
	Lit    bool
	Pulse  bool
	// Rung is how far the session has gone quiet, and Blinds, BoardedUp
	// and Overgrown are what that rung draws (item 86).
	Rung      state.Rung
	Blinds    bool
	BoardedUp bool
	Overgrown bool
	// Vacant is a plot with no building: a session nothing has been typed
	// into yet.
	Vacant bool
	// Group is the group the session belongs to, if any (item 87).
	Group *state.Group
	// Station is where the building's project stands on its group's
	// line, counting from 1, of Stations.
	Station, Stations int
	// Packed is a crate in the depot; Hue is its district's colour, an
	// index into ContainerHues (item 86).
	Packed bool
	Hue    int
	// Type is its district's type; empty is residential (item 109).
	Type DistrictType
	// Where is its district's name, told apart from any twin (item 139).
	Where string
}

// ContainerHues names the container colours a project's cargo comes in,
// in the order the kit's pieces come; a project keeps one for good.
var ContainerHues = []string{"red", "blue", "green"}

// projectHue picks a project's container colour from its root.
func projectHue(root string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(root))
	return int(h.Sum32() % uint32(len(ContainerHues)))
}

type District struct {
	Name string
	Root string
	// Type is what kind of buildings the district grows (item 109).
	Type      DistrictType
	columns   int
	size      Point
	Rect      Rect
	Buildings []*Building
	roads     []roadNote
	at        time.Time
	// Depot marks the one district that keeps every packed-up session,
	// a row per district (item 86).
	Depot bool
	// Rung is the freshest rung among the district's sessions: its whole
	// lot ages only when every session on it has (item 86).
	Rung state.Rung
	Rows []DepotRow
}

type roadNote struct {
	other    string
	messages int
	out      bool
	files    int
}

type City struct {
	Districts []*District
	Plant     Plant
	Towers    []*Tower
	Library   Library
	Hall      Hall
	Roads     []RoadLine
	Streets   []Street
	// Lines are the groups drawn across districts, and groups the
	// snapshot's groups they are drawn from (item 87).
	Lines  []Line
	groups []*state.Group
	// names is every known root's district name (item 139).
	names map[string]string
	// folderless is the sessions with no known folder: on no district,
	// still counted in the statistics (item 139).
	folderless []state.Session
	// offMap is the sessions Shown leaves off the map: still counted in
	// the statistics (item 273).
	offMap []state.Session
	// memo is what the scene remembers between snapshots, or nil for a
	// city built from nothing (item 197).
	memo *memo
	// planID is which of the memo's plans the city stands on (item 206).
	planID int
	// buildings is every district's buildings, and byID each by its
	// session, once the districts are final (item 270).
	buildings []*Building
	byID      map[string]*Building
	// landings are the street cells the bridges land on, the first
	// bridge first (items 218 and 220).
	landings []landing
	// bridgeRoads are the road cells laid only to reach a bridge's
	// landing (item 247).
	bridgeRoads []Cell
	// Corridors are the plan's whole avenue grid, and StreetCells the
	// road laid on it: the cells a connection's route crosses and the
	// way to each bridge, each joined to the road beside it. A corridor
	// without road is belt.
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
	// ledger is the statistics store the plant, the strip and the
	// trains read from, nil when statistics are off (item 116).
	ledger Ledger
	Lamps  []Cell
	bounds Rect
	plan   plan.Plan
	// heading is the camera's, which decides each lot's driveway; the
	// plan's own lamps and trees are kept whole so the streets can be
	// placed again when it turns. Item 108.
	heading   int
	driveways map[*District]Driveway
	// layout keeps each lot's driveway once it is chosen (item 108).
	layout    *Layout
	planLamps []Cell
	planTrees []Tree
}

type Hit struct {
	District *District
	Building *Building
	Landmark Landmark
	Tower    *Tower
	Road     *RoadLine
	Beam     *Beam
	Line     *PowerLine
	Train    *Train
	Voyage   *Voyage
	// Courier is the refresh boat. It carries what pressing the refresh
	// key does, because Aria had not realised the key existed. Item 56.
	Courier *Courier
	// The movers. These five were the only objects on the map that could
	// not be asked what they meant, and three of them are vehicles, which
	// is most of why they blurred together. See movers.go.
	Subagent *Building
	Smoke    *Building
	// Gauge is a usage boat on the river. It carries its reading rather
	// than pointing at something else, because the reading is a moment
	// in time and the boat is the only place it lives. Item 49.
	Gauge *GaugeHit
	Car   *CarHit
	Flag  *FlagHit
	// Inspector is the car parked outside while a session has a pull
	// request open, and Delivery a merged one's truck (item 89).
	Inspector *Building
	Delivery  *Delivery
	// Message is a van carrying one message between sessions (item 89).
	Message *Message
	// Station is a stop on a group's line (item 87).
	Station *StationHit
}

// StationHit is one station and the line it stands on.
type StationHit struct {
	Line  *Line
	Index int
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

// A card's face leads with a sentence (item 88): what the thing is and
// whose move it is, its group, the sentence, at most three numbers, the
// actions, and Details folded away under them.
type Card struct {
	Title    string
	Kind     string
	Move     string
	Group    string
	Sentence string
	Numbers  []Number
	// Details is the full table; Open is whether it is folded out.
	Details []string
	Open    bool
	// Hint is what the button under the pointer does (item 88).
	Hint    string
	Lines   []string
	Actions []string
	// Para, when set, is prose wrapped under the lines.
	Para string
	// Series, when set, is drawn as a sparkline under the lines.
	Series []float64
}

// Number is one figure on a card's face.
type Number struct {
	Label string
	Value string
}

// FaceNumbers is the most a card's face carries.
const FaceNumbers = 3

func ProjectRoot(cwd string) string {
	if cwd == "" {
		return ""
	}
	clean := filepath.Clean(cwd)
	if Temporary(clean) {
		return TemporaryRoot
	}
	if i := strings.Index(clean, "/"+worktreeSegment+"/"); i >= 0 {
		return clean[:i]
	}
	return clean
}

func Build(snapshot state.Snapshot, layout *Layout) *City {
	return BuildWith(snapshot, layout, state.DefaultThresholds)
}

// BuildWith builds the city with the ladder's thresholds. A stopped
// session keeps its lot in its own district and climbs the ladder there,
// so a district stays while it has any session at all (item 86).
func BuildWith(snapshot state.Snapshot, layout *Layout, ladder state.Thresholds) *City {
	return build(snapshot, layout, ladder, false, nil, nil, nil)
}

// BuildWithLedger builds the city reading its usage from a statistics
// store.
func BuildWithLedger(snapshot state.Snapshot, layout *Layout, ledger Ledger) *City {
	return build(snapshot, layout, state.DefaultThresholds, false, ledger, nil, nil)
}

// build builds the city; keepDepot holds the depot open, empty, while a
// truck is still leaving it with the last crate.
func build(snapshot state.Snapshot, layout *Layout, ladder state.Thresholds, keepDepot bool, ledger Ledger, m *memo, offMap []state.Session) *City {
	byRoot := map[string][]state.Session{}
	packed := map[string][]state.Session{}
	known := layout.HiddenRoots()
	var folderless []state.Session
	for _, s := range snapshot.Sessions {
		root := ProjectRoot(s.CWD)
		if root == "" {
			folderless = append(folderless, s)
			continue
		}
		known = append(known, root)
		if layout.IsHidden(root) {
			continue
		}
		if s.Packed {
			packed[root] = append(packed[root], s)
			continue
		}
		byRoot[root] = append(byRoot[root], s)
	}
	roots := make([]string, 0, len(byRoot))
	for root := range byRoot {
		roots = append(roots, root)
	}
	names := DistrictNames(known)
	sort.Slice(roots, func(i, j int) bool {
		return names[roots[i]] < names[roots[j]]
	})

	city := &City{Time: snapshot.At, ledger: ledger, names: names, folderless: folderless, offMap: offMap, memo: m}
	if len(byRoot)+len(packed) == 0 && !keepDepot {
		city.placeLandmarks(snapshot)
		city.Plant.Day = city.Breakdown(LastDay)
		return city
	}
	for _, root := range roots {
		city.Districts = append(city.Districts, buildDistrict(root, names[root], byRoot[root], layout, snapshot.At, ladder))
	}
	in := planInput(city.Districts, len(snapshot.Servers))
	in.SecondBridge = len(city.Districts) >= SecondBridgeAt || (layout != nil && layout.Bridge2 != nil)
	p := m.plan(in, layout.memory())
	depot := buildDepot(packed, names, float64(p.SpanCols())*CellSize, snapshot.At, keepDepot)
	if depot != nil {
		in.DepotRows = cellsNeeded(depot.size.Y)
		in.DepotCols = cellsNeeded(depot.size.X)
		p = m.plan(in, layout.memory())
	}
	city.plan = p
	city.planID = m.current()
	city.bounds = cellRect(p.Bounds)
	for _, d := range city.Districts {
		d.placeOn(cellRect(p.Blocks[d.Root]))
		d.at = snapshot.At
	}
	if depot != nil {
		depot.placeOn(cellRect(p.Depot))
		city.Districts = append(city.Districts, depot)
	}
	city.indexBuildings()
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
	city.layout = layout
	city.planLamps = append([]Cell(nil), city.Lamps...)
	city.planTrees = append([]Tree(nil), city.Trees...)
	city.Plaza = cellRect(p.Plaza)
	city.Fountain = cellAt(p.Fountain)
	city.placeLandmarks(snapshot)
	city.placeRoads(snapshot.Roads)
	city.placeRiver()
	city.placeBridge()
	city.placeStreets()
	city.placeCamps(snapshot.Teams)
	city.placeGroups(snapshot.Groups)
	if m != nil {
		m.orders.Sweep()
	}
	city.Plant.Day = city.Breakdown(LastDay)
	city.Trains = trains(city.Plant.Day, snapshot.Sessions, snapshot.At)
	city.priceSessions()
	return city
}

// DistrictName is a root's name on the map, told apart from any other
// root the city knows that shares its base name (item 139).
func (c *City) DistrictName(root string) string {
	if name, ok := c.names[root]; ok {
		return name
	}
	return DistrictNames([]string{root})[root]
}

func buildDistrict(root, name string, sessions []state.Session, layout *Layout, now time.Time, ladder state.Thresholds) *District {
	district := &District{Name: name, Root: root, Type: layout.DistrictType(root)}
	sort.Slice(sessions, func(i, j int) bool {
		if !sessions[i].StartedAt.Equal(sessions[j].StartedAt) {
			return sessions[i].StartedAt.Before(sessions[j].StartedAt)
		}
		return sessions[i].ID < sessions[j].ID
	})
	slots := layout.Slots(root, sessions)
	district.columns = buildingColumns(maxOf(slots) + 1)
	used := min(maxOf(slots)+1, district.columns)
	rows := math.Ceil(float64(maxOf(slots)+1) / float64(district.columns))
	district.size = Point{
		X: 2*DistrictPadding + pitch(used, BuildingSize, BuildingGap),
		Y: 2*DistrictPadding + pitch(int(rows), BuildingSize, BuildingGap),
	}
	for _, s := range sessions {
		column, row := slots[s.ID]%district.columns, slots[s.ID]/district.columns
		at := Point{X: DistrictPadding + float64(column)*(BuildingSize+BuildingGap), Y: DistrictPadding + float64(row)*(BuildingSize+BuildingGap)}
		b := newBuilding(s, slots[s.ID], at, BuildingSize, s.Rung(now, ladder))
		b.Type = district.Type
		b.Where = name
		district.Buildings = append(district.Buildings, b)
	}
	district.Rung = freshest(district.Buildings)
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

func newBuilding(s state.Session, slot int, at Point, size float64, rung state.Rung) *Building {
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
		Rung:      rung,
		Blinds:    rung != state.RungLive,
		BoardedUp: rung == state.RungShuttered || rung == state.RungOvergrown,
		Packed:    rung == state.RungPacked,
		Overgrown: rung == state.RungOvergrown,
		Vacant:    s.State == state.Empty,
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
	if c.buildings != nil {
		return c.buildings[:len(c.buildings):len(c.buildings)]
	}
	var buildings []*Building
	for _, d := range c.Districts {
		buildings = append(buildings, d.Buildings...)
	}
	return buildings
}

// indexBuildings lists the city's buildings once its districts are
// final, so a frame that asks for them, or for one by its session,
// does not walk every district to find it (item 270).
func (c *City) indexBuildings() {
	c.buildings = c.Buildings()
	c.byID = make(map[string]*Building, len(c.buildings))
	for _, b := range c.buildings {
		if _, dup := c.byID[b.Session.ID]; !dup {
			c.byID[b.Session.ID] = b
		}
	}
}

// building is the building of a session, or nil.
func (c *City) building(id string) *Building {
	if c.byID != nil {
		return c.byID[id]
	}
	for _, b := range c.Buildings() {
		if b.Session.ID == id {
			return b
		}
	}
	return nil
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
	// The parks, the belt and their trees are ground, and the ground
	// means nothing, so it answers no hover. Item 85.
	if c.Plaza.Area() > 0 && c.Plaza.Contains(p) {
		return Hit{Landmark: LandmarkPlaza}
	}
	return Hit{}
}

// stateWord is the state line's word: the rung for a stopped session,
// else its state.
func (b *Building) stateWord() string {
	if b.Rung != "" && b.Rung != state.RungLive {
		return string(b.Rung)
	}
	return string(b.Session.State)
}

// quietFor says how long a session has been quiet: minutes, then hours,
// then days.
func quietFor(d time.Duration) string {
	switch {
	case d < time.Hour:
		return plural(int(d/time.Minute), "minute")
	case d < 24*time.Hour:
		return plural(int(d/time.Hour), "hour")
	}
	return plural(int(d/(24*time.Hour)), "day")
}

func (b *Building) Card(now time.Time) Card {
	return b.face(now, ViewAttention, "")
}

// table is the building's full table, the card's Details.
func (b *Building) table(now time.Time) []string {
	s := b.Session
	lines := []string{"state    " + b.stateWord()}
	if b.Rung != state.RungLive && !s.LastActivity.IsZero() && !s.LastActivity.After(now) {
		lines = append(lines, "quiet    for "+quietFor(now.Sub(s.LastActivity)))
	}
	if s.ConversationGone {
		lines = append(lines, "gone     Claude Code deleted the conversation (cleanupPeriodDays)")
	}
	if b.Packed {
		lines = append(lines, packedLine(s, now))
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
	return lines
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
	if d.Depot {
		return d.depotCard()
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
	return d.face(lines)
}

// rungOrder is the ladder from freshest to quietest.
var rungOrder = map[state.Rung]int{state.RungLive: 0, state.RungResting: 1, state.RungShuttered: 2, state.RungOvergrown: 3}

// freshest is the freshest rung among buildings, or live when there are
// none.
func freshest(buildings []*Building) state.Rung {
	if len(buildings) == 0 {
		return state.RungLive
	}
	best := state.RungOvergrown
	for _, b := range buildings {
		if rungOrder[b.Rung] < rungOrder[best] {
			best = b.Rung
		}
	}
	return best
}
