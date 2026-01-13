package feeds

import (
	"sync"
	"time"
)

// Cache provides in-memory caching for feeds
type Cache struct {
	mu     sync.RWMutex
	feeds  map[string]*cachedFeed
	maxAge time.Duration
}

type cachedFeed struct {
	feed      *NamedFeed
	expiresAt time.Time
}

// NewCache creates a new feed cache
func NewCache(maxAge time.Duration) *Cache {
	return &Cache{
		feeds:  make(map[string]*cachedFeed),
		maxAge: maxAge,
	}
}

// Get retrieves a feed from cache
func (c *Cache) Get(slug string) (*NamedFeed, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, ok := c.feeds[slug]
	if !ok {
		return nil, false
	}

	// Check if expired
	if time.Now().After(cached.expiresAt) {
		return nil, false
	}

	return cached.feed, true
}

// Set stores a feed in cache
func (c *Cache) Set(slug string, feed *NamedFeed) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.feeds[slug] = &cachedFeed{
		feed:      feed,
		expiresAt: time.Now().Add(c.maxAge),
	}
}

// Invalidate removes a feed from cache
func (c *Cache) Invalidate(slug string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.feeds, slug)
}

// Clear removes all feeds from cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.feeds = make(map[string]*cachedFeed)
}

// CleanExpired removes expired entries from cache
func (c *Cache) CleanExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	removed := 0

	for slug, cached := range c.feeds {
		if now.After(cached.expiresAt) {
			delete(c.feeds, slug)
			removed++
		}
	}

	return removed
}

// Size returns the number of cached feeds
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.feeds)
}
