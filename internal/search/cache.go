package search

import (
	"container/list"
	"time"
)

// cacheKey is the current-entry cache identity: query text, the normalised
// server filter, the global catalog revision and the model identity digest
// (plan §4.8). Credentials are deliberately absent.
type cacheKey struct {
	query    string
	servers  string
	revision string
	model    string
}

type cacheEntry struct {
	key     cacheKey
	vector  []float64
	expires time.Time
	element *list.Element
}

// vectorCache is the bounded, TTL-scoped query-vector cache.
type vectorCache struct {
	entries int
	ttl     time.Duration
	items   map[cacheKey]*cacheEntry
	order   *list.List
}

func newVectorCache(entries int, ttl time.Duration) *vectorCache {
	return &vectorCache{
		entries: entries,
		ttl:     ttl,
		items:   make(map[cacheKey]*cacheEntry, entries),
		order:   list.New(),
	}
}

func (c *vectorCache) get(key cacheKey, now time.Time) ([]float64, bool) {
	entry, ok := c.items[key]
	if !ok {
		return nil, false
	}
	if !now.Before(entry.expires) {
		c.remove(entry)
		return nil, false
	}
	c.order.MoveToFront(entry.element)
	return entry.vector, true
}

func (c *vectorCache) put(key cacheKey, vector []float64, now time.Time) {
	if entry, ok := c.items[key]; ok {
		entry.vector = vector
		entry.expires = now.Add(c.ttl)
		c.order.MoveToFront(entry.element)
		return
	}
	entry := &cacheEntry{key: key, vector: vector, expires: now.Add(c.ttl)}
	entry.element = c.order.PushFront(entry)
	c.items[key] = entry
	for len(c.items) > c.entries {
		oldest := c.order.Back()
		if oldest == nil {
			return
		}
		c.remove(oldest.Value.(*cacheEntry))
	}
}

func (c *vectorCache) remove(entry *cacheEntry) {
	if entry.element != nil {
		c.order.Remove(entry.element)
	}
	delete(c.items, entry.key)
}
