package group

import (
	"hash/fnv"
	"strings"
)

const (
	pasteMinWords   = 40
	pasteShingle    = 5
	pasteSimilarity = 0.6
)

// SamePaste reports whether two pasted texts are near-duplicates: the
// same plan with a few words changed, compared by overlapping runs of
// words.
func SamePaste(a, b string) bool {
	pa, pb := shingles(a), shingles(b)
	if pa == nil || pb == nil {
		return false
	}
	shared := 0
	for h := range pa {
		if pb[h] {
			shared++
		}
	}
	return float64(shared)/float64(len(pa)+len(pb)-shared) >= pasteSimilarity
}

func shingles(text string) map[uint64]bool {
	words := strings.Fields(strings.ToLower(text))
	if len(words) < pasteMinWords {
		return nil
	}
	set := map[uint64]bool{}
	for i := 0; i+pasteShingle <= len(words); i++ {
		h := fnv.New64a()
		_, _ = h.Write([]byte(strings.Join(words[i:i+pasteShingle], " ")))
		set[h.Sum64()] = true
	}
	return set
}
