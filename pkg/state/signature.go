package state

import (
	"crypto/sha256"
	"maps"
	"slices"
)

// signatureParts keeps each session's share of the group signature
// while what it is made of has not changed, so a load with 2,000
// sessions does not join, sort and hash every one of them again
// (item 205).
type signatureParts struct {
	parts map[string]*signaturePart
	gen   int
}

type signaturePart struct {
	cwd, branch, team, job string
	resting                bool
	visits                 map[string]int
	messaged               []string
	sum                    [sha256.Size]byte
	gen                    int
}

// signature is groupSignature's, in a form that changes whenever it
// would: each session's part is hashed, and the parts are combined in
// an order of their own, by adding them.
func (p *signatureParts) signature(sessions []Session) [sha256.Size]byte {
	if p.parts == nil {
		p.parts = map[string]*signaturePart{}
	}
	p.gen++
	var total [sha256.Size]byte
	for i := range sessions {
		s := &sessions[i]
		part := p.parts[s.ID]
		if part == nil || part.gen == p.gen || !part.same(s) {
			if part != nil && part.gen == p.gen {
				return groupSignature(sessions)
			}
			part = &signaturePart{cwd: s.CWD, branch: s.Branch, team: s.Team, job: s.Job, resting: s.State == Resting,
				visits: s.Visits, messaged: s.Messaged, sum: sha256.Sum256([]byte(signatureLine(*s)))}
			p.parts[s.ID] = part
		}
		part.gen = p.gen
		addTo(&total, part.sum)
	}
	for id, part := range p.parts {
		if part.gen != p.gen {
			delete(p.parts, id)
		}
	}
	return total
}

func (part *signaturePart) same(s *Session) bool {
	return part.cwd == s.CWD && part.branch == s.Branch && part.team == s.Team && part.job == s.Job &&
		part.resting == (s.State == Resting) && maps.Equal(part.visits, s.Visits) && slices.Equal(part.messaged, s.Messaged)
}

func addTo(total *[sha256.Size]byte, sum [sha256.Size]byte) {
	carry := 0
	for i := len(total) - 1; i >= 0; i-- {
		v := int(total[i]) + int(sum[i]) + carry
		total[i], carry = byte(v), v>>8
	}
}
