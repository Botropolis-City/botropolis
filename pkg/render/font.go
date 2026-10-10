package render

import (
	"bytes"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"botropolis.city/botropolis/pkg/assets/fonts"
	"botropolis.city/botropolis/pkg/ui"
)

// faces is the embedded vector face at each of the theme's four sizes,
// already scaled to the display.
type faces struct {
	source *text.GoTextFaceSource
	by     map[ui.Size]*text.GoTextFace
}

func newFaces(th ui.Theme) (*faces, error) {
	data, err := fonts.Inter()
	if err != nil {
		return nil, err
	}
	source, err := text.NewGoTextFaceSource(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	f := &faces{source: source, by: map[ui.Size]*text.GoTextFace{}}
	for _, size := range []ui.Size{ui.Small, ui.Body, ui.Title, ui.Display} {
		f.by[size] = &text.GoTextFace{Source: source, Size: th.Pt(size)}
	}
	return f, nil
}

func (f *faces) Face(size ui.Size) *text.GoTextFace {
	return f.by[size]
}

// Measure is the width and line height of s at a size; it satisfies
// ui.Measure so the layouts can size themselves.
func (f *faces) Measure(s string, size ui.Size) (float64, float64) {
	face := f.Face(size)
	m := face.Metrics()
	height := m.HAscent + m.HDescent
	if s == "" {
		return 0, height
	}
	w, _ := text.Measure(s, face, height)
	return w, height
}
