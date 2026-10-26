package render

import (
	"container/list"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeLook struct {
	freed, gone bool
	place       *list.Element
}

func (f *fakeLook) Deallocate()               { f.freed = true }
func (f *fakeLook) markGone()                 { f.gone = true }
func (f *fakeLook) setPlace(e *list.Element)  { f.place = e }
func (f *fakeLook) cachePlace() *list.Element { return f.place }

// Item 212: a kept record checks that the looks it draws from are still
// cached by a mark on each, and touches each by its place in the cache,
// rather than finding it by its key.
func TestLookCacheMarks(t *testing.T) {
	put := func(c *lookCache[string, *fakeLook], key string, bytes int) *fakeLook {
		v := &fakeLook{}
		c.put(key, v, bytes)
		return v
	}
	for _, out := range []struct {
		name string
		do   func(c *lookCache[string, *fakeLook])
	}{
		{"removed from the cache", func(c *lookCache[string, *fakeLook]) { c.remove("a") }},
		{"evicted to make room", func(c *lookCache[string, *fakeLook]) { put(c, "b", 80) }},
		{"let go with the whole cache", func(c *lookCache[string, *fakeLook]) { c.markAllGone() }},
	} {
		t.Run("when a look is "+out.name, func(t *testing.T) {
			c := newLookCache[string, *fakeLook](100)
			a := put(c, "a", 40)
			out.do(c)

			t.Run("it should be marked gone", func(t *testing.T) {
				assert.True(t, a.gone)
			})
		})
	}

	t.Run("when a look is touched by its place in the cache", func(t *testing.T) {
		c := newLookCache[string, *fakeLook](100)
		a := put(c, "a", 40)
		put(c, "b", 40)
		c.touch(a)
		put(c, "c", 40)
		_, kept := c.peek("a")

		t.Run("it should be the last the cache evicts, as after get", func(t *testing.T) {
			assert.True(t, kept)
		})
	})
}

// Item 192: a building's look is baked once and kept, least recently
// used first out, in a cache capped in bytes.
func TestLookCache(t *testing.T) {
	put := func(c *lookCache[string, *fakeLook], key string, bytes int) *fakeLook {
		v := &fakeLook{}
		c.put(key, v, bytes)
		return v
	}

	t.Run("when entries are put and their total stays under the cap", func(t *testing.T) {
		c := newLookCache[string, *fakeLook](100)
		put(c, "a", 30)
		put(c, "b", 30)
		put(c, "c", 30)

		t.Run("it should keep every entry", func(t *testing.T) {
			assert.Equal(t, 3, c.len())
		})
	})

	t.Run("when a put takes the total over the cap", func(t *testing.T) {
		c := newLookCache[string, *fakeLook](100)
		a := put(c, "a", 40)
		put(c, "b", 40)
		put(c, "c", 40)

		t.Run("it should evict the least recently used entry first", func(t *testing.T) {
			_, ok := c.get("a")
			assert.False(t, ok)
		})

		t.Run("it should keep the total at or under the cap", func(t *testing.T) {
			assert.LessOrEqual(t, c.bytes(), 100)
		})

		t.Run("it should deallocate each entry it evicts", func(t *testing.T) {
			assert.True(t, a.freed)
		})
	})

	t.Run("when an entry is read before the cap is passed", func(t *testing.T) {
		c := newLookCache[string, *fakeLook](100)
		put(c, "a", 40)
		put(c, "b", 40)
		c.get("a")
		put(c, "c", 40)

		t.Run("it should survive the eviction ahead of an older unread entry", func(t *testing.T) {
			_, ok := c.get("a")
			assert.True(t, ok)
		})
	})

	t.Run("when an entry is larger than the cap by itself", func(t *testing.T) {
		c := newLookCache[string, *fakeLook](100)
		big := put(c, "big", 101)

		t.Run("it should not be kept", func(t *testing.T) {
			_, ok := c.get("big")
			assert.False(t, ok)
		})

		t.Run("it should be deallocated", func(t *testing.T) {
			assert.True(t, big.freed)
		})
	})

	t.Run("when a key is put again", func(t *testing.T) {
		c := newLookCache[string, *fakeLook](100)
		old := put(c, "a", 40)
		put(c, "a", 50)

		t.Run("it should count only the new entry's bytes", func(t *testing.T) {
			assert.Equal(t, 50, c.bytes())
		})

		t.Run("it should deallocate the entry it replaces", func(t *testing.T) {
			assert.True(t, old.freed)
		})
	})

	t.Run("when a building's key is first seen", func(t *testing.T) {
		c := newLookCache[string, *fakeLook](100)

		t.Run("it should not bake", func(t *testing.T) {
			assert.False(t, c.steady(runID{id: "building"}, "k1", 1))
		})

		t.Run("and it holds the next frame", func(t *testing.T) {
			t.Run("it should bake", func(t *testing.T) {
				assert.True(t, c.steady(runID{id: "building"}, "k1", 2))
			})
		})

		t.Run("and it changes the next frame", func(t *testing.T) {
			t.Run("it should not bake", func(t *testing.T) {
				assert.False(t, c.steady(runID{id: "building"}, "k2", 3))
			})
		})

		t.Run("and it returns after a frame missed", func(t *testing.T) {
			c.steady(runID{id: "building"}, "k3", 4)

			t.Run("it should not bake", func(t *testing.T) {
				assert.False(t, c.steady(runID{id: "building"}, "k3", 6))
			})
		})
	})
}

func TestLookCacheForget(t *testing.T) {
	t.Run("when a building drawn before a frame is forgotten", func(t *testing.T) {
		c := newLookCache[string, *fakeLook](100)
		c.steady(runID{id: "gone"}, "k", 10)
		c.steady(runID{id: "here"}, "k", 12)
		c.forget(11)

		t.Run("it should drop what was seen of it", func(t *testing.T) {
			assert.NotContains(t, c.seen, runID{id: "gone"})
		})

		t.Run("it should keep what was seen of a building drawn since", func(t *testing.T) {
			assert.Contains(t, c.seen, runID{id: "here"})
		})
	})
}

func TestLookCacheCap(t *testing.T) {
	t.Run("it should be one sixteenth of the city's settled memory budget", func(t *testing.T) {
		assert.Equal(t, 512<<20/16, lookCacheCap)
	})
}
