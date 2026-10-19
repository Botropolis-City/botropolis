package statedir

import (
	"errors"
	"fmt"
)

// ErrNewer is what every store returns for a file a newer Botropolis
// wrote in a shape this build cannot read: refused by name, never reset
// (item 149).
var ErrNewer = errors.New("written by a newer Botropolis")

// NewerError is a file of a version this build cannot read.
type NewerError struct {
	Path    string
	Kind    string
	Version int
	Ours    int
}

func (e NewerError) Error() string {
	return fmt.Sprintf("%s is %s version %d, which this build's version %d cannot read: update Botropolis", e.Path, e.Kind, e.Version, e.Ours)
}

func (e NewerError) Is(target error) bool { return target == ErrNewer }
