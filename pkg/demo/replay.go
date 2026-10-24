package demo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// pendingLine is a line of a replayed recording and how far through
// the recording it falls, from 0 to 1.
type pendingLine struct {
	line []byte
	at   float64
}

// replayStep is how often a replay reveals what is due: often enough
// that a building grows in steps the eye follows, rarely enough that
// the city's two-second poll sees several at once rather than each.
const replayStep = 500 * time.Millisecond

// splitReplay divides a recording into what the session holds the
// moment it arrives -- its opening prompt and its title -- and the rest,
// each line placed by its own timestamp between the first and the last.
// A line with no timestamp travels with the one before it.
func splitReplay(lines [][]byte) (head [][]byte, rest []pendingLine) {
	first, last := earliest(lines), time.Time{}
	if l, ok := Latest(lines); ok {
		last = l
	}
	span := last.Sub(first)
	at := 0.0
	for _, line := range lines {
		if ts, ok := Latest([][]byte{line}); ok && span > 0 {
			at = float64(ts.Sub(first)) / float64(span)
		}
		if at == 0 || isTitle(line) {
			head = append(head, line)
			continue
		}
		rest = append(rest, pendingLine{line: line, at: at})
	}
	return head, rest
}

func isTitle(line []byte) bool {
	return bytes.Contains(line, []byte(`"type":"custom-title"`)) || bytes.Contains(line, []byte(`"type":"ai-title"`))
}

// scaleContext scales every main-line usage so the last reaches the
// target share of the window and the rest keep their proportions: a
// replayed session's context fills as it goes on.
func scaleContext(lines [][]byte, target string) ([][]byte, error) {
	pct, err := strconv.Atoi(strings.TrimSuffix(target, "%"))
	if err != nil || !strings.HasSuffix(target, "%") || pct <= 0 || pct >= 100 {
		return nil, fmt.Errorf("context %q: want a percentage between 1%% and 99%%", target)
	}
	type usageAt struct {
		i    int
		rec  map[string]any
		u    map[string]any
		ctx  int64
		rest int64
	}
	var found []usageAt
	for i, line := range lines {
		v, err := decode(line)
		if err != nil {
			continue
		}
		rec, _ := v.(map[string]any)
		if rec["type"] != "assistant" || rec["isSidechain"] == true {
			continue
		}
		msg, _ := rec["message"].(map[string]any)
		u, _ := msg["usage"].(map[string]any)
		if u == nil {
			continue
		}
		num := func(k string) int64 {
			n, _ := u[k].(json.Number).Int64()
			return n
		}
		rest := num("input_tokens") + num("cache_creation_input_tokens")
		found = append(found, usageAt{i: i, rec: rec, u: u, ctx: rest + num("cache_read_input_tokens"), rest: rest})
	}
	if len(found) == 0 || found[len(found)-1].ctx == 0 {
		return nil, fmt.Errorf("no main-line usage to scale")
	}
	k := float64(int64(pct)*contextWindow/100) / float64(found[len(found)-1].ctx)
	out := append([][]byte(nil), lines...)
	for _, f := range found {
		f.u["cache_read_input_tokens"] = max(int64(float64(f.ctx)*k+0.5)-f.rest, 0)
		data, err := json.Marshal(f.rec)
		if err != nil {
			return nil, err
		}
		out[f.i] = data
	}
	return out, nil
}
