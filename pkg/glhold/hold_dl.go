//go:build linux || freebsd

package glhold

import "github.com/ebitengine/purego"

// mode opens a library only if it is already loaded, and marks it never
// to be unloaded. The handle is kept open for the life of the process.
const mode = purego.RTLD_NOW | rtldNoload | rtldNodelete

type opener func(name string, mode int) (uintptr, error)

// Hold pins each of names that is already loaded. One that is not is
// left unloaded: an EGL run never loads GLX for this.
func Hold(names ...string) { hold(purego.Dlopen, names) }

func hold(open opener, names []string) {
	for _, name := range names {
		_, _ = open(name, mode)
	}
}
