package group

import (
	"regexp"
	"strconv"
	"time"
)

// Mention is a line from one of a group's sites that may name a pull
// request: Site is the site's index, and Command whether it was run.
type Mention struct {
	Site    int
	Text    string
	Command bool
	At      time.Time
}

// TeammatePR is a pull request a group's sessions named but none of
// them opened, so it was opened outside them (item 90). Its state is
// merged or closed only when one of the sessions ran the command that
// did it; words alone never change it.
type TeammatePR struct {
	PR     SitePR
	Closed bool
	// Site is the site that last mentioned it, and Seen when.
	Site int
	Seen time.Time
}

var prURL = regexp.MustCompile(`https?://[^\s/]+/([\w.-]+/[\w.-]+)(?:/-)?/(?:pull|pull-requests|merge_requests)/([0-9]+)`)

// TeammatePRs are the pull requests named in mentions that are not one
// of own, in the order first named.
func TeammatePRs(own []SitePR, mentions []Mention) []TeammatePR {
	known := map[string]bool{}
	for _, pr := range own {
		known[pr.URL] = true
	}
	byURL := map[string]*TeammatePR{}
	var order []string
	for _, m := range mentions {
		for _, match := range prURL.FindAllStringSubmatch(m.Text, -1) {
			url := match[0]
			if known[url] {
				continue
			}
			t, ok := byURL[url]
			if !ok {
				n, _ := strconv.Atoi(match[2])
				t = &TeammatePR{PR: SitePR{Repository: match[1], Number: n, URL: url}, Site: m.Site, Seen: m.At}
				byURL[url] = t
				order = append(order, url)
			}
			if !m.At.Before(t.Seen) {
				t.Site, t.Seen = m.Site, m.At
			}
			if m.Command && ghPR("merge").MatchString(m.Text) {
				t.PR.Merged = true
			}
			if m.Command && ghPR("close").MatchString(m.Text) {
				t.Closed = true
			}
		}
	}
	out := make([]TeammatePR, 0, len(order))
	for _, url := range order {
		out = append(out, *byURL[url])
	}
	return out
}

func ghPR(verb string) *regexp.Regexp {
	return regexp.MustCompile(`\bgh\s+pr\s+` + verb + `\b`)
}
