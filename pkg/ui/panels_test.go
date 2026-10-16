package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/ui"
)

func TestPanelsSayWhatTheyAre(t *testing.T) {
	th := ui.NewTheme(1)
	const w, h = 1200.0, 800.0
	panels := map[string]func() (title, subtitle ui.Text){
		ui.PanelLenses: func() (ui.Text, ui.Text) {
			p := ui.LayoutViewKey(th, w, h, city.LensAttention, measure7)
			return p.Title, p.Subtitle
		},
		ui.PanelPower: func() (ui.Text, ui.Text) {
			p := ui.LayoutBreakdown(th, w, h, city.Breakdown{}, nil, nil, measure7)
			return p.Title, p.Subtitle
		},
		ui.PanelTimeline: func() (ui.Text, ui.Text) {
			p := ui.LayoutTimeline(th, w, h, "Timeline", nil, 0, measure7)
			return p.Title, p.Subtitle
		},
		ui.PanelSettings: func() (ui.Text, ui.Text) {
			p := ui.LayoutSettings(th, w, h, ui.Settings{}, measure7)
			return p.Title, p.Subtitle
		},
		ui.PanelReader: func() (ui.Text, ui.Text) {
			p := ui.LayoutReader(th, w, h, ui.ReaderInput{Title: "a session"}, measure7)
			return p.Title, p.Subtitle
		},
	}
	for name, layout := range panels {
		t.Run("when it is "+name, func(t *testing.T) {
			title, subtitle := layout()

			t.Run("it should put its one line under its title", func(t *testing.T) {
				assert.True(t, subtitle.Text == ui.PanelAbout[name] && subtitle.Text != "" && subtitle.At.Y > title.At.Y && subtitle.Size == ui.Small, "%+v", subtitle)
			})
		})
	}
}
