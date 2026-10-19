package render

// drawOrder numbers what a frame's map issues, in the order it is
// issued: world sprites and road tags (item 75).
type drawOrder struct {
	n, lastWorld int
	tags         []int
}

func (o *drawOrder) reset() {
	o.n, o.lastWorld, o.tags = 0, 0, o.tags[:0]
}

func (o *drawOrder) world() {
	o.n++
	o.lastWorld = o.n
}

func (o *drawOrder) tag() {
	o.n++
	o.tags = append(o.tags, o.n)
}

// tagsUnderWorld is how many tags were issued before a world sprite.
func (o *drawOrder) tagsUnderWorld() int {
	under := 0
	for _, at := range o.tags {
		if at < o.lastWorld {
			under++
		}
	}
	return under
}

func (o *drawOrder) copyFrom(from *drawOrder) {
	o.n, o.lastWorld = from.n, from.lastWorld
	o.tags = append(o.tags[:0], from.tags...)
}
