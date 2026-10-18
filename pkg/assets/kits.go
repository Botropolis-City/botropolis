package assets

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"sort"
)

//go:embed kits/*.png kits/*.json kits/*/License.txt
var kitFiles embed.FS

// Headings are the four ways the camera can face, in the order the atlas
// stores them: the map's home heading first, then a quarter turn each.
var Headings = []int{0, 90, 180, 270}

// KitSprite is one piece at one heading on an atlas page: its pixels and
// where the piece's ground origin lands inside them, so the renderer can
// set the sprite on a cell.
type KitSprite struct {
	Page   int
	Rect   image.Rectangle
	Anchor image.Point
	// Panes are a building's window panes as the sprite shows them, in
	// the sprite's own pixels (item 86).
	Panes []Pane
	// Body is one of the city's own buildings' footprint without what
	// stands beside it, in kit units, one to a cell (item 109).
	Body Body
}

// Body is a building's footprint in kit units: its width along x, depth
// along y, how far along x its centre stands from the piece's, and how
// high its roof is at that centre. Zero for a piece the pipeline does
// not model.
type Body struct {
	W, D, X, Roof float64
}

// Pane is one window pane: the corners of the parallelogram it covers,
// top left, top right, bottom right, bottom left, so a pane on either
// face of a building carries that face's slope.
type Pane [4]image.Point

// KitAtlas is every piece the map draws, cut at one zoom level.
type KitAtlas struct {
	Zoom    float64
	Tile    float64
	Pages   []image.Image
	sprites map[string][4]KitSprite
}

// LoadKits reads every atlas cut by tools/render-sprites, lowest zoom
// first.
func LoadKits() ([]KitAtlas, error) {
	entries, err := kitFiles.ReadDir("kits")
	if err != nil {
		return nil, err
	}
	var atlases []KitAtlas
	for _, e := range entries {
		name := e.Name()
		if len(name) < 6 || name[len(name)-5:] != ".json" {
			continue
		}
		atlas, err := loadKitAtlas("kits/" + name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		atlases = append(atlases, atlas)
	}
	sort.Slice(atlases, func(i, j int) bool { return atlases[i].Zoom < atlases[j].Zoom })
	return atlases, nil
}

func loadKitAtlas(path string) (KitAtlas, error) {
	data, err := kitFiles.ReadFile(path)
	if err != nil {
		return KitAtlas{}, err
	}
	var m struct {
		Zoom    float64  `json:"zoom"`
		Tile    float64  `json:"tile"`
		Pages   []string `json:"pages"`
		Sprites map[string]map[string]struct {
			Page  int       `json:"page"`
			X     int       `json:"x"`
			Y     int       `json:"y"`
			W     int       `json:"w"`
			H     int       `json:"h"`
			AX    float64   `json:"ax"`
			AY    float64   `json:"ay"`
			Panes [][8]int  `json:"panes"`
			Body  []float64 `json:"body"`
		} `json:"sprites"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return KitAtlas{}, err
	}
	atlas := KitAtlas{Zoom: m.Zoom, Tile: m.Tile, sprites: map[string][4]KitSprite{}}
	for _, page := range m.Pages {
		raw, err := kitFiles.ReadFile("kits/" + page)
		if err != nil {
			return KitAtlas{}, err
		}
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			return KitAtlas{}, fmt.Errorf("%s: %w", page, err)
		}
		atlas.Pages = append(atlas.Pages, img)
	}
	for name, headings := range m.Sprites {
		var set [4]KitSprite
		for i, heading := range Headings {
			s, ok := headings[fmt.Sprint(heading)]
			if !ok {
				return KitAtlas{}, fmt.Errorf("%s has no heading %d", name, heading)
			}
			set[i] = KitSprite{
				Page:   s.Page,
				Rect:   image.Rect(s.X, s.Y, s.X+s.W, s.Y+s.H),
				Anchor: image.Pt(int(s.AX+0.5), int(s.AY+0.5)),
			}
			if len(s.Body) >= 3 {
				set[i].Body = Body{W: s.Body[0], D: s.Body[1], X: s.Body[2]}
			}
			if len(s.Body) == 4 {
				set[i].Body.Roof = s.Body[3]
			}
			for _, q := range s.Panes {
				set[i].Panes = append(set[i].Panes, Pane{{q[0], q[1]}, {q[2], q[3]}, {q[4], q[5]}, {q[6], q[7]}})
			}
		}
		atlas.sprites[name] = set
	}
	return atlas, nil
}

// Sprite is a piece ("city-kit-commercial/building-a") at a heading in
// degrees, if the atlas has it.
func (a KitAtlas) Sprite(name string, heading int) (KitSprite, bool) {
	set, ok := a.sprites[name]
	if !ok {
		return KitSprite{}, false
	}
	return set[((heading/90)%4+4)%4], true
}

// Names lists every piece in the atlas, sorted.
func (a KitAtlas) Names() []string {
	names := make([]string, 0, len(a.sprites))
	for name := range a.sprites {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// KitLicense is a kit's licence text, for crediting it wherever it ships.
func KitLicense(kit string) (string, error) {
	data, err := kitFiles.ReadFile("kits/" + kit + "/License.txt")
	return string(data), err
}
