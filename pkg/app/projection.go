package app

import (
	"fmt"
	"io"

	"botropolis.city/botropolis/pkg/city"
)

// projectionFor is the projection the map opens in. The top-down one is
// no longer offered: a config that still asks for it opens the isometric
// map and says so once (item 106).
func projectionFor(name string, log io.Writer) (city.Projection, error) {
	p, warning, ok := city.UserProjection(name)
	if !ok {
		return p, fmt.Errorf("unknown projection %q: use iso", name)
	}
	if warning != "" {
		_, _ = fmt.Fprintf(log, "botropolis: %s\n", warning)
	}
	return p, nil
}
