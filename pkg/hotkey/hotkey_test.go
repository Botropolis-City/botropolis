package hotkey

import (
	"errors"
	"testing"

	"github.com/jezek/xgb/xproto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
)

func TestCommonMeaning(t *testing.T) {
	for _, c := range []config.Chord{
		{Ctrl: true, Key: "k"},
		{Ctrl: true, Key: "p"},
		{Ctrl: true, Shift: true, Key: "p"},
	} {
		t.Run("when the chord is "+c.String(), func(t *testing.T) {
			t.Run("it should name what it commonly means", func(t *testing.T) {
				assert.NotEmpty(t, CommonMeaning(c))
			})
		})
	}

	t.Run("when the chord is ctrl+alt+f", func(t *testing.T) {
		t.Run("it should name nothing", func(t *testing.T) {
			assert.Empty(t, CommonMeaning(config.Chord{Ctrl: true, Alt: true, Key: "f"}))
		})
	})
}

func TestGrabMasks(t *testing.T) {
	t.Run("when a chord becomes grabs", func(t *testing.T) {
		chord := config.Chord{Ctrl: true, Alt: true, Key: "f"}
		own := uint16(xproto.ModMaskControl | xproto.ModMask1)
		masks := grabMasks(chord, locks{caps: xproto.ModMaskLock, num: xproto.ModMask2, scroll: xproto.ModMask5})

		t.Run("it should grab once for each combination of Caps, Num and Scroll Lock", func(t *testing.T) {
			distinct := map[uint16]bool{}
			for _, m := range masks {
				distinct[m&^own] = true
			}
			assert.Equal(t, [2]int{8, 8}, [2]int{len(masks), len(distinct)})
		})

		t.Run("it should carry the chord's own modifiers in every grab", func(t *testing.T) {
			for _, m := range masks {
				require.Equal(t, own, m&own, "mask %#x", m)
			}
		})

		t.Run("and the key has no keycode on this keyboard", func(t *testing.T) {
			keyboard := keyboardMap{first: 8, perCode: 2, syms: []xproto.Keysym{0x61, 0x41}}
			_, err := keyboard.keycode("f")

			t.Run("it should say so", func(t *testing.T) {
				assert.ErrorContains(t, err, "no f key")
			})
		})
	})

	t.Run("when the keyboard has the key", func(t *testing.T) {
		keyboard := keyboardMap{first: 8, perCode: 2, syms: []xproto.Keysym{0x61, 0x41, 0x66, 0x46}}
		code, err := keyboard.keycode("f")
		require.NoError(t, err)

		t.Run("it should find its keycode", func(t *testing.T) {
			assert.Equal(t, xproto.Keycode(9), code)
		})
	})
}

func TestGrabRefused(t *testing.T) {
	t.Run("when the server refuses a grab with BadAccess", func(t *testing.T) {
		err := grabError(config.Chord{Ctrl: true, Alt: true, Key: "f"}, xproto.AccessError{})

		t.Run("it should say another application holds it", func(t *testing.T) {
			assert.True(t, errors.Is(err, ErrHeld))
		})

		t.Run("it should give the fallback", func(t *testing.T) {
			assert.ErrorContains(t, err, "botropolis find")
		})
	})
}

func TestOpen(t *testing.T) {
	for _, c := range []struct {
		desktop string
		goos    string
		env     map[string]string
	}{
		{"Wayland with no DISPLAY", "linux", map[string]string{"WAYLAND_DISPLAY": "wayland-0"}},
		{"Windows", "windows", map[string]string{"DISPLAY": ":0"}},
	} {
		t.Run("when the desktop is "+c.desktop, func(t *testing.T) {
			_, err := Open(c.goos, func(k string) string { return c.env[k] })

			t.Run("it should say this desktop has no way to register one", func(t *testing.T) {
				assert.True(t, errors.Is(err, ErrUnsupported))
			})

			t.Run("it should give the fallback", func(t *testing.T) {
				assert.ErrorContains(t, err, "botropolis find")
			})
		})
	}
}

func TestRefusedOnMacOS(t *testing.T) {
	for _, c := range []config.Chord{
		{Alt: true, Key: "f"},
		{Shift: true, Key: "f"},
		{Alt: true, Shift: true, Key: "f"},
	} {
		t.Run("when the chord is "+c.String(), func(t *testing.T) {
			err := RefusedOn("darwin", c)

			t.Run("it should refuse it", func(t *testing.T) {
				assert.True(t, errors.Is(err, ErrRefused))
			})

			t.Run("it should say macOS needs Ctrl or Cmd with it", func(t *testing.T) {
				assert.ErrorContains(t, err, "needs ctrl or cmd")
			})
		})
	}

	for _, c := range []config.Chord{
		{Ctrl: true, Alt: true, Key: "f"},
		{Cmd: true, Shift: true, Key: "f"},
		{Ctrl: true, Shift: true, Key: "f"},
	} {
		t.Run("when the chord is "+c.String(), func(t *testing.T) {
			t.Run("it should allow it", func(t *testing.T) {
				assert.NoError(t, RefusedOn("darwin", c))
			})
		})
	}
}
