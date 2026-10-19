// Package proc asks the system about other processes (item 130).
package proc

// StartTime is when pid started, in units of the system's own choosing:
// two times compare only on the machine that read them. It is set when
// the process is made, so a pid handed to another process later gives
// another time. It is false for a pid with no process, and on systems
// it cannot ask.
func StartTime(pid int) (uint64, bool) {
	return startTime(pid)
}
