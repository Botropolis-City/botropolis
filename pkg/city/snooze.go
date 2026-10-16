package city

import (
	"errors"
	"strings"
	"time"
)

// SnoozeChoice is one of the snooze picker's ready times.
type SnoozeChoice struct {
	Label string
	At    time.Time
}

// eveningAt and morningAt are the snooze picker's ready hours.
const eveningAt, morningAt = 18, 9

// SnoozeChoices are the snooze picker's ready times: an hour from now,
// this evening while it is still to come, and tomorrow morning.
func SnoozeChoices(now time.Time) []SnoozeChoice {
	choices := []SnoozeChoice{{Label: "in an hour", At: now.Add(time.Hour)}}
	evening := at(now, 0, eveningAt, 0)
	if now.Before(evening) {
		choices = append(choices, SnoozeChoice{Label: "this evening at 18:00", At: evening})
	}
	return append(choices, SnoozeChoice{Label: "tomorrow at 09:00", At: at(now, 1, morningAt, 0)})
}

func at(now time.Time, days, hour, minute int) time.Time {
	y, m, d := now.Date()
	return time.Date(y, m, d+days, hour, minute, 0, 0, now.Location())
}

// errSnoozeFormat says what a snooze time may be written as.
var errSnoozeFormat = errors.New("write a time like 18:00, tomorrow 9:00, in 2h or 2026-10-20 9:00")

// ParseSnooze reads a typed snooze time: a time today ("18:00"),
// tomorrow ("tomorrow 9:00"), a while from now ("in 2h", "in 1h30m"),
// or a date and time ("2026-10-20 9:00"). A time that is not later
// than now is refused, with the reason.
func ParseSnooze(text string, now time.Time) (time.Time, error) {
	text = strings.ToLower(strings.TrimSpace(text))
	var t time.Time
	switch {
	case strings.HasPrefix(text, "in "):
		d, err := time.ParseDuration(strings.TrimSpace(strings.TrimPrefix(text, "in ")))
		if err != nil {
			return time.Time{}, errSnoozeFormat
		}
		t = now.Add(d)
	case strings.HasPrefix(text, "tomorrow "):
		h, m, err := clock(strings.TrimPrefix(text, "tomorrow "))
		if err != nil {
			return time.Time{}, err
		}
		t = at(now, 1, h, m)
	case strings.Contains(text, "-"):
		day, err := time.ParseInLocation("2006-01-02 15:04", text, now.Location())
		if err != nil {
			if day, err = time.ParseInLocation("2006-01-02 3:04", text, now.Location()); err != nil {
				return time.Time{}, errSnoozeFormat
			}
		}
		t = day
	default:
		h, m, err := clock(text)
		if err != nil {
			return time.Time{}, err
		}
		t = at(now, 0, h, m)
	}
	if !t.After(now) {
		return time.Time{}, errors.New("that time has passed; write a later one, or tomorrow 9:00")
	}
	return t, nil
}

// clock reads "9:30" or "09:30" as an hour and a minute.
func clock(text string) (int, int, error) {
	t, err := time.Parse("15:04", strings.TrimSpace(text))
	if err != nil {
		if t, err = time.Parse("3:04", strings.TrimSpace(text)); err != nil {
			return 0, 0, errSnoozeFormat
		}
	}
	return t.Hour(), t.Minute(), nil
}
