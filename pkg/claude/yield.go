package claude

import "io"

// yieldEvery is how many bytes a paced read scans between yields: about
// 10 ms of work at the rate a transcript is parsed (item 203).
const yieldEvery = 1 << 20

// yielding is r calling yield after every yieldEvery bytes read through
// it, or r itself when yield is nil.
func yielding(r io.Reader, yield func()) io.Reader {
	if yield == nil {
		return r
	}
	return &yieldingReader{r: r, yield: yield}
}

type yieldingReader struct {
	r     io.Reader
	yield func()
	since int
}

func (y *yieldingReader) Read(p []byte) (int, error) {
	n, err := y.r.Read(p)
	if y.since += n; y.since >= yieldEvery {
		y.since = 0
		y.yield()
	}
	return n, err
}
