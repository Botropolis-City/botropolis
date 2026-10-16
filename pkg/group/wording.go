package group

import (
	_ "embed"
	"math"
	"sort"
	"strings"
	"unicode"
)

const (
	// wordingThreshold is how much rarity two sessions must share: two
	// words each in one session in ten, or one far rarer.
	wordingThreshold = 4.0
	// distinctiveShare is the most sessions a word can appear in and
	// still say something about the two that share it.
	distinctiveShare = 0.2
	// wordingOverlap is how much of what each says two sessions must
	// share.
	wordingOverlap = 0.1
)

//go:embed stopwords/english.txt
var english string

var stopWords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(english + `
	onto will just also please lets use using make sure need now new add get see look like one two done
	less much`) {
		stopWords[w] = true
		stopWords[strings.Split(w, "'")[0]] = true
	}
}

// Wording weighs the words sessions share by how rare each is across the
// whole history, over all of each session's prompts.
type Wording struct {
	words map[string]map[string]bool
	idf   map[string]float64
	// df is how many sessions use each word, for naming, which weighs a
	// word against the sessions outside the group so that it works on a
	// history too small for any word to be rare across the whole of it.
	df map[string]int
}

func NewWording(prompts map[string][]string, muted []string) *Wording {
	mute := map[string]bool{}
	for _, m := range muted {
		mute[strings.ToLower(m)] = true
	}
	w := &Wording{words: map[string]map[string]bool{}, idf: map[string]float64{}, df: map[string]int{}}
	df := w.df
	for id, ps := range prompts {
		set := map[string]bool{}
		for _, p := range ps {
			for _, tok := range tokens(p) {
				if !mute[tok] {
					set[tok] = true
				}
			}
		}
		w.words[id] = set
		for tok := range set {
			df[tok]++
		}
	}
	n := float64(len(prompts))
	for tok, d := range df {
		if float64(d) <= math.Max(2, distinctiveShare*n) {
			w.idf[tok] = math.Log(n / float64(d))
		}
	}
	return w
}

func tokens(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_'
	})
	out := fields[:0]
	for _, f := range fields {
		f = strings.Trim(f, "-_")
		if len(f) < 3 || stopWords[f] || strings.IndexFunc(f, unicode.IsLetter) < 0 {
			continue
		}
		out = append(out, f)
	}
	return out
}

// Shared is the distinctive words two sessions have in common, rarest
// first, and how much they weigh together.
func (w *Wording) Shared(a, b string) ([]string, float64) {
	var shared []string
	score := 0.0
	for tok := range w.words[a] {
		if idf, ok := w.idf[tok]; ok && w.words[b][tok] {
			shared = append(shared, tok)
			score += idf
		}
	}
	w.byRarity(shared)
	return shared, score
}

// Related is whether two sessions share enough rare words, and enough of
// what each says, that a long session does not match everything by
// sheer length.
func (w *Wording) Related(a, b string) bool {
	_, score := w.Shared(a, b)
	return score >= wordingThreshold && w.overlap(a, b) >= wordingOverlap
}

// overlap is the cosine of the two sessions' distinctive words, each
// weighted by its rarity.
func (w *Wording) overlap(a, b string) float64 {
	weight := func(id string) float64 {
		sum := 0.0
		for tok := range w.words[id] {
			sum += w.idf[tok] * w.idf[tok]
		}
		return sum
	}
	shared := 0.0
	for tok := range w.words[a] {
		if w.words[b][tok] {
			shared += w.idf[tok] * w.idf[tok]
		}
	}
	if shared == 0 {
		return 0
	}
	return shared / math.Sqrt(weight(a)*weight(b))
}

// Top is the n most distinctive words across a set of sessions: of the
// words more of them use than of the sessions outside them, those more of
// them use first, then the rarer outside them.
func (w *Wording) Top(ids []string, n int) []string {
	count := map[string]int{}
	for _, id := range ids {
		for tok := range w.words[id] {
			count[tok]++
		}
	}
	outside := len(w.words) - len(ids)
	rarity := map[string]float64{}
	words := make([]string, 0, len(count))
	for tok, c := range count {
		out := w.df[tok] - c
		if outside > 0 && float64(out)/float64(outside) >= float64(c)/float64(len(ids)) {
			continue
		}
		rarity[tok] = math.Log(float64(outside+1) / float64(out+1))
		words = append(words, tok)
	}
	sort.Slice(words, func(i, j int) bool {
		a, b := words[i], words[j]
		if count[a] != count[b] {
			return count[a] > count[b]
		}
		if rarity[a] != rarity[b] {
			return rarity[a] > rarity[b]
		}
		return a < b
	})
	if len(words) > n {
		words = words[:n]
	}
	return words
}

func (w *Wording) byRarity(words []string) {
	sort.Slice(words, func(i, j int) bool { return w.rarer(words[i], words[j]) })
}

func (w *Wording) rarer(a, b string) bool {
	if w.idf[a] != w.idf[b] {
		return w.idf[a] > w.idf[b]
	}
	return a < b
}
