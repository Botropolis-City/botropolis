package city

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/state"
)

// VeilKind is what the veil does with one string a snapshot carries.
type VeilKind string

const (
	// VeilKeep keeps a string that names nobody: an id, a state, a model.
	VeilKeep VeilKind = "keep"
	// VeilPath keeps a folder or file, because the layout and the roads
	// are keyed by it; the guard still refuses its folder names.
	VeilPath VeilKind = "path"
	// VeilText blanks free text: prompts, notes, reasons.
	VeilText VeilKind = "text"
	// VeilLink drops a link outright.
	VeilLink VeilKind = "link"
	// VeilTool keeps a tool's own name but veils an MCP server in it.
	VeilTool VeilKind = "tool"
	// The rest are names, each replaced with "<Kind> N", the same N for
	// the same name for as long as the veil is worn.
	VeilSession    VeilKind = "Session"
	VeilServer     VeilKind = "Tower"
	VeilSkill      VeilKind = "Skill"
	VeilGroup      VeilKind = "Group"
	VeilTeam       VeilKind = "Team"
	VeilAgent      VeilKind = "Agent"
	VeilRepository VeilKind = "Repository"
)

// veiledText is what free text reads as under the veil.
const veiledText = "…"

// VeilTable says, for every string a snapshot can carry, by its type and
// field, what the veil does with it. TestVeilAccountsForEveryWord fails
// on a string that is not here, so a new field cannot leak a name by
// being forgotten.
var VeilTable = map[string]VeilKind{
	"Session.ID":                          VeilKeep,
	"Session.Harness":                     VeilKeep,
	"Session.Title":                       VeilSession,
	"Session.CWD":                         VeilPath,
	"Session.Branch":                      VeilText,
	"Session.Model":                       VeilKeep,
	"Session.Kind":                        VeilKeep,
	"Session.Job":                         VeilKeep,
	"Session.Name":                        VeilSession,
	"Session.State":                       VeilKeep,
	"Session.Turn":                        VeilKeep,
	"Session.RecordStatus":                VeilKeep,
	"Session.Tool":                        VeilTool,
	"Session.Note":                        VeilText,
	"Session.Subject":                     VeilText,
	"Session.SubagentNames":               VeilAgent,
	"Session.SubagentDirs":                VeilPath,
	"Session.Team":                        VeilTeam,
	"Session.Leads":                       VeilTeam,
	"Session.Group":                       VeilKeep,
	"Session.GroupJoined":                 VeilText,
	"Session.Agent":                       VeilAgent,
	"Session.Messages{key}":               VeilKeep,
	"PeerMessage.ID":                      VeilKeep,
	"PeerMessage.FromName":                VeilAgent,
	"Session.Messaged":                    VeilKeep,
	"Session.GroupWords":                  VeilText,
	"Session.Touches{key}":                VeilPath,
	"Session.Visits{key}":                 VeilPath,
	"Session.VisitSpans{key}":             VeilPath,
	"Session.MCPCalls{key}":               VeilServer,
	"Session.Skills{key}":                 VeilSkill,
	"PR.URL":                              VeilLink,
	"PR.Repository":                       VeilRepository,
	"PR.State":                            VeilKeep,
	"OrderLine.Text":                      VeilText,
	"Recap.Text":                          VeilText,
	"Session.LastPrompt":                  VeilText,
	"Server.Name":                         VeilServer,
	"Server.Type":                         VeilKeep,
	"Skill.Name":                          VeilSkill,
	"Road.From":                           VeilPath,
	"Road.To":                             VeilPath,
	"Road.Paths":                          VeilPath,
	"Team.Name":                           VeilTeam,
	"Team.LeadSessionID":                  VeilKeep,
	"TeamMember.Name":                     VeilAgent,
	"TeamMember.AgentID":                  VeilKeep,
	"TeamMember.AgentType":                VeilKeep,
	"TeamMember.CWD":                      VeilPath,
	"Group.ID":                            VeilKeep,
	"Group.Name":                          VeilGroup,
	"Group.Short":                         VeilGroup,
	"Group.Anchor":                        VeilKeep,
	"Group.Members":                       VeilKeep,
	"Group.MemberDirs":                    VeilPath,
	"Group.Reason":                        VeilText,
	"Group.Move":                          VeilKeep,
	"Group.Why":                           VeilText,
	"Journal.LastTime":                    VeilKeep,
	"Journal.Next":                        VeilText,
	"Power.ByModel{key}":                  VeilKeep,
	"Stats.LastComputed":                  VeilKeep,
	"DailyActivity.Date":                  VeilKeep,
	"DailyModelTokens.Date":               VeilKeep,
	"DailyModelTokens.TokensByModel{key}": VeilKeep,
	"Stats.ModelUsage{key}":               VeilKeep,
	"SkippedFile.Path":                    VeilPath,
	"StatsStore.State":                    VeilKeep,
	"StatsStore.Path":                     VeilText,
	"StatsStore.Reason":                   VeilText,
	"StatsStore.Why":                      VeilKeep,
}

// Veil is the neutral names a city wears while it is shared (item 168):
// District A, Session 3, Tower 2. A name keeps its alias for as long as
// the veil is worn, so a clip does not rename things halfway through.
type Veil struct {
	aliases map[VeilKind]map[string]string
}

func NewVeil() *Veil {
	return &Veil{aliases: map[VeilKind]map[string]string{}}
}

// alias is a name's stand-in, numbered in the order names are first met.
func (v *Veil) alias(kind VeilKind, name string) string {
	seen := v.aliases[kind]
	if seen == nil {
		seen = map[string]string{}
		v.aliases[kind] = seen
	}
	if a, ok := seen[name]; ok {
		return a
	}
	n := len(seen) + 1
	a := fmt.Sprintf("%s %d", kind, n)
	if kind == districtKind {
		a = "District " + letters(n)
	}
	seen[name] = a
	return a
}

const districtKind VeilKind = "District"

// letters is A for 1, Z for 26, AA for 27.
func letters(n int) string {
	var s string
	for n > 0 {
		n--
		s = string(rune('A'+n%26)) + s
		n /= 26
	}
	return s
}

// Snapshot is a copy of s wearing the veil; s itself is not touched.
func (v *Veil) Snapshot(s state.Snapshot) state.Snapshot {
	order := slices.Clone(s.Sessions)
	slices.SortStableFunc(order, func(a, b state.Session) int {
		if c := a.StartedAt.Compare(b.StartedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	for _, session := range order {
		v.alias(VeilSession, session.ID)
	}
	for _, server := range s.Servers {
		v.alias(VeilServer, server.Name)
	}
	out := v.copy(reflect.ValueOf(s), "Snapshot").Interface().(state.Snapshot)
	for i := range out.Sessions {
		alias := v.alias(VeilSession, out.Sessions[i].ID)
		out.Sessions[i].Title = alias
		if out.Sessions[i].Name != "" {
			out.Sessions[i].Name = alias
		}
	}
	return out
}

func (v *Veil) word(kind VeilKind, s string) string {
	if s == "" {
		return s
	}
	switch kind {
	case VeilKeep, VeilPath:
		return s
	case VeilText:
		return veiledText
	case VeilLink:
		return ""
	case VeilSession:
		// Set by id once the copy is made, since a title is no key.
		return s
	case VeilTool:
		parts := strings.SplitN(s, "__", 3)
		if len(parts) == 3 && parts[0] == "mcp" {
			return "mcp__" + v.alias(VeilServer, parts[1]) + "__" + parts[2]
		}
		return s
	}
	return v.alias(kind, s)
}

var timeType = reflect.TypeOf(time.Time{})

func (v *Veil) copy(val reflect.Value, path string) reflect.Value {
	switch val.Kind() {
	case reflect.String:
		kind, ok := VeilTable[path]
		if !ok {
			// Not in the table: the test says so, and until then the
			// string is not let through.
			kind = VeilText
		}
		return reflect.ValueOf(v.word(kind, val.String())).Convert(val.Type())
	case reflect.Pointer:
		if val.IsNil() {
			return val
		}
		out := reflect.New(val.Type().Elem())
		out.Elem().Set(v.copy(val.Elem(), path))
		return out
	case reflect.Slice:
		if val.IsNil() {
			return val
		}
		out := reflect.MakeSlice(val.Type(), val.Len(), val.Len())
		for i := range val.Len() {
			out.Index(i).Set(v.copy(val.Index(i), path))
		}
		return out
	case reflect.Array:
		out := reflect.New(val.Type()).Elem()
		for i := range val.Len() {
			out.Index(i).Set(v.copy(val.Index(i), path))
		}
		return out
	case reflect.Map:
		if val.IsNil() {
			return val
		}
		out := reflect.MakeMapWithSize(val.Type(), val.Len())
		iter := val.MapRange()
		for iter.Next() {
			out.SetMapIndex(v.copy(iter.Key(), path+"{key}"), v.copy(iter.Value(), path))
		}
		return out
	case reflect.Struct:
		out := reflect.New(val.Type()).Elem()
		out.Set(val)
		if val.Type() == timeType {
			return out
		}
		for i := range val.NumField() {
			if !val.Type().Field(i).IsExported() {
				continue
			}
			out.Field(i).Set(v.copy(val.Field(i), val.Type().Name()+"."+val.Type().Field(i).Name))
		}
		return out
	}
	return val
}

// name gives each district its alias, in the order the city lists them.
func (v *Veil) name(c *City) {
	for _, d := range c.Districts {
		if d.Depot {
			continue
		}
		d.Name = v.alias(districtKind, d.Root)
		c.names[d.Root] = d.Name
	}
	for root := range c.names {
		c.names[root] = v.alias(districtKind, root)
	}
}

// Identifying is what in a snapshot identifies the person: Names are
// every name the veil replaces and every folder in every path it keeps,
// and Texts every free text it blanks. The guard refuses to draw any of
// them while the city is shared (item 168).
type Identifying struct {
	Names []string
	Texts []string
}

func Words(s state.Snapshot) Identifying {
	seen := map[string]bool{}
	var id Identifying
	add := func(list *[]string, w string) {
		w = strings.TrimSpace(w)
		if w == "" || seen[w] {
			return
		}
		seen[w] = true
		*list = append(*list, w)
	}
	walkWords(reflect.ValueOf(s), "Snapshot", func(path, str string) {
		switch VeilTable[path] {
		case VeilKeep:
		case VeilPath:
			for _, part := range strings.Split(filepath.ToSlash(str), "/") {
				add(&id.Names, part)
			}
		case VeilTool:
			if parts := strings.SplitN(str, "__", 3); len(parts) == 3 && parts[0] == "mcp" {
				add(&id.Names, parts[1])
			}
		case VeilText, VeilLink:
			add(&id.Texts, str)
		default:
			add(&id.Names, str)
		}
	})
	return id
}

func walkWords(val reflect.Value, path string, visit func(path, s string)) {
	switch val.Kind() {
	case reflect.String:
		visit(path, val.String())
	case reflect.Pointer:
		if !val.IsNil() {
			walkWords(val.Elem(), path, visit)
		}
	case reflect.Slice, reflect.Array:
		for i := range val.Len() {
			walkWords(val.Index(i), path, visit)
		}
	case reflect.Map:
		iter := val.MapRange()
		for iter.Next() {
			walkWords(iter.Key(), path+"{key}", visit)
			walkWords(iter.Value(), path, visit)
		}
	case reflect.Struct:
		if val.Type() == timeType {
			return
		}
		for i := range val.NumField() {
			if val.Type().Field(i).IsExported() {
				walkWords(val.Field(i), val.Type().Name()+"."+val.Type().Field(i).Name, visit)
			}
		}
	}
}

// SetVeil puts the veil on, or with nil takes it off, and redraws the
// city from the snapshot as it came.
func (s *Scene) SetVeil(v *Veil) {
	s.veil = v
	s.restate()
}

// Veiled is whether the city is wearing the veil.
func (s *Scene) Veiled() bool { return s.veil != nil }

// RealSnapshot is the snapshot as it came, whether or not the city wears
// the veil.
func (s *Scene) RealSnapshot() state.Snapshot { return s.real }
