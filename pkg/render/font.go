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
	// strong and code are a reply's bold, from Inter's own weight axis,
	// and its code, in Go Mono. Item 182.
	strong map[ui.Size]*text.GoTextFace
	code   map[ui.Size]*text.GoTextFace
}

// strongWeight is bold on Inter's wght axis.
const strongWeight = 700

// codeScale sets Go Mono a little smaller than Inter at the same size,
// since its letters are wider and taller for their point size.
const codeScale = 0.9

func newFaces(th ui.Theme) (*faces, error) {
	data, err := fonts.Inter()
	if err != nil {
		return nil, err
	}
	source, err := text.NewGoTextFaceSource(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	mono, err := text.NewGoTextFaceSource(bytes.NewReader(fonts.GoMono()))
	if err != nil {
		return nil, err
	}
	f := &faces{source: source, by: map[ui.Size]*text.GoTextFace{}, strong: map[ui.Size]*text.GoTextFace{}, code: map[ui.Size]*text.GoTextFace{}}
	for _, size := range []ui.Size{ui.Small, ui.Body, ui.Title, ui.Display} {
		face := &text.GoTextFace{Source: source, Size: th.Pt(size)}
		face.SetFeature(text.MustParseTag("tnum"), 1)
		f.by[size] = face
		strong := &text.GoTextFace{Source: source, Size: th.Pt(size)}
		strong.SetFeature(text.MustParseTag("tnum"), 1)
		strong.SetVariation(text.MustParseTag("wght"), strongWeight)
		f.strong[size] = strong
		f.code[size] = &text.GoTextFace{Source: mono, Size: th.Pt(size) * codeScale}
	}
	return f, nil
}

// Styling is the face a span of a style is set in at a size, and how far
// below the line's top to draw it so it sits on the plain face's
// baseline.
func (f *faces) Styling(size ui.Size, style ui.SpanStyle) (*text.GoTextFace, float64) {
	face := f.by[size]
	switch style {
	case ui.SpanStrong:
		face = f.strong[size]
	case ui.SpanCode:
		face = f.code[size]
	}
	return face, f.by[size].Metrics().HAscent - face.Metrics().HAscent
}

// Styled measures s in its style's face; it satisfies ui.StyleMeasure.
// Every style takes the plain line's height, so a line's height does
// not depend on what is in it.
func (f *faces) Styled(s string, size ui.Size, style ui.SpanStyle) (float64, float64) {
	_, height := f.Measure("", size)
	if s == "" {
		return 0, height
	}
	face, _ := f.Styling(size, style)
	w, _ := text.Measure(s, face, height)
	return w, height
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
