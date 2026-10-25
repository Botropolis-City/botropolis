package hotkey

import (
	"fmt"
	"strconv"
	"strings"

	"botropolis.city/botropolis/pkg/config"
)

// Carbon's modifier bits for RegisterEventHotKey, from Events.h. Item 95.
const (
	carbonCmd     = 1 << 8
	carbonShift   = 1 << 9
	carbonOption  = 1 << 11
	carbonControl = 1 << 12
)

// carbonKey is a chord as RegisterEventHotKey takes it: a virtual key
// code and modifier bits.
type carbonKey struct {
	code uint32
	mods uint32
}

func carbonChord(c config.Chord) (carbonKey, error) {
	code, ok := virtualKey(c.Key)
	if !ok {
		return carbonKey{}, fmt.Errorf("%s: %s is not a key a global shortcut can use on macOS", c, c.Key)
	}
	k := carbonKey{code: code}
	for _, m := range []struct {
		held bool
		bit  uint32
	}{{c.Cmd, carbonCmd}, {c.Shift, carbonShift}, {c.Alt, carbonOption}, {c.Ctrl, carbonControl}} {
		if m.held {
			k.mods |= m.bit
		}
	}
	return k, nil
}

// virtualKeys are the kVK_ codes of HIToolbox's Events.h, which name
// positions on an ANSI keyboard rather than characters.
var virtualKeys = map[string]uint32{
	"a": 0x00, "s": 0x01, "d": 0x02, "f": 0x03, "h": 0x04, "g": 0x05, "z": 0x06, "x": 0x07,
	"c": 0x08, "v": 0x09, "b": 0x0b, "q": 0x0c, "w": 0x0d, "e": 0x0e, "r": 0x0f, "y": 0x10,
	"t": 0x11, "digit1": 0x12, "digit2": 0x13, "digit3": 0x14, "digit4": 0x15, "digit6": 0x16,
	"digit5": 0x17, "equal": 0x18, "digit9": 0x19, "digit7": 0x1a, "minus": 0x1b, "digit8": 0x1c,
	"digit0": 0x1d, "bracketright": 0x1e, "o": 0x1f, "u": 0x20, "bracketleft": 0x21, "i": 0x22,
	"p": 0x23, "enter": 0x24, "l": 0x25, "j": 0x26, "quote": 0x27, "k": 0x28, "semicolon": 0x29,
	"backslash": 0x2a, "comma": 0x2b, "slash": 0x2c, "n": 0x2d, "m": 0x2e, "period": 0x2f,
	"tab": 0x30, "space": 0x31, "backquote": 0x32, "backspace": 0x33, "escape": 0x35,
	"numpaddecimal": 0x41, "numpadmultiply": 0x43, "numpadadd": 0x45, "numpaddivide": 0x4b,
	"numpadenter": 0x4c, "numpadsubtract": 0x4e,
	"numpad0": 0x52, "numpad1": 0x53, "numpad2": 0x54, "numpad3": 0x55, "numpad4": 0x56,
	"numpad5": 0x57, "numpad6": 0x58, "numpad7": 0x59, "numpad8": 0x5b, "numpad9": 0x5c,
	"insert": 0x72, "home": 0x73, "pageup": 0x74, "delete": 0x75, "end": 0x77, "pagedown": 0x79,
	"arrowleft": 0x7b, "arrowright": 0x7c, "arrowdown": 0x7d, "arrowup": 0x7e,
}

var functionKeys = []uint32{
	0x7a, 0x78, 0x63, 0x76, 0x60, 0x61, 0x62, 0x64, 0x65, 0x6d,
	0x67, 0x6f, 0x69, 0x6b, 0x71, 0x6a, 0x40, 0x4f, 0x50, 0x5a,
}

func virtualKey(name string) (uint32, bool) {
	name = strings.ToLower(name)
	if code, ok := virtualKeys[name]; ok {
		return code, true
	}
	if rest, ok := strings.CutPrefix(name, "f"); ok {
		if n, err := strconv.Atoi(rest); err == nil && n >= 1 && n <= len(functionKeys) {
			return functionKeys[n-1], true
		}
	}
	return 0, false
}
