package cache

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// Entry represents a cache entry with TTL and metadata
type Entry struct {
	Data         []byte
	Expiry       time.Time
	ETag         string
	LastModified string
}

// IsExpired checks if the entry has expired
func (e *Entry) IsExpired() bool {
	return time.Now().After(e.Expiry)
}

// Size returns the approximate size of the entry in bytes
func (e *Entry) Size() int64 {
	return int64(len(e.Data) + len(e.ETag) + len(e.LastModified) + 24) // +24 for time.Time
}

// Cache is a thread-safe in-memory cache with TTL support
type Cache struct {
	mu        sync.RWMutex
	entries   map[string]*Entry
	maxSize   int
	maxMemory int64 // Maximum memory usage in bytes
	maxTTL    time.Duration // Maximum TTL allowed
	ttl       time.Duration
	minTTL    time.Duration        // Minimum TTL for entries
	accessLRU map[string]time.Time // Track access time for LRU eviction

	// Metrics
	hits      int64
	misses    int64
	evictions int64
	memory    int64 // Current memory usage
}

// NewCache creates a new cache with the given max size and default TTL
func NewCache(maxSize int, defaultTTL time.Duration, minTTL time.Duration) *Cache {
	return NewCacheWithMemoryLimit(maxSize, defaultTTL, minTTL, 20*1024*1024, 24*time.Hour) // 20MB default, 24h max TTL
}

// NewCacheWithMemoryLimit creates a cache with memory limit
func NewCacheWithMemoryLimit(maxSize int, defaultTTL time.Duration, minTTL time.Duration, maxMemory int64, maxTTL time.Duration) *Cache {
	return &Cache{
		entries:   make(map[string]*Entry),
		maxSize:   maxSize,
		maxMemory: maxMemory,
		maxTTL:    maxTTL,
		ttl:       defaultTTL,
		minTTL:    minTTL,
		accessLRU: make(map[string]time.Time),
		memory:    0,
		hits:      0,
		misses:    0,
		evictions: 0,
	}
}

// Get retrieves an entry from the cache
// Returns (entry, found) where found is false if not found or expired
func (c *Cache) Get(key string) (*Entry, bool) {
	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	// Check expiration
	if entry.IsExpired() {
		// Remove expired entry
		c.mu.Lock()
		delete(c.entries, key)
		delete(c.accessLRU, key)
		c.memory -= entry.Size()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	// Update access time for LRU and record hit
	c.mu.Lock()
	c.accessLRU[key] = time.Now()
	c.hits++
	c.mu.Unlock()

	return entry, true
}

// Set stores an entry in the cache with the given TTL
// If TTL is less than minTTL, minTTL is used
// If TTL is greater than maxTTL, maxTTL is used
func (c *Cache) Set(key string, data []byte, ttl time.Duration, etag string, lastModified string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Enforce minimum TTL
	if ttl < c.minTTL {
		ttl = c.minTTL
	}

	// Enforce maximum TTL
	if ttl > c.maxTTL {
		ttl = c.maxTTL
	}

	newEntry := &Entry{
		Data:         data,
		Expiry:       time.Now().Add(ttl),
		ETag:         etag,
		LastModified: lastModified,
	}
	newSize := newEntry.Size()

	// Remove old entry if updating
	if oldEntry, exists := c.entries[key]; exists {
		c.memory -= oldEntry.Size()
	}

	// Evict entries if we exceed memory or size limits
	for (len(c.entries) >= c.maxSize || c.memory+newSize > c.maxMemory) && len(c.entries) > 0 {
		c.evictLRU()
	}

	c.entries[key] = newEntry
	c.accessLRU[key] = time.Now()
	c.memory += newSize
}

// SetWithDefaultTTL stores an entry with the default TTL
func (c *Cache) SetWithDefaultTTL(key string, data []byte, etag string, lastModified string) {
	c.Set(key, data, c.ttl, etag, lastModified)
}

// Delete removes an entry from the cache
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.entries[key]; exists {
		c.memory -= entry.Size()
	}
	delete(c.entries, key)
	delete(c.accessLRU, key)
}

// Clear removes all entries from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*Entry)
	c.accessLRU = make(map[string]time.Time)
	c.memory = 0
}

// Size returns the current number of entries in the cache
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}

// evictLRU evicts the least recently used entry
// Must be called with lock held
func (c *Cache) evictLRU() {
	if len(c.entries) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, accessTime := range c.accessLRU {
		if first || accessTime.Before(oldestTime) {
			oldestKey = key
			oldestTime = accessTime
			first = false
		}
	}

	if oldestKey != "" {
		if entry, exists := c.entries[oldestKey]; exists {
			c.memory -= entry.Size()
		}
		delete(c.entries, oldestKey)
		delete(c.accessLRU, oldestKey)
		c.evictions++
	}
}

// CleanupExpired removes all expired entries from the cache
// This should be called periodically (e.g., every minute)
func (c *Cache) CleanupExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	removed := 0
	now := time.Now()

	for key, entry := range c.entries {
		if now.After(entry.Expiry) {
			c.memory -= entry.Size()
			delete(c.entries, key)
			delete(c.accessLRU, key)
			removed++
		}
	}

	return removed
}

// Stats returns cache statistics
type Stats struct {
	Entries     int
	MaxSize     int
	Memory      int64 // Current memory usage in bytes
	MaxMemory   int64 // Maximum memory limit in bytes
	DefaultTTL  time.Duration
	MinTTL      time.Duration
	MaxTTL      time.Duration
	Hits        int64
	Misses      int64
	Evictions   int64
	HitRatio    float64 // Hit ratio (0.0 to 1.0)
}

// GetStats returns current cache statistics
func (c *Cache) GetStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRatio := 0.0
	if total > 0 {
		hitRatio = float64(c.hits) / float64(total)
	}

	return Stats{
		Entries:    len(c.entries),
		MaxSize:    c.maxSize,
		Memory:     c.memory,
		MaxMemory:  c.maxMemory,
		DefaultTTL: c.ttl,
		MinTTL:     c.minTTL,
		MaxTTL:     c.maxTTL,
		Hits:       c.hits,
		Misses:     c.misses,
		Evictions:  c.evictions,
		HitRatio:   hitRatio,
	}
}

// HashKey generates a cache key from multiple components
// Uses SHA256 to create a consistent key from arbitrary strings
func HashKey(components ...string) string {
	h := sha256.New()
	for _, comp := range components {
		h.Write([]byte(comp))
		h.Write([]byte{0}) // Separator to prevent collisions
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
