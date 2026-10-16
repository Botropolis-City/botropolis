package city

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/format"
	"botropolis.city/botropolis/pkg/plan"
	"botropolis.city/botropolis/pkg/state"
)

const (
	PlantWidth    = 120.0
	PlantHeight   = 64.0
	TowerSize     = 32.0
	TowerGap      = 24.0
	LibraryWidth  = 72.0
	LibraryHeight = 96.0
	HallWidth     = 96.0
	HallHeight    = 64.0
	maxModelLines = 4
	recentDays    = 7
	LandmarkGap   = 64.0
	maxSkillLines = 6

	// Screen-space room for labels drawn beside the map at a fixed font size.
	// LabelCharWidth is the fixed label font's advance, so insets can be
	// sized to the words they hold.
	LabelCharWidth    = 7.0
	TowerLabelMargin  = 24.0
	LibraryLabelWidth = 40.0
	LabelHeight       = 20.0
)

type Landmark string

const (
	LandmarkNone     Landmark = ""
	LandmarkPlant    Landmark = "plant"
	LandmarkTower    Landmark = "tower"
	LandmarkLibrary  Landmark = "library"
	LandmarkHall     Landmark = "hall"
	LandmarkWater    Landmark = "water"
	LandmarkPark     Landmark = "park"
	LandmarkPlaza    Landmark = "plaza"
	LandmarkFountain Landmark = "fountain"
)

type Plant struct {
	Rect      Rect
	Power     state.Power
	BudgetUSD float64
}

type Tower struct {
	Rect   Rect
	Server state.Server
}

type Library struct {
	Rect   Rect
	Skills []state.Skill
}

type Hall struct {
	Rect  Rect
	Stats claude.Stats
}

type PowerLine struct {
	From, To      Point
	Building      *Building
	Fresh, Cached float64
}

// Label is what the road sign says: its message and file traffic.
func (r RoadLine) Label() string {
	label := roadTraffic(r.Messages+r.BackMessages, r.Files+r.BackFiles)
	if r.Visits > 0 {
		if label != "" {
			label += ", "
		}
		label += plural(r.Visits, "subagent")
	}
	return label
}

func roadTraffic(messages, files int) string {
	var parts []string
	if messages > 0 {
		parts = append(parts, plural(messages, "msg"))
	}
	if files > 0 {
		parts = append(parts, plural(files, "file"))
	}
	return strings.Join(parts, ", ")
}

// arrow joins the two ends of a line's card title with a glyph the bitmap
// font has.
const arrow = " -> "

// RoadLine is the connection between two districts, one per pair:
// Messages and Files cross from From to To, the Back counts the other
// way, and Active is how many of its travellers are working. Item 85.
type RoadLine struct {
	A, B         Point
	From, To     *District
	Messages     int
	Files        int
	BackMessages int
	BackFiles    int
	Visits       int
	Sessions     int
	Active       int
	Paths        []string
}

func (r RoadLine) Card() Card {
	lines := []string{"traffic  " + r.Label()}
	if back := roadTraffic(r.BackMessages, r.BackFiles); back != "" {
		lines = []string{
			"to       " + r.To.Name + " " + roadTraffic(r.Messages, r.Files),
			"to       " + r.From.Name + " " + back,
		}
		if r.Visits > 0 {
			lines = append(lines, "visits   "+plural(r.Visits, "subagent"))
		}
	}
	lines = append(lines, "sessions "+fmt.Sprint(r.Sessions))
	if r.Active > 0 {
		lines = append(lines, "working  "+fmt.Sprint(r.Active))
	}
	if len(r.Paths) > 0 {
		short := make([]string, 0, len(r.Paths))
		for _, path := range r.Paths {
			path = strings.TrimPrefix(path, r.To.Root+"/")
			short = append(short, strings.TrimPrefix(path, r.From.Root+"/"))
		}
		lines = append(lines, "files    "+strings.Join(short, ", "))
	}
	return Card{Title: r.From.Name + arrow + r.To.Name, Lines: lines}
}

func (b Beam) Card() Card {
	return Card{Title: b.Tower.Server.Name + arrow + b.Building.Card(time.Time{}).Title, Lines: []string{
		fmt.Sprintf("calls    %d this session", b.Calls),
	}}
}

func (l PowerLine) Card() Card {
	return Card{Title: "power" + arrow + l.Building.Card(time.Time{}).Title, Lines: []string{
		fmt.Sprintf("fresh    %s/min", format.Tokens(l.Fresh/60)),
		fmt.Sprintf("cached   %s/min", format.Tokens(l.Cached/60)),
	}}
}

func (c *City) placeRoads(roads []state.Road) {
	c.Roads = nil
	byRoot := map[string]*District{}
	for _, d := range c.Districts {
		byRoot[d.Root] = d
	}
	// Two roads can land on one pair of districts — the two directions,
	// or a worktree's session beside its project's — and a pair is one
	// connection, so they fold into the first.
	type pair struct{ a, b *District }
	index := map[pair]int{}
	for _, r := range roads {
		from, ok := byRoot[ProjectRoot(r.From)]
		if !ok {
			continue
		}
		to, ok := byRoot[ProjectRoot(r.To)]
		if !ok || from == to {
			continue
		}
		from.roads = append(from.roads, roadNote{other: to.Name, messages: r.Messages, files: r.Files, out: true})
		to.roads = append(to.roads, roadNote{other: from.Name, messages: r.Messages, files: r.Files, out: false})
		if r.BackMessages+r.BackFiles > 0 {
			to.roads = append(to.roads, roadNote{other: from.Name, messages: r.BackMessages, files: r.BackFiles, out: true})
			from.roads = append(from.roads, roadNote{other: to.Name, messages: r.BackMessages, files: r.BackFiles, out: false})
		}
		i, seen := index[pair{from, to}]
		if !seen {
			i, seen = index[pair{to, from}]
			if seen {
				r.Messages, r.Files, r.BackMessages, r.BackFiles = r.BackMessages, r.BackFiles, r.Messages, r.Files
			}
		}
		if !seen {
			index[pair{from, to}] = len(c.Roads)
			c.Roads = append(c.Roads, RoadLine{A: from.Rect.Center(), B: to.Rect.Center(), From: from, To: to,
				Messages: r.Messages, Files: r.Files, BackMessages: r.BackMessages, BackFiles: r.BackFiles,
				Visits: r.Visits, Sessions: r.Sessions, Active: r.Active, Paths: r.Paths})
			continue
		}
		line := &c.Roads[i]
		line.Messages += r.Messages
		line.Files += r.Files
		line.BackMessages += r.BackMessages
		line.BackFiles += r.BackFiles
		line.Visits += r.Visits
		line.Sessions += r.Sessions
		line.Active += r.Active
		line.Paths = append(line.Paths, r.Paths...)
		if len(line.Paths) > state.MaxRoadPaths {
			line.Paths = line.Paths[:state.MaxRoadPaths]
		}
	}
}

type Beam struct {
	From, To Point
	Tower    *Tower
	Building *Building
	Calls    int
}

// placeLandmarks puts the civic buildings on the plaza — the plant top
// left, the hall top right, the library bottom left, the fountain on the
// centre cell — and one tower per server along the ridge.
func (c *City) placeLandmarks(snapshot state.Snapshot) {
	if c.Plaza.Area() == 0 {
		return
	}
	pad := Tile
	plaza := c.Plaza
	// Each civic building stands on the plot the plan reserved for it,
	// rather than being measured off the plaza a second time. The plan
	// is what keeps the planting off this ground, so the planting and
	// the building have to be reading the same reservation or they are
	// two opinions again.
	plots := c.plan.Plots
	c.Plant = Plant{Power: snapshot.Power}
	c.Plant.Rect = plotRect(plots.Plant, plaza, pad, PlantWidth, PlantHeight,
		RectAt(plaza.Min.X+pad, plaza.Min.Y+pad, PlantWidth, PlantHeight))

	c.Towers = nil
	for i, server := range snapshot.Servers {
		if i >= len(c.plan.Towers) {
			break
		}
		cell := cellAt(c.plan.Towers[i])
		c.Towers = append(c.Towers, &Tower{
			Server: server,
			Rect:   RectAt(cell.Center().X-TowerSize/2, cell.Center().Y-TowerSize/2, TowerSize, TowerSize),
		})
	}

	c.Library = Library{Skills: snapshot.Skills}
	c.Library.Rect = plotRect(plots.Library, plaza, pad, LibraryWidth, LibraryHeight,
		RectAt(plaza.Min.X+pad, plaza.Max.Y-pad-LibraryHeight, LibraryWidth, LibraryHeight))

	c.Hall = Hall{}
	if snapshot.Stats != nil {
		c.Hall.Stats = *snapshot.Stats
		c.Hall.Rect = plotRect(plots.Hall, plaza, pad, HallWidth, HallHeight,
			RectAt(plaza.Max.X-pad-HallWidth, plaza.Min.Y+pad, HallWidth, HallHeight))
	}
}

func (c *City) DistrictBounds() Rect {
	if len(c.Districts) == 0 {
		return Rect{}
	}
	bounds := c.Districts[0].Rect
	for _, d := range c.Districts[1:] {
		bounds = bounds.Union(d.Rect)
	}
	return bounds
}

func (c *City) PowerLines() []PowerLine {
	var lines []PowerLine
	if c.Plant.Rect.Area() == 0 {
		return nil
	}
	// A wire is the daemon's live line to a session: no hook events, no
	// wire, and the session is being read from files.
	for _, b := range c.Buildings() {
		if !b.Lit || !b.Session.Hooked || b.Session.FreshTokensPerHour+b.Session.CacheReadPerHour <= 0 {
			continue
		}
		lines = append(lines, PowerLine{
			From: c.Plant.Rect.Center(), To: b.Rect.Center(), Building: b,
			Fresh: b.Session.FreshTokensPerHour, Cached: b.Session.CacheReadPerHour,
		})
	}
	return lines
}

func (c *City) Beams() []Beam {
	towers := map[string]*Tower{}
	for _, t := range c.Towers {
		towers[t.Server.Name] = t
	}
	var beams []Beam
	for _, b := range c.Buildings() {
		names := make([]string, 0, len(b.Session.MCPCalls))
		for name := range b.Session.MCPCalls {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			tower, ok := towers[name]
			if !ok {
				continue
			}
			beams = append(beams, Beam{
				From: tower.Rect.Center(), To: b.Rect.Center(), Tower: tower, Building: b, Calls: b.Session.MCPCalls[name],
			})
		}
	}
	return beams
}

func (p Plant) Card() Card {
	cost := "cost     " + format.USD(p.Power.CostUSD, p.Power.CostKnown) + " in 24h (pro-rated)"
	if !p.Power.CostKnown {
		cost = "cost     " + format.Unknown + " in 24h (no cost-state in any transcript)"
	}
	lines := []string{cost}
	if p.BudgetUSD > 0 && p.Power.CostKnown {
		lines = append(lines, fmt.Sprintf("budget   $%.0f/day, %s used", p.BudgetUSD, format.Percent(100*p.Power.CostUSD/p.BudgetUSD)))
	}
	lines = append(lines,
		fmt.Sprintf("fresh    %s tokens", format.Tokens(float64(p.Power.Fresh))),
		fmt.Sprintf("cached   %s tokens", format.Tokens(float64(p.Power.Cached))),
		fmt.Sprintf("hit rate %s of prompt tokens from cache", format.Percent(100*p.Power.HitRatio())),
	)
	models := make([]string, 0, len(p.Power.ByModel))
	for model := range p.Power.ByModel {
		models = append(models, model)
	}
	sort.Strings(models)
	for _, model := range models {
		lines = append(lines, fmt.Sprintf("%s  %s out", model, format.Tokens(float64(p.Power.ByModel[model].Output))))
	}
	return Card{Title: "Power plant", Lines: lines}
}

func (t Tower) Card() Card {
	config := "configured"
	if !t.Server.Configured {
		config = "used, not configured"
	}
	return Card{Title: t.Server.Name, Lines: []string{
		fmt.Sprintf("calls    %d in %s", t.Server.Calls, plural(t.Server.Sessions, "session")),
		"config   " + config,
		"type     " + format.Dash(t.Server.Type),
	}}
}

func (l Library) Card() Card {
	lines := []string{}
	for i, skill := range l.Skills {
		if i == maxSkillLines {
			break
		}
		lines = append(lines, fmt.Sprintf("%s  %d calls, %s", skill.Name, skill.Calls, plural(skill.Sessions, "session")))
	}
	if len(lines) == 0 {
		lines = []string{"no skills invoked"}
	}
	return Card{Title: "Library", Lines: lines}
}

func (h Hall) Card(now time.Time) Card {
	st := h.Stats
	computed := "computed " + format.Dash(st.LastComputed)
	if on, ok := st.ComputedOn(); ok {
		computed += fmt.Sprintf(" (%s ago)", plural(int(now.Sub(on).Hours()/24), "day"))
	}
	lines := []string{
		computed,
		fmt.Sprintf("sessions %d since %s", st.TotalSessions, st.FirstSessionAt.Format("2006-01-02")),
		fmt.Sprintf("messages %d", st.TotalMessages),
	}
	if days := st.RecentDays(recentDays); len(days) > 0 {
		var msgs, sessions int
		for _, d := range days {
			msgs += d.Messages
			sessions += d.Sessions
		}
		lines = append(lines, fmt.Sprintf("last %s  %d msgs/day, %s", plural(len(days), "day"), msgs/len(days), plural(sessions, "session")))
	}
	if hour := st.BusiestHour(); hour >= 0 {
		lines = append(lines, fmt.Sprintf("busiest  %02d:00", hour))
	}
	for i, m := range st.ModelShare() {
		if i == maxModelLines {
			break
		}
		lines = append(lines, fmt.Sprintf("%s  %s", m.Model, format.Percent(100*m.Share)))
	}
	return Card{Title: "City hall", Lines: lines}
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// plotRect centres a landmark of w by h on the plot the plan reserved,
// padded off its edges, and falls back to the given rect when the plaza
// was too small for the plan to reserve anything.
func plotRect(plot plan.Block, plaza Rect, pad, w, h float64, fallback Rect) Rect {
	if plot.Cols == 0 || plot.Rows == 0 {
		return fallback
	}
	area := Rect{
		Min: Point{X: float64(plot.Min.Col) * CellSize, Y: float64(plot.Min.Row) * CellSize},
	}
	area.Max = Point{X: area.Min.X + float64(plot.Cols)*CellSize, Y: area.Min.Y + float64(plot.Rows)*CellSize}
	area = area.Inset(pad)
	return RectAt(area.Min.X+(area.Width()-w)/2, area.Min.Y+(area.Height()-h)/2, w, h)
}
