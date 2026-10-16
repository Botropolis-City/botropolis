package cli_test

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/find"
)

type fakeFind struct {
	searched []find.Query
	opened   int
}

func (f *fakeFind) Run(_ io.Writer, q find.Query) error {
	f.searched = append(f.searched, q)
	return nil
}

func (f *fakeFind) Open() error {
	f.opened++
	return nil
}

func TestFindCLI(t *testing.T) {
	t.Run("when run with no words", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("find"))

		t.Run("it should open the finder in the city, starting one if none is running", func(t *testing.T) {
			assert.Equal(t, [2]int{1, 0}, [2]int{h.find.opened, len(h.find.searched)})
		})
	})

	t.Run("when run with words", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("find", "-n", "5", "kiln", "overheat"))

		t.Run("it should print matches, as in slice 2", func(t *testing.T) {
			assert.Equal(t, [2]any{0, []find.Query{{Words: "kiln overheat", Limit: 5}}}, [2]any{h.find.opened, h.find.searched})
		})
	})

	t.Run("when its help is shown", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("find", "--help"))
		help := h.out.String()

		for desktop, says := range map[string]string{
			"GNOME": "GNOME: Settings → Keyboard → Keyboard Shortcuts → Custom Shortcuts",
			"KDE":   "KDE: System Settings → Keyboard → Shortcuts → Add New → Command or Script",
			"macOS": "macOS: in Shortcuts, a shortcut with a Run Shell Script action",
			"i3":    `bindsym $mod+k [instance="botropolis"] focus; exec --no-startup-id botropolis find`,
		} {
			t.Run("it should show how to bind it in "+desktop, func(t *testing.T) {
				assert.Contains(t, help, says)
			})
		}
	})
}
