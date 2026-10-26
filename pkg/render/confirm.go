package render

import "time"

// ConfirmWithin is how long a quit prompt waits for its answer.
const ConfirmWithin = 10 * time.Second

// confirmUpdates is how many updates a quit prompt also waits through
// before it stands down: a loop held by a slow frame or snapshot reads a
// y pressed at once only after the clock has run, so the clock alone
// would stand the prompt down under it and the city would not quit
// (item 270).
const confirmUpdates = 100

// confirm is the guard on quitting: Escape or q asks, only y or Enter
// within the window confirms, and anything else — Escape again, any
// other key, or the clock — stands it down. Two Escapes never quit.
type confirm struct {
	askedAt time.Time
	updates int
}

// tick counts an update of the loop while the prompt is up.
func (c *confirm) tick() {
	if !c.askedAt.IsZero() {
		c.updates++
	}
}

func (c *confirm) ask(now time.Time) {
	c.askedAt, c.updates = now, 0
}

// confirm reports whether an answer of yes arrives while the prompt is
// up, and stands the prompt down either way.
func (c *confirm) confirm(now time.Time) bool {
	armed := c.armed(now)
	c.askedAt = time.Time{}
	return armed
}

func (c *confirm) cancel() {
	c.askedAt = time.Time{}
}

// armed reports whether the prompt is still waiting for an answer: it
// stands down once the window has passed and the loop has run its
// updates through it.
func (c *confirm) armed(now time.Time) bool {
	if c.askedAt.IsZero() {
		return false
	}
	if now.Sub(c.askedAt) > ConfirmWithin && c.updates >= confirmUpdates {
		c.askedAt = time.Time{}
		return false
	}
	return true
}
