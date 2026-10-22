package group

// readTexts is what was found in a list of texts, kept so that when the
// list grows only the texts added are read; a list whose earlier texts
// changed is read again whole (item 158). Found is never changed once
// made, since an earlier run may still hold it.
type readTexts struct {
	texts []string
	found map[string]bool
}

func (r readTexts) next(texts []string, read func(string) []string) readTexts {
	if r.found != nil && len(r.texts) == len(texts) && isPrefix(r.texts, texts) {
		return r
	}
	if !isPrefix(r.texts, texts) {
		r = readTexts{}
	}
	found := make(map[string]bool, len(r.found))
	for k := range r.found {
		found[k] = true
	}
	for _, text := range texts[len(r.texts):] {
		for _, k := range read(text) {
			found[k] = true
		}
	}
	return readTexts{texts: append([]string(nil), texts...), found: found}
}

// same is whether r is last unchanged, as next returns it when nothing
// was added.
func (r readTexts) same(last readTexts) bool {
	return r.found != nil && len(r.texts) == len(last.texts) && (len(r.texts) == 0 || &r.texts[0] == &last.texts[0])
}

func isPrefix(prefix, of []string) bool {
	if len(prefix) > len(of) {
		return false
	}
	for i := range prefix {
		if prefix[i] != of[i] {
			return false
		}
	}
	return true
}

// readRefs is a session's references, read from its prompts and its
// pastes.
type readRefs struct {
	prompts, pasted readTexts
	refs            []string
}

func (r readRefs) next(m Member) readRefs {
	prompts, pasted := r.prompts.next(m.Prompts, References), r.pasted.next(m.Pasted, References)
	if r.refs != nil && prompts.same(r.prompts) && pasted.same(r.pasted) {
		return r
	}
	r.prompts, r.pasted = prompts, pasted
	seen := make(map[string]bool, len(r.prompts.found)+len(r.pasted.found))
	for k := range r.prompts.found {
		seen[k] = true
	}
	for k := range r.pasted.found {
		seen[k] = true
	}
	r.refs = sortedKeys(seen)
	return r
}
