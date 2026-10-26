package render

import "container/list"

// A building's look changes far less often than it is drawn, so it is
// baked once into an image and kept (item 192). The cache holds as many
// bytes of image as a sixteenth of the city's settled memory budget
// (performance/budgets.md: 512 MiB, GPU included), least recently used
// out first, so memory grows with what is on screen, never with every
// look a building could have.
const (
	citySettledBudget = 512 << 20
	lookCacheCap      = citySettledBudget / 16
)

type deallocator interface{ Deallocate() }

// A value that can be told it has left the cache, and that knows its
// place in the cache's order, is checked and touched by itself rather
// than found by its key (item 212).
type (
	goneMarker  interface{ markGone() }
	cachePlaced interface {
		setPlace(*list.Element)
		cachePlace() *list.Element
	}
)

type lookEntry[K comparable, V deallocator] struct {
	key   K
	value V
	bytes int
}

// seenKey is the key a building drew under, and the frame it did.
type seenKey[K comparable] struct {
	key   K
	frame int
}

type lookCache[K comparable, V deallocator] struct {
	cap     int
	total   int
	order   list.List
	entries map[K]*list.Element
	seen    map[runID]seenKey[K]
	// replaced is the key the last call to steady replaced, and
	// hadReplaced whether there was one, for the debug build's look-miss
	// line (item 202).
	replaced    K
	hadReplaced bool
}

func newLookCache[K comparable, V deallocator](cap int) *lookCache[K, V] {
	return &lookCache[K, V]{cap: cap, entries: map[K]*list.Element{}, seen: map[runID]seenKey[K]{}}
}

func (c *lookCache[K, V]) len() int   { return len(c.entries) }
func (c *lookCache[K, V]) bytes() int { return c.total }

// get is the look kept under a key, which counts as using it.
func (c *lookCache[K, V]) get(key K) (V, bool) {
	e, ok := c.entries[key]
	if !ok {
		var zero V
		return zero, false
	}
	c.order.MoveToFront(e)
	return e.Value.(*lookEntry[K, V]).value, true
}

// markAllGone marks every look the cache holds as let go, when the
// cache itself is let go whole.
func (c *lookCache[K, V]) markAllGone() {
	for _, e := range c.entries {
		if g, ok := any(e.Value.(*lookEntry[K, V]).value).(goneMarker); ok {
			g.markGone()
		}
	}
}

// touch counts a look kept in the cache as used, as get does, by its
// place in the cache rather than its key.
func (c *lookCache[K, V]) touch(v V) {
	if p, ok := any(v).(cachePlaced); ok && p.cachePlace() != nil {
		c.order.MoveToFront(p.cachePlace())
	}
}

// peek is the look kept under a key, without counting as using it.
func (c *lookCache[K, V]) peek(key K) (V, bool) {
	e, ok := c.entries[key]
	if !ok {
		var zero V
		return zero, false
	}
	return e.Value.(*lookEntry[K, V]).value, true
}

// put keeps a look of a size in bytes, evicting the least recently used
// until it fits; a look bigger than the whole cap is not kept.
func (c *lookCache[K, V]) put(key K, value V, bytes int) {
	c.remove(key)
	if bytes > c.cap {
		value.Deallocate()
		return
	}
	for c.total+bytes > c.cap {
		c.remove(c.order.Back().Value.(*lookEntry[K, V]).key)
	}
	e := c.order.PushFront(&lookEntry[K, V]{key: key, value: value, bytes: bytes})
	c.entries[key] = e
	if p, ok := any(value).(cachePlaced); ok {
		p.setPlace(e)
	}
	c.total += bytes
}

func (c *lookCache[K, V]) remove(key K) {
	e, ok := c.entries[key]
	if !ok {
		return
	}
	entry := e.Value.(*lookEntry[K, V])
	if g, ok := any(entry.value).(goneMarker); ok {
		g.markGone()
	}
	entry.value.Deallocate()
	c.total -= entry.bytes
	c.order.Remove(e)
	delete(c.entries, key)
}

// steady notes the key a building draws under this frame and says
// whether it drew under the same key the frame before: only a look that
// has held is worth baking, so a zoom or a slide draws as it always did
// rather than baking every frame.
func (c *lookCache[K, V]) steady(id runID, key K, frame int) bool {
	last, ok := c.seen[id]
	c.replaced, c.hadReplaced = last.key, ok
	c.seen[id] = seenKey[K]{key: key, frame: frame}
	return ok && last.key == key && last.frame == frame-1
}

// forget drops what was seen of buildings not drawn for a while, so the
// record does not outlive the sessions in it.
func (c *lookCache[K, V]) forget(before int) {
	for id, s := range c.seen {
		if s.frame < before {
			delete(c.seen, id)
		}
	}
}
