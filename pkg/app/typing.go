package app

import (
	"fmt"
	"io"

	"botropolis.city/botropolis/pkg/city"
)

// typingFor is how new districts are given a type. A value the city does
// not know is random, and says so once (item 109).
func typingFor(value string, log io.Writer) city.Typing {
	typing, ok := city.ParseTyping(value)
	if !ok {
		_, _ = fmt.Fprintf(log, "botropolis: unknown district_type %q; new districts get a random type\n", value)
	}
	return typing
}
