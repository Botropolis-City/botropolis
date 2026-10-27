package render

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"unicode"

	"botropolis.city/botropolis/pkg/config"
)

// pasting is a paste on its way: who asked for it, whether the
// clipboard is still being read, and the text once it has come back.
type pasting struct {
	mu     sync.Mutex
	holder any
	asking bool
	ready  bool
	text   string
}

// pasteChords are the chords that paste on goos: Ctrl+V and
// Shift+Insert on Linux, ⌘V on macOS.
func pasteChords(goos string) []config.Chord {
	if goos == "darwin" {
		return []config.Chord{{Cmd: true, Key: "v"}}
	}
	return []config.Chord{{Ctrl: true, Key: "v"}, {Shift: true, Key: "insert"}}
}

func (g *Game) pastePressed() bool {
	for _, c := range pasteChords(runtime.GOOS) {
		if g.chordPressed(c) && g.onlyHeld(c) {
			return true
		}
	}
	return false
}

// pasted is the one way a paste reaches a text field (item 290): each
// field's holder calls it every frame it has the keys. The paste chord
// starts reading the clipboard beside the loop, so a tool that hangs
// cannot stop the map; the text it brings back goes to the holder that
// asked, on a later frame, and to no other.
func (g *Game) pasted(holder any) (string, bool) {
	p := &g.pasting
	p.mu.Lock()
	if p.ready {
		mine := p.holder == holder
		text := p.text
		p.ready, p.holder, p.text = false, nil, ""
		if mine {
			p.mu.Unlock()
			return pasteLine(text), true
		}
	}
	if !g.pastePressed() || p.asking {
		p.mu.Unlock()
		return "", false
	}
	if g.paste == nil {
		p.mu.Unlock()
		g.SetStatus("pasting is not available here")
		return "", false
	}
	p.asking, p.holder = true, holder
	p.mu.Unlock()
	read := g.paste
	g.acting.Add(1)
	go func() {
		defer g.acting.Done()
		text, err := read(context.Background())
		p.mu.Lock()
		p.asking = false
		if err == nil {
			p.ready, p.text = true, text
		} else {
			p.holder = nil
		}
		p.mu.Unlock()
		if err != nil {
			g.SetStatus(err.Error())
		}
	}()
	return "", false
}

// pasteLine is the clipboard as one line, since every field is one:
// a trailing newline is dropped, the other line breaks and tabs become
// spaces, and other control characters go.
func pasteLine(s string) string {
	s = strings.TrimSuffix(strings.TrimSuffix(s, "\n"), "\r")
	s = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\t", " ").Replace(s)
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

// clipboardFor is what reads the clipboard for a city: nothing for one
// that a script drives, films or photographs, since whatever was last
// copied could end up in a published frame (item 290).
func clipboardFor(opts Options) func(context.Context) (string, error) {
	if opts.Screenshot != "" || opts.Record != "" || len(opts.Keys) > 0 || len(opts.Script) > 0 {
		return nil
	}
	return opts.Paste
}
