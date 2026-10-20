// Package fonts embeds the vector faces the chrome is set in: Inter,
// under the SIL Open Font License, shipped as its variable TTF so every
// weight comes from one file; and Go Mono, by Bigelow & Holmes under the
// Go project's BSD licence, for code in the reading pane (item 182).
package fonts

import (
	"embed"

	"golang.org/x/image/font/gofont/gomono"
)

//go:embed inter/InterVariable.ttf inter/LICENSE.txt gomono/LICENSE
var files embed.FS

// Inter is the raw TrueType data of Inter's variable face.
func Inter() ([]byte, error) {
	return files.ReadFile("inter/InterVariable.ttf")
}

// License is Inter's OFL text, for crediting it wherever the font ships.
func License() (string, error) {
	data, err := files.ReadFile("inter/LICENSE.txt")
	return string(data), err
}

// GoMono is the raw TrueType data of Go Mono, as golang.org/x/image
// ships it.
func GoMono() []byte {
	return gomono.TTF
}

// GoMonoLicense is Go Mono's licence, for crediting it wherever the font
// ships.
func GoMonoLicense() (string, error) {
	data, err := files.ReadFile("gomono/LICENSE")
	return string(data), err
}
