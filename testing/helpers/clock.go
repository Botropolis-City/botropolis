package helpers

import (
	"fmt"
	"time"
)

// ZoneAt is a TZ under which now reads hour o'clock, so a city run under
// it is in a chosen light whatever the box's clock says. Go reads TZ as
// a zone name, not a POSIX rule, so it is one of the fixed Etc/GMT
// zones, whose sign is the reverse of the offset. Item 308.
func ZoneAt(hour int, now time.Time) string {
	offset := ((hour-now.UTC().Hour())%24 + 24) % 24
	if offset > 12 {
		offset -= 24
	}
	return fmt.Sprintf("Etc/GMT%+d", -offset)
}
