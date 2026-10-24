package demo

import (
	"encoding/json"
	"fmt"
	"time"
)

// recordedTurns is what a recording's main-line turns say about how its
// session works: its model, its context at the first and last turn,
// what a turn writes to cache on average, and what a turn that only
// calls tools writes out, since hours of work are mostly those, and the
// recording's answers and thinking would pull a plain mean far above
// them.
type recordedTurns struct {
	model                      string
	firstRead, lastContext     int64
	input, cacheCreate, output int64
}

type recordedTurn struct {
	input, cacheCreate, output int64
	tool, other                bool
}

func turnsOf(lines [][]byte) (recordedTurns, bool) {
	var r recordedTurns
	byID := map[string]*recordedTurn{}
	var turns []*recordedTurn
	for _, line := range lines {
		var rec struct {
			Type        string `json:"type"`
			IsSidechain bool   `json:"isSidechain"`
			Message     struct {
				ID      string `json:"id"`
				Model   string `json:"model"`
				Content []struct {
					Type string `json:"type"`
				} `json:"content"`
				Usage *struct {
					Input       int64 `json:"input_tokens"`
					CacheCreate int64 `json:"cache_creation_input_tokens"`
					CacheRead   int64 `json:"cache_read_input_tokens"`
					Output      int64 `json:"output_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &rec) != nil || rec.Type != "assistant" || rec.IsSidechain || rec.Message.Usage == nil {
			continue
		}
		u := rec.Message.Usage
		r.lastContext = u.Input + u.CacheCreate + u.CacheRead
		if rec.Message.Model != "" {
			r.model = rec.Message.Model
		}
		t := byID[rec.Message.ID]
		if t == nil || rec.Message.ID == "" {
			if len(turns) == 0 {
				r.firstRead = u.CacheRead
			}
			t = &recordedTurn{input: u.Input, cacheCreate: u.CacheCreate, output: u.Output}
			turns = append(turns, t)
			byID[rec.Message.ID] = t
		}
		for _, block := range rec.Message.Content {
			t.tool = t.tool || block.Type == "tool_use"
			t.other = t.other || block.Type != "tool_use"
		}
	}
	if len(turns) == 0 {
		return r, false
	}
	var tools, said int64
	for _, t := range turns {
		r.input += t.input
		r.cacheCreate += t.cacheCreate
		if t.tool && !t.other {
			tools++
			said += t.output
		}
	}
	n := int64(len(turns))
	r.input, r.cacheCreate = r.input/n, r.cacheCreate/n
	if tools == 0 {
		for _, t := range turns {
			said += t.output
		}
		tools = n
	}
	r.output = said / tools
	return r, true
}

// fillHours puts a turn every pace between a transcript's opening, the
// first opened lines, and the recording that follows, so a session
// opened hours ago holds hours of work (item 240). Each turn reads its
// context from cache, growing from the recording's first read to its
// last context, writes what recordedTurns says a turn writes, and
// says nothing: no text and no tool call, so no card, prompt or group
// signal changes.
func fillHours(lines [][]byte, opened int, pace time.Duration, id, cwd string) ([][]byte, error) {
	if pace <= 0 || opened >= len(lines) {
		return lines, nil
	}
	recorded, ok := turnsOf(lines)
	if !ok {
		return lines, nil
	}
	begin := earliest(lines[:opened])
	until := earliest(lines[opened:])
	steps := int64(until.Sub(begin) / pace)
	if until.Sub(begin)%pace == 0 {
		steps--
	}
	if steps <= 0 {
		return lines, nil
	}
	top := max(recorded.lastContext-recorded.input-recorded.cacheCreate, recorded.firstRead)
	fill := make([][]byte, 0, steps)
	for k := int64(1); k <= steps; k++ {
		read := recorded.firstRead + (top-recorded.firstRead)*k/steps
		data, err := json.Marshal(map[string]any{
			"type":        "assistant",
			"sessionId":   id,
			"cwd":         cwd,
			"isSidechain": false,
			"timestamp":   begin.Add(time.Duration(k) * pace).UTC().Format("2006-01-02T15:04:05.000Z07:00"),
			"message": map[string]any{
				"id":      fmt.Sprintf("msg_demo_%s_%d", id, k),
				"type":    "message",
				"role":    "assistant",
				"model":   recorded.model,
				"content": []any{},
				"usage": map[string]any{
					"input_tokens":                recorded.input,
					"cache_creation_input_tokens": recorded.cacheCreate,
					"cache_read_input_tokens":     read,
					"output_tokens":               recorded.output,
				},
			},
		})
		if err != nil {
			return nil, err
		}
		fill = append(fill, data)
	}
	out := make([][]byte, 0, len(lines)+len(fill))
	out = append(out, lines[:opened]...)
	out = append(out, fill...)
	return append(out, lines[opened:]...), nil
}
