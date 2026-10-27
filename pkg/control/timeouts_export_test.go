package control

import "time"

func ShortenTimeouts(c *Control, d time.Duration) {
	c.timeouts = timeouts{start: d, end: d, list: d}
}
