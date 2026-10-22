package helpers

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"
)

// HistoryPrompt is one line of a synthetic prompt history.
type HistoryPrompt struct {
	Session string
	Dir     string
	Text    string
	At      time.Time
}

var commonWords = strings.Fields(`build tests failing parser config output server client cache logger
	handler request response module package release deploy script error branch commit review docs
	refactor layout render styles schema migration`)

// SyntheticHistory is a prompt history made up from a seed, in time
// order: sessions working on a dozen topics across a dozen directories,
// each topic with words of its own and some with a ticket key, among
// words every session uses and words only one does. It stands in for a
// real history, which is never copied.
func SyntheticHistory(seed int64, sessions, prompts int, start time.Time) []HistoryPrompt {
	r := rand.New(rand.NewSource(seed))
	var out []HistoryPrompt
	at := start
	for s := 0; s < sessions; s++ {
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", s)
		dir := fmt.Sprintf("/w/repo%02d", r.Intn(12))
		topic := r.Intn(14)
		if r.Intn(5) > 0 {
			at = at.Add(time.Duration(30+r.Intn(240)) * time.Minute)
		} else {
			at = at.Add(time.Duration(r.Intn(50)) * time.Second)
		}
		when := at
		for p := 0; p < prompts; p++ {
			words := []string{commonWords[r.Intn(len(commonWords))], commonWords[r.Intn(len(commonWords))]}
			if topic < 12 {
				words = append(words, fmt.Sprintf("topic%cword%c", 'a'+topic, 'a'+r.Intn(6)))
				if r.Intn(3) == 0 {
					words = append(words, fmt.Sprintf("topic%cword%c", 'a'+topic, 'a'+r.Intn(6)))
				}
				if topic%4 == 0 && r.Intn(8) == 0 {
					words = append(words, fmt.Sprintf("PROJ-%d", 100+topic))
				}
			}
			if r.Intn(4) == 0 {
				words = append(words, fmt.Sprintf("only%dx%d", s, p))
			}
			out = append(out, HistoryPrompt{Session: id, Dir: dir, Text: "please " + strings.Join(words, " "), At: when})
			when = when.Add(time.Duration(1+r.Intn(20)) * time.Minute)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}

// ManySessions is a heavy user's prompt history made up from a seed, in
// time order: n small sessions started across the 30 days before end,
// about one topic for every eight sessions and one directory for every
// twenty, so that what any one session shares grows with its own work
// and not with the history's length (item 198).
func ManySessions(seed int64, n int, end time.Time) []HistoryPrompt {
	r := rand.New(rand.NewSource(seed))
	topics, dirs := max(1, n/8), max(1, n/20)
	span := 29 * 24 * time.Hour
	start := end.Add(-span)
	var out []HistoryPrompt
	for s := 0; s < n; s++ {
		id := fmt.Sprintf("00000000-0000-4198-8000-%012d", s)
		dir := fmt.Sprintf("/w/many%03d", r.Intn(dirs))
		topic := r.Intn(topics)
		when := start.Add(time.Duration(s) * span / time.Duration(n+1)).Add(time.Duration(r.Intn(600)) * time.Second)
		prompts := 2 + r.Intn(4)
		for p := 0; p < prompts; p++ {
			words := []string{commonWords[r.Intn(len(commonWords))], commonWords[r.Intn(len(commonWords))],
				fmt.Sprintf("many%dtopic%c", topic, 'a'+r.Intn(4))}
			if r.Intn(10) == 0 {
				words = append(words, fmt.Sprintf("MANY-%d", topic))
			}
			if r.Intn(4) == 0 {
				words = append(words, fmt.Sprintf("only%dx%d", s, p))
			}
			out = append(out, HistoryPrompt{Session: id, Dir: dir, Text: "please " + strings.Join(words, " "), At: when})
			when = when.Add(time.Duration(1+r.Intn(5)) * time.Minute)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}
