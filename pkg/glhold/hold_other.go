//go:build !linux && !freebsd

package glhold

// Hold does nothing where the city does not draw through Mesa's GL.
func Hold(...string) {}
