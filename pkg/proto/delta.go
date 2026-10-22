package proto

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"botropolis.city/botropolis/pkg/state"
)

// ErrResync is a delta that does not follow the snapshot a client
// holds: it lets the subscription go and attaches again, for a whole
// snapshot (item 201).
var ErrResync = errors.New("a delta did not follow the snapshot held, so the stream must start again")

// Delta is what changed in the daemon's view since the update a
// connection was last sent, which is Base: each top-level section that
// changed, whole, but for the lists keyed by ID, which say what changed
// in them.
type Delta struct {
	Base     uint64                     `json:"base"`
	Sections map[string]json.RawMessage `json:"sections,omitempty"`
	Lists    map[string]*List           `json:"lists,omitempty"`
}

// List is what changed in a list keyed by ID: a changed item carries
// only the top-level fields that changed, null for one that is gone.
// Order is sent only when it is not the last one's with the removed
// taken out and the added put last.
type List struct {
	Added   []json.RawMessage                     `json:"added,omitempty"`
	Removed []string                              `json:"removed,omitempty"`
	Changed map[string]map[string]json.RawMessage `json:"changed,omitempty"`
	Order   []string                              `json:"order,omitempty"`
}

var (
	null  = json.RawMessage("null")
	empty = json.RawMessage("[]")
)

// keyed is a list of the snapshot sent item by item: the sessions, and
// the groups, whose journals move with every line a member writes.
type keyed struct {
	name   string
	encode func(state.Snapshot) ([]string, []json.RawMessage, bool, error)
	clear  func(*state.Snapshot)
	apply  func(*state.Snapshot, *List, bool) error
}

var lists = []keyed{
	keyedList("sessions", func(s *state.Snapshot) *[]state.Session { return &s.Sessions }, func(s state.Session) string { return s.ID }),
	keyedList("groups", func(s *state.Snapshot) *[]state.Group { return &s.Groups }, func(g state.Group) string { return g.ID }),
}

func keyedList[T any](name string, of func(*state.Snapshot) *[]T, id func(T) string) keyed {
	fields := wireFields(reflect.TypeFor[T]())
	omitted := omitsEmpty(name)
	return keyed{
		name: name,
		encode: func(s state.Snapshot) ([]string, []json.RawMessage, bool, error) {
			items := *of(&s)
			ids, raws := make([]string, len(items)), make([]json.RawMessage, len(items))
			for i, item := range items {
				raw, err := json.Marshal(item)
				if err != nil {
					return nil, nil, false, err
				}
				ids[i], raws[i] = id(item), raw
			}
			return ids, raws, items != nil && len(items) == 0 && !omitted, nil
		},
		clear: func(s *state.Snapshot) { *of(s) = nil },
		apply: func(s *state.Snapshot, l *List, emptySlice bool) error {
			items, err := applyList(*of(s), id, fields, l)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			if len(items) == 0 && !emptySlice {
				items = nil
			}
			*of(s) = items
			return nil
		},
	}
}

// sent is a keyed list as a connection was last sent it.
type sent struct {
	raw   map[string]json.RawMessage
	order []string
}

// Sender turns a connection's stream of views into deltas, each against
// the one it last sent on that connection, so an update the daemon
// folded into a later one leaves no gap.
type Sender struct {
	seq      uint64
	held     bool
	sections map[string]json.RawMessage
	lists    map[string]sent
}

func NewSender() *Sender {
	return &Sender{}
}

// Next is the response that brings a client holding the last view sent
// to this one: the whole snapshot first, and whenever a delta would not
// be smaller or a list cannot be keyed.
func (s *Sender) Next(snapshot state.Snapshot) (Response, error) {
	s.seq++
	rest := snapshot
	now := map[string]sent{}
	markers := map[string]json.RawMessage{}
	size, unique := 0, true
	for _, l := range lists {
		ids, raws, emptySlice, err := l.encode(snapshot)
		if err != nil {
			return Response{}, err
		}
		list := sent{raw: make(map[string]json.RawMessage, len(ids)), order: ids}
		for i, id := range ids {
			if _, seen := list.raw[id]; seen {
				unique = false
			}
			list.raw[id] = raws[i]
			size += len(raws[i]) + 1
		}
		now[l.name] = list
		markers[l.name] = null
		if emptySlice {
			markers[l.name] = empty
		}
		l.clear(&rest)
	}
	raw, err := json.Marshal(rest)
	if err != nil {
		return Response{}, err
	}
	size += len(raw)
	var sections map[string]json.RawMessage
	if err := json.Unmarshal(raw, &sections); err != nil {
		return Response{}, err
	}
	// A list's own key says only whether an empty one is sent as [] or
	// as nothing, which decode differently.
	for k, v := range markers {
		sections[k] = v
	}
	full := func() (Response, error) {
		s.held = unique
		s.sections, s.lists = sections, now
		return Response{Seq: s.seq, Snapshot: &snapshot}, nil
	}
	if !s.held || !unique {
		return full()
	}
	delta := Delta{Base: s.seq - 1}
	for k, v := range sections {
		if !bytes.Equal(s.sections[k], v) {
			delta.Sections = put(delta.Sections, k, v)
		}
	}
	for k := range s.sections {
		if _, ok := sections[k]; !ok {
			delta.Sections = put(delta.Sections, k, null)
		}
	}
	for name, list := range now {
		changes, err := diffList(s.lists[name], list)
		if err != nil {
			return Response{}, err
		}
		if changes != nil {
			if delta.Lists == nil {
				delta.Lists = map[string]*List{}
			}
			delta.Lists[name] = changes
		}
	}
	encoded, err := json.Marshal(delta)
	if err != nil {
		return Response{}, err
	}
	if len(encoded) >= size {
		return full()
	}
	s.sections, s.lists = sections, now
	return Response{Seq: s.seq, Delta: &delta}, nil
}

func diffList(was, now sent) (*List, error) {
	var l List
	removed := map[string]bool{}
	for _, id := range was.order {
		if _, ok := now.raw[id]; !ok {
			l.Removed = append(l.Removed, id)
			removed[id] = true
		}
	}
	var added []string
	for _, id := range now.order {
		before, ok := was.raw[id]
		switch {
		case !ok:
			l.Added = append(l.Added, now.raw[id])
			added = append(added, id)
		case !bytes.Equal(before, now.raw[id]):
			fields, err := fieldsChanged(before, now.raw[id])
			if err != nil {
				return nil, err
			}
			if l.Changed == nil {
				l.Changed = map[string]map[string]json.RawMessage{}
			}
			l.Changed[id] = fields
		}
	}
	expected := slices.DeleteFunc(slices.Clone(was.order), func(id string) bool { return removed[id] })
	if expected = append(expected, added...); !slices.Equal(expected, now.order) {
		l.Order = now.order
		if l.Order == nil {
			l.Order = []string{}
		}
	}
	if l.Added == nil && l.Removed == nil && l.Changed == nil && l.Order == nil {
		return nil, nil
	}
	return &l, nil
}

func put(m map[string]json.RawMessage, k string, v json.RawMessage) map[string]json.RawMessage {
	if m == nil {
		m = map[string]json.RawMessage{}
	}
	m[k] = v
	return m
}

func fieldsChanged(was, now json.RawMessage) (map[string]json.RawMessage, error) {
	var before, after map[string]json.RawMessage
	if err := json.Unmarshal(was, &before); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(now, &after); err != nil {
		return nil, err
	}
	changed := map[string]json.RawMessage{}
	for k, v := range after {
		if !bytes.Equal(before[k], v) {
			changed[k] = v
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			changed[k] = null
		}
	}
	return changed, nil
}

// Mirror is a client's copy of the daemon's view, which each response
// on a subscription brings up to date.
type Mirror struct {
	seq      uint64
	held     bool
	snapshot state.Snapshot
	// emptySlice is, for each keyed list, whether an empty one is held
	// as an empty slice rather than nil, as decoding it whole would.
	emptySlice map[string]bool
}

// Take brings the mirror up to date with a response, and is the view it
// then holds.
func (m *Mirror) Take(r Response) (state.Snapshot, error) {
	switch {
	case r.Snapshot != nil:
		m.seq, m.held, m.snapshot = r.Seq, true, *r.Snapshot
		m.emptySlice = map[string]bool{
			"sessions": r.Snapshot.Sessions != nil && len(r.Snapshot.Sessions) == 0,
			"groups":   r.Snapshot.Groups != nil && len(r.Snapshot.Groups) == 0,
		}
		return m.snapshot, nil
	case r.Delta != nil:
		if !m.held || r.Delta.Base != m.seq {
			m.held = false
			return state.Snapshot{}, ErrResync
		}
		next, err := m.apply(*r.Delta)
		if err != nil {
			m.held = false
			return state.Snapshot{}, fmt.Errorf("%w: %w", ErrResync, err)
		}
		m.seq, m.snapshot = r.Seq, next
		return next, nil
	}
	return state.Snapshot{}, errors.New("response carried no snapshot")
}

func (m *Mirror) apply(d Delta) (state.Snapshot, error) {
	next := m.snapshot
	listed := map[string]bool{}
	for _, l := range lists {
		listed[l.name] = true
		if raw, ok := d.Sections[l.name]; ok {
			m.emptySlice[l.name] = bytes.Equal(raw, empty)
		}
	}
	for k, raw := range d.Sections {
		if listed[k] {
			continue
		}
		if err := patch(&next, snapshotFields, k, raw); err != nil {
			return state.Snapshot{}, err
		}
	}
	for _, l := range lists {
		if err := l.apply(&next, d.Lists[l.name], m.emptySlice[l.name]); err != nil {
			return state.Snapshot{}, err
		}
	}
	return next, nil
}

// applyList is a list as it is after its changes, in a slice of its own
// so the list held before is left as it was.
func applyList[T any](prev []T, id func(T) string, fields map[string]int, l *List) ([]T, error) {
	if l == nil {
		return slices.Clone(prev), nil
	}
	byID := make(map[string]T, len(prev)+len(l.Added))
	order := make([]string, 0, len(prev)+len(l.Added))
	for _, item := range prev {
		byID[id(item)] = item
		order = append(order, id(item))
	}
	for _, k := range l.Removed {
		delete(byID, k)
	}
	order = slices.DeleteFunc(order, func(k string) bool { _, ok := byID[k]; return !ok })
	for k, changed := range l.Changed {
		item, ok := byID[k]
		if !ok {
			return nil, fmt.Errorf("a change to %s, which is not held", k)
		}
		for field, raw := range changed {
			if err := patch(&item, fields, field, raw); err != nil {
				return nil, err
			}
		}
		byID[k] = item
	}
	for _, raw := range l.Added {
		var item T
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		byID[id(item)] = item
		order = append(order, id(item))
	}
	if l.Order != nil {
		order = l.Order
	}
	out := make([]T, 0, len(order))
	for _, k := range order {
		item, ok := byID[k]
		if !ok {
			return nil, fmt.Errorf("%s is in the order but not held", k)
		}
		out = append(out, item)
	}
	return out, nil
}

// patch sets one field, named as it is on the wire, as a whole snapshot
// decoded afresh would have it: null or absent is the zero value.
func patch[T any](v *T, fields map[string]int, key string, raw json.RawMessage) error {
	i, ok := fields[key]
	if !ok {
		return fmt.Errorf("no field %q", key)
	}
	field := reflect.ValueOf(v).Elem().Field(i)
	field.SetZero()
	return json.Unmarshal(raw, field.Addr().Interface())
}

var snapshotFields = wireFields(reflect.TypeFor[state.Snapshot]())

func wireFields(t reflect.Type) map[string]int {
	fields := map[string]int{}
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		switch name {
		case "-":
			continue
		case "":
			name = f.Name
		}
		fields[name] = i
	}
	return fields
}

// omitsEmpty is whether the snapshot's field of that wire name leaves
// an empty value out.
func omitsEmpty(name string) bool {
	f := reflect.TypeFor[state.Snapshot]().Field(snapshotFields[name])
	_, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
	return slices.Contains(strings.Split(opts, ","), "omitempty")
}
