package city

// Kind is a kind of thing on the map (item 88). Every card starts with
// its kind's line, explain mode captions one of each kind on screen, and
// the reference behind ? has an entry for each.
type Kind int

const (
	KindNone Kind = iota
	KindSession
	KindProject
	KindPlant
	KindLibrary
	KindHall
	KindTower
	KindRiver
	KindFountain
	KindPlaza
	KindRoad
	KindBeam
	KindWire
	KindTrain
	KindTug
	KindLaunch
	KindDrone
	KindSmoke
	KindGauge
	KindCar
	KindFlag
	KindStation
	// KindInspector and KindDelivery are a pull request's other two
	// movers (item 89).
	KindInspector
	KindDelivery
	KindVan
	// KindClinic is the clinic, where what needs you stands (item 280).
	KindClinic
)

// Kinds is every kind, in the order the reference lists them.
var Kinds = []Kind{
	KindSession, KindProject, KindStation, KindDrone, KindSmoke, KindFlag, KindInspector, KindDelivery, KindVan,
	KindPlant, KindWire, KindTrain, KindTower, KindBeam, KindLibrary, KindHall, KindClinic,
	KindRoad, KindCar, KindRiver, KindGauge, KindTug, KindLaunch, KindFountain, KindPlaza,
}

type kindText struct {
	name, is, about string
	related         []Kind
}

var kindTexts = map[Kind]kindText{
	KindSession: {"session", "a Claude Code session", "Each building is one Claude Code session. Its colour says its state: waiting for you, working, or resting. While Claude is mid-turn the roof is scaffolded, and each tool it uses sends a load up the hoist on the building's corner. Select it to read its card; its card leads with what Claude said last.",
		[]Kind{KindProject, KindDrone, KindSmoke, KindFlag}},
	KindProject: {"project", "a directory where sessions run", "Each district is a project: a directory on this machine. Its buildings are the sessions started there.",
		[]Kind{KindSession, KindRoad, KindStation}},
	KindStation: {"station", "a project on a group's line", "A group of sessions working on one thing is drawn as a line through its projects, with a station at each. A station's colour says whose move it is there.",
		[]Kind{KindProject, KindSession}},
	KindDrone: {"drone", "one subagent in flight", "Each drone working on a building is one subagent the session has running. It flies to the project its subagent works in, and home when it finishes.",
		[]Kind{KindSession}},
	KindSmoke: {"smoke", "errors from the API", "Smoke over a roof means the session has had errors from the API. The more smoke, the more errors.",
		[]Kind{KindSession}},
	KindFlag: {"flag", "a pull request", "A flag on a roof is a pull request the session opened. It turns green when the pull request is merged, and comes down when it is closed.",
		[]Kind{KindSession, KindInspector, KindDelivery}},
	KindInspector: {"inspector", "pull requests waiting on review", "An inspector's car parks outside a session's building while the session has a pull request open, and drives off when none is.",
		[]Kind{KindFlag, KindSession}},
	KindVan: {"van", "a message between sessions", "A van drives the road from one project to another for each message one session sends another. Between two stations on a group's line, a carriage runs the line instead.",
		[]Kind{KindRoad, KindCar}},
	KindDelivery: {"delivery truck", "a merged pull request", "When a pull request merges, a truck takes a crate away over the bridge. When the merge leaves its group with nothing open, the group's ribbon is cut.",
		[]Kind{KindFlag, KindInspector}},
	KindPlant: {"power plant", "where the tokens go", "The power plant is the city's spend: tokens and cost across every session. Select it to see where the spend went.",
		[]Kind{KindWire, KindTrain}},
	KindWire: {"wire", "a session's tokens, from the plant", "A wire runs from the plant to every session the daemon hears from. A thicker wire carries more tokens. A building with no wire is read from files only.",
		[]Kind{KindPlant, KindSession}},
	KindTrain: {"train", "one model's spend today", "Each train on the loop is one model. Its length is that model's share of today's tokens.",
		[]Kind{KindPlant}},
	KindTower: {"tower", "an MCP server", "Each radio tower is an MCP server. Point at one to light the sessions that call it.",
		[]Kind{KindBeam, KindSession}},
	KindBeam: {"beam", "calls from a session to a server", "A beam joins a session to an MCP server it calls. A brighter beam carries more calls.",
		[]Kind{KindTower, KindSession}},
	KindLibrary: {"library", "the skills in use", "The library holds the skills sessions have used, and how often.",
		[]Kind{KindSession}},
	KindHall: {"city hall", "the board of groups", "City hall holds the board of groups: each piece of work, whose move it is, and what is next. Move to group on a session's card, then new group…, makes a group.",
		[]Kind{KindStation}},
	KindClinic: {"clinic", "what needs you, and what is in trouble", "The clinic counts the sessions waiting on you and the ones in trouble: one whose last reply was an error from the API, one that stopped mid-turn in the last day, or one whose last three tool calls or more failed. Click it, or press a, for the list: the ones needing you first, then the troubled ones, each a key away.",
		[]Kind{KindSession, KindSmoke}},
	KindRoad: {"road", "traffic between two projects", "A road joins two projects that sessions crossed between, by messages or by files. Roads appear only where traffic runs.",
		[]Kind{KindProject, KindCar}},
	KindCar: {"car", "a session crossing to another project", "A car on a road is a session working across two projects right now.",
		[]Kind{KindRoad}},
	KindRiver: {"river", "your usage, and sessions coming and going", "The river runs through the island to the sea. Its boats carry your usage and the sessions arriving and leaving.",
		[]Kind{KindGauge, KindTug, KindLaunch}},
	KindGauge: {"gauge boat", "your share of a usage limit", "Each big boat on the river is one usage limit. It holds station between the buoys at the share you have used.",
		[]Kind{KindRiver, KindLaunch}},
	KindTug: {"tug", "a session arriving or leaving", "A tug brings a new session to its project, and carries one away when it leaves the map.",
		[]Kind{KindRiver, KindSession}},
	KindLaunch: {"launch", "fresh usage figures on their way", "The white launch is a refresh of your usage figures that you asked for with " + RefreshKey + ".",
		[]Kind{KindGauge}},
	KindFountain: {"fountain", "the middle of the city", "The fountain marks the middle of the plaza.",
		[]Kind{KindPlaza}},
	KindPlaza: {"plaza", "the civic centre", "The plaza holds the civic buildings: the power plant, city hall, the library and the clinic.",
		[]Kind{KindPlant, KindHall, KindLibrary, KindClinic}},
}

func (k Kind) Name() string { return kindTexts[k].name }

// Is is what the kind is, in one line.
func (k Kind) Is() string { return kindTexts[k].is }

// About is the reference's entry for the kind.
func (k Kind) About() string { return kindTexts[k].about }

// Related are the kinds the reference's entry links to.
func (k Kind) Related() []Kind { return kindTexts[k].related }

// Caption is the kind as explain mode and a first sighting say it.
func (k Kind) Caption() string { return k.Name() + " · " + k.Is() }

// KindOf is the kind of thing a hit is on, the movers before the
// buildings they circle, as the card answers.
func KindOf(h Hit) Kind {
	switch {
	case h.Station != nil:
		return KindStation
	case h.Gauge != nil:
		return KindGauge
	case h.Car != nil:
		return KindCar
	case h.Flag != nil:
		return KindFlag
	case h.Delivery != nil:
		return KindDelivery
	case h.Message != nil:
		return KindVan
	case h.Inspector != nil:
		return KindInspector
	case h.Subagent != nil:
		return KindDrone
	case h.Smoke != nil:
		return KindSmoke
	case h.Landmark == LandmarkPlant:
		return KindPlant
	case h.Landmark == LandmarkTower && h.Tower != nil:
		return KindTower
	case h.Landmark == LandmarkLibrary:
		return KindLibrary
	case h.Landmark == LandmarkHall:
		return KindHall
	case h.Landmark == LandmarkClinic:
		return KindClinic
	case h.Building != nil:
		return KindSession
	case h.District != nil:
		return KindProject
	case h.Road != nil:
		return KindRoad
	case h.Beam != nil:
		return KindBeam
	case h.Line != nil:
		return KindWire
	case h.Train != nil:
		return KindTrain
	case h.Voyage != nil:
		return KindTug
	case h.Courier != nil:
		return KindLaunch
	case h.Landmark == LandmarkWater:
		return KindRiver
	case h.Landmark == LandmarkFountain:
		return KindFountain
	case h.Landmark == LandmarkPlaza:
		return KindPlaza
	}
	return KindNone
}
