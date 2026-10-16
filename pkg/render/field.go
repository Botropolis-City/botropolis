package render

// field is the finder's line of text: what has been committed, where
// the caret is in it, in runes, and what an input method is composing
// there and has not committed yet.
type field struct {
	text      []rune
	caret     int
	composing string
}

// insert puts committed text at the caret and moves the caret past it.
func (f *field) insert(s string) {
	add := []rune(s)
	f.text = append(f.text[:f.caret], append(add, f.text[f.caret:]...)...)
	f.caret += len(add)
}

// backspace deletes the rune before the caret.
func (f *field) backspace() {
	if f.caret == 0 {
		return
	}
	f.text = append(f.text[:f.caret-1], f.text[f.caret:]...)
	f.caret--
}

// compose shows what an input method is composing at the caret; it is
// drawn but is not part of the query until it is committed.
func (f *field) compose(s string) {
	f.composing = s
}

// query is the committed text, which is what filters.
func (f *field) query() string {
	return string(f.text)
}

// shown is the text as drawn, with the composition at the caret, and
// the caret as a rune index into it, after the composition.
func (f *field) shown() (string, int) {
	comp := []rune(f.composing)
	shown := append(append(append([]rune{}, f.text[:f.caret]...), comp...), f.text[f.caret:]...)
	return string(shown), f.caret + len(comp)
}
