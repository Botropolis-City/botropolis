package group

import (
	"regexp"
	"sort"
	"strings"
)

var (
	ticketKey    = regexp.MustCompile(`(?:^|[^A-Za-z0-9])([A-Z][A-Z0-9]{1,9}-[0-9]+)\b`)
	changeURL    = regexp.MustCompile(`https?://[^\s/]+/([\w.-]+/[\w.-]+)(?:/-)?/(?:pull|merge_requests)/([0-9]+)`)
	dependsOn    = regexp.MustCompile(`(?im)^[ \t]*Depends-On:[ \t]*(\S[^\r\n]*?)[ \t]*$`)
	notTicketKey = map[string]bool{
		"UTF": true, "SHA": true, "ISO": true, "RFC": true, "HTTP": true, "TLS": true, "SSL": true,
		"AES": true, "RSA": true, "ECMA": true, "IEEE": true, "CP": true, "MD": true, "ES": true,
		"BASE": true, "IPV": true, "UTC": true, "GMT": true, "PEP": true,
	}
)

// References are what a piece of text points at that another session
// could point at too: ticket keys, pull and merge requests, and
// Depends-On lines.
func References(text string) []string {
	seen := map[string]bool{}
	for _, m := range ticketKey.FindAllStringSubmatch(text, -1) {
		if !notTicketKey[m[1][:strings.IndexByte(m[1], '-')]] {
			seen[m[1]] = true
		}
	}
	for _, m := range changeURL.FindAllStringSubmatch(text, -1) {
		seen[m[1]+"#"+m[2]] = true
	}
	for _, m := range dependsOn.FindAllStringSubmatch(text, -1) {
		seen[m[1]] = true
	}
	if len(seen) == 0 {
		return nil
	}
	refs := make([]string, 0, len(seen))
	for r := range seen {
		refs = append(refs, r)
	}
	sort.Strings(refs)
	return refs
}
