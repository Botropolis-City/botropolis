package proc

import (
	"bytes"
	"os"
	"strconv"
	"strings"
)

func startTime(pid int) (uint64, bool) {
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, false
	}
	return parseStat(stat)
}

// parseStat reads field 22 of a process's stat, in clock ticks since
// boot. Fields are counted from after the name, which can hold spaces
// and parentheses.
func parseStat(stat []byte) (uint64, bool) {
	shut := bytes.LastIndexByte(stat, ')')
	if shut < 0 {
		return 0, false
	}
	fields := strings.Fields(string(stat[shut+1:]))
	if len(fields) < 20 {
		return 0, false
	}
	started, err := strconv.ParseUint(fields[19], 10, 64)
	return started, err == nil
}
