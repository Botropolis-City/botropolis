package ui

import "botropolis.city/botropolis/pkg/city"

// Setting is one row of the settings panel: a config key, its label,
// the values it can take and the one it has.
type Setting struct {
	Key     string
	Label   string
	Options []string
	Value   string
}

// Settings is the panel's state: the rows and which one is selected.
type Settings struct {
	Items  []Setting
	Cursor int
}

func NewSettings(items []Setting) Settings {
	return Settings{Items: items}
}

// Move steps the cursor, stopping at either end.
func (s *Settings) Move(delta int) {
	s.Cursor += delta
	if s.Cursor < 0 {
		s.Cursor = 0
	}
	if s.Cursor >= len(s.Items) {
		s.Cursor = len(s.Items) - 1
	}
}

// Adjust cycles the selected row's value through its options, wrapping at
// both ends, and returns the row as it now is. A value that is not among
// the options steps to the first one.
func (s *Settings) Adjust(delta int) Setting {
	if len(s.Items) == 0 {
		return Setting{}
	}
	item := &s.Items[s.Cursor]
	if len(item.Options) == 0 {
		return *item
	}
	at := -1
	for i, o := range item.Options {
		if o == item.Value {
			at = i
		}
	}
	if at < 0 {
		item.Value = item.Options[0]
		return *item
	}
	n := len(item.Options)
	item.Value = item.Options[((at+delta)%n+n)%n]
	return *item
}

// SettingRow is one laid-out row: label on the left, value in a column.
type SettingRow struct {
	Setting
	Label Text
	Value Text
	// Row is the band the whole row occupies, so it can be clicked as
	// well as stepped with the keyboard. Bug 43.
	Row      city.Rect
	Selected bool
}

// SettingsPanel is the settings laid out on a centred panel.
type SettingsPanel struct {
	Rect  city.Rect
	Title Text
	Rows  []SettingRow
}

// Hit finds the setting under a screen point and reports its index, so
// a click can move the cursor to a row and step it the way the
// keyboard does.
func (p SettingsPanel) Hit(at city.Point) (int, bool) {
	for i, r := range p.Rows {
		if r.Row.Contains(at) {
			return i, true
		}
	}
	return 0, false
}

const settingsTitle = "Settings"

// LayoutSettings centres the panel in a width×height window, with the
// selected row's value in chevrons to show it can be stepped.
func LayoutSettings(th Theme, width, height float64, s Settings, measure Measure) SettingsPanel {
	grid := th.Grid()
	pad := 3 * grid
	_, lineH := measure("", Body)
	_, titleH := measure(settingsTitle, Title)
	rowStep := lineH + grid + grid/2
	var labelW, valueW float64
	values := make([]string, len(s.Items))
	for i, item := range s.Items {
		if w, _ := measure(item.Label, Body); w > labelW {
			labelW = w
		}
		values[i] = item.Value
		if i == s.Cursor {
			values[i] = "‹ " + item.Value + " ›"
		}
		if w, _ := measure(values[i], Body); w > valueW {
			valueW = w
		}
	}
	w := pad + labelW + pad + valueW + pad
	h := pad + titleH + grid + rowStep*float64(len(s.Items)) - grid/2 + pad
	p := SettingsPanel{Rect: city.RectAt((width-w)/2, (height-h)/2, w, h)}
	p.Title = Text{Text: settingsTitle, At: p.Rect.Min.Add(city.Point{X: pad, Y: pad}), Size: Title}
	y := p.Rect.Min.Y + pad + titleH + grid
	for i, item := range s.Items {
		p.Rows = append(p.Rows, SettingRow{
			Setting:  item,
			Label:    Text{Text: item.Label, At: city.Point{X: p.Rect.Min.X + pad, Y: y}, Size: Body},
			Value:    Text{Text: values[i], At: city.Point{X: p.Rect.Min.X + pad + labelW + pad, Y: y}, Size: Body},
			Row:      city.RectAt(p.Rect.Min.X, y, p.Rect.Width(), rowStep),
			Selected: i == s.Cursor,
		})
		y += rowStep
	}
	return p
}
