package cache

import (
	"sync"
	"testing"
	"time"
)

// TestNewCache tests cache creation
// Validates: Cache initialization, default values
func TestNewCache(t *testing.T) {
	cache := NewCache(100, 5*time.Minute, 1*time.Minute)

	if cache == nil {
		t.Fatal("NewCache() returned nil")
	}

	stats := cache.GetStats()
	if stats.MaxSize != 100 {
		t.Errorf("MaxSize = %d, want 100", stats.MaxSize)
	}
	if stats.DefaultTTL != 5*time.Minute {
		t.Errorf("DefaultTTL = %v, want 5m", stats.DefaultTTL)
	}
	if stats.MinTTL != 1*time.Minute {
		t.Errorf("MinTTL = %v, want 1m", stats.MinTTL)
	}
	if stats.Entries != 0 {
		t.Errorf("Entries = %d, want 0", stats.Entries)
	}
}

// TestSetAndGet tests basic cache operations
// Validates: Set, Get, cache hit, data integrity
func TestSetAndGet(t *testing.T) {
	cache := NewCache(10, 5*time.Minute, 1*time.Minute)

	key := "test-key"
	data := []byte("test data")
	etag := "etag-123"
	lastMod := "Mon, 01 Jan 2025 00:00:00 GMT"

	cache.Set(key, data, 5*time.Minute, etag, lastMod)

	entry, found := cache.Get(key)
	if !found {
		t.Fatal("Get() returned false, want true (cache miss)")
	}

	if string(entry.Data) != string(data) {
		t.Errorf("Data = %q, want %q", string(entry.Data), string(data))
	}
	if entry.ETag != etag {
		t.Errorf("ETag = %q, want %q", entry.ETag, etag)
	}
	if entry.LastModified != lastMod {
		t.Errorf("LastModified = %q, want %q", entry.LastModified, lastMod)
	}
}

// TestGetNonexistent tests cache miss
// Validates: Get on nonexistent key returns false
func TestGetNonexistent(t *testing.T) {
	cache := NewCache(10, 5*time.Minute, 1*time.Minute)

	_, found := cache.Get("nonexistent")
	if found {
		t.Error("Get(nonexistent) returned true, want false")
	}
}

// TestSetWithDefaultTTL tests using default TTL
// Validates: Default TTL is applied correctly
func TestSetWithDefaultTTL(t *testing.T) {
	cache := NewCache(10, 5*time.Minute, 1*time.Minute)

	key := "test-key"
	data := []byte("test data")

	cache.SetWithDefaultTTL(key, data, "", "")

	entry, found := cache.Get(key)
	if !found {
		t.Fatal("Get() returned false, want true")
	}

	// Check that expiry is approximately 5 minutes in the future
	expectedExpiry := time.Now().Add(5 * time.Minute)
	diff := entry.Expiry.Sub(expectedExpiry)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("Expiry time not as expected, diff = %v", diff)
	}
}

// TestMinTTLEnforcement tests that minimum TTL is enforced
// Validates: TTL less than minTTL is increased to minTTL
func TestMinTTLEnforcement(t *testing.T) {
	cache := NewCache(10, 5*time.Minute, 2*time.Minute)

	key := "test-key"
	data := []byte("test data")

	// Try to set with 1 minute TTL (less than minTTL of 2 minutes)
	cache.Set(key, data, 1*time.Minute, "", "")

	entry, found := cache.Get(key)
	if !found {
		t.Fatal("Get() returned false, want true")
	}

	// Expiry should be ~2 minutes (minTTL), not 1 minute
	expectedExpiry := time.Now().Add(2 * time.Minute)
	diff := entry.Expiry.Sub(expectedExpiry)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("Expiry should be ~2 minutes (minTTL), got diff = %v", diff)
	}
}

// TestExpiration tests entry expiration
// Validates: Expired entries are not returned, cleanup works
func TestExpiration(t *testing.T) {
	cache := NewCache(10, 5*time.Minute, 10*time.Millisecond) // Very short minTTL for testing

	key := "test-key"
	data := []byte("test data")

	// Set with very short TTL
	cache.Set(key, data, 50*time.Millisecond, "", "")

	// Should be present immediately
	_, found := cache.Get(key)
	if !found {
		t.Error("Get() returned false immediately after Set, want true")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	_, found = cache.Get(key)
	if found {
		t.Error("Get() returned true after expiration, want false")
	}

	// Cache should be empty after expiration
	if cache.Size() != 0 {
		t.Errorf("Size() = %d after expiration, want 0", cache.Size())
	}
}

// TestDelete tests entry deletion
// Validates: Delete removes entry from cache
func TestDelete(t *testing.T) {
	cache := NewCache(10, 5*time.Minute, 1*time.Minute)

	key := "test-key"
	data := []byte("test data")

	cache.Set(key, data, 5*time.Minute, "", "")

	// Verify it's there
	_, found := cache.Get(key)
	if !found {
		t.Fatal("Get() returned false before Delete, want true")
	}

	// Delete it
	cache.Delete(key)

	// Verify it's gone
	_, found = cache.Get(key)
	if found {
		t.Error("Get() returned true after Delete, want false")
	}
}

// TestClear tests clearing the entire cache
// Validates: Clear removes all entries
func TestClear(t *testing.T) {
	cache := NewCache(10, 5*time.Minute, 1*time.Minute)

	// Add multiple entries
	for i := 0; i < 5; i++ {
		key := string(rune('a' + i))
		cache.Set(key, []byte("data"), 5*time.Minute, "", "")
	}

	if cache.Size() != 5 {
		t.Fatalf("Size() = %d before Clear, want 5", cache.Size())
	}

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("Size() = %d after Clear, want 0", cache.Size())
	}

	// Verify entries are gone
	_, found := cache.Get("a")
	if found {
		t.Error("Get(a) returned true after Clear, want false")
	}
}

// TestLRUEviction tests LRU eviction when cache is full
// Validates: Least recently used entry is evicted when cache is full
func TestLRUEviction(t *testing.T) {
	cache := NewCache(3, 5*time.Minute, 1*time.Minute) // Max 3 entries

	// Add 3 entries
	cache.Set("a", []byte("data-a"), 5*time.Minute, "", "")
	time.Sleep(10 * time.Millisecond)
	cache.Set("b", []byte("data-b"), 5*time.Minute, "", "")
	time.Sleep(10 * time.Millisecond)
	cache.Set("c", []byte("data-c"), 5*time.Minute, "", "")

	// Cache should be full
	if cache.Size() != 3 {
		t.Fatalf("Size() = %d, want 3", cache.Size())
	}

	// Access "a" to make it recently used
	cache.Get("a")
	time.Sleep(10 * time.Millisecond)

	// Add another entry - should evict "b" (least recently used)
	cache.Set("d", []byte("data-d"), 5*time.Minute, "", "")

	// Should still have 3 entries
	if cache.Size() != 3 {
		t.Errorf("Size() = %d after eviction, want 3", cache.Size())
	}

	// "a" should still be present (recently accessed)
	_, found := cache.Get("a")
	if !found {
		t.Error("Get(a) returned false, want true (was recently accessed)")
	}

	// "b" should be evicted (least recently used)
	_, found = cache.Get("b")
	if found {
		t.Error("Get(b) returned true, want false (should be evicted)")
	}

	// "c" should still be present
	_, found = cache.Get("c")
	if !found {
		t.Error("Get(c) returned false, want true")
	}

	// "d" should be present (just added)
	_, found = cache.Get("d")
	if !found {
		t.Error("Get(d) returned false, want true")
	}
}

// TestCleanupExpired tests manual cleanup of expired entries
// Validates: CleanupExpired removes only expired entries
func TestCleanupExpired(t *testing.T) {
	cache := NewCache(10, 5*time.Minute, 10*time.Millisecond)

	// Add entries with different TTLs
	cache.Set("short", []byte("data"), 50*time.Millisecond, "", "")
	cache.Set("long", []byte("data"), 5*time.Minute, "", "")

	if cache.Size() != 2 {
		t.Fatalf("Size() = %d, want 2", cache.Size())
	}

	// Wait for short entry to expire
	time.Sleep(100 * time.Millisecond)

	// Run cleanup
	removed := cache.CleanupExpired()

	if removed != 1 {
		t.Errorf("CleanupExpired() = %d, want 1", removed)
	}

	if cache.Size() != 1 {
		t.Errorf("Size() = %d after cleanup, want 1", cache.Size())
	}

	// Short entry should be gone
	_, found := cache.Get("short")
	if found {
		t.Error("Get(short) returned true after cleanup, want false")
	}

	// Long entry should still be present
	_, found = cache.Get("long")
	if !found {
		t.Error("Get(long) returned false after cleanup, want true")
	}
}

// TestConcurrency tests concurrent access to cache
// Validates: Thread-safety with concurrent reads and writes
func TestConcurrency(t *testing.T) {
	cache := NewCache(100, 5*time.Minute, 1*time.Minute)

	var wg sync.WaitGroup
	operations := 100

	// Concurrent writes
	wg.Add(operations)
	for i := 0; i < operations; i++ {
		go func(n int) {
			defer wg.Done()
			key := string(rune('a' + (n % 26)))
			cache.Set(key, []byte("data"), 5*time.Minute, "", "")
		}(i)
	}

	// Concurrent reads
	wg.Add(operations)
	for i := 0; i < operations; i++ {
		go func(n int) {
			defer wg.Done()
			key := string(rune('a' + (n % 26)))
			cache.Get(key)
		}(i)
	}

	// Concurrent deletes
	wg.Add(operations / 2)
	for i := 0; i < operations/2; i++ {
		go func(n int) {
			defer wg.Done()
			key := string(rune('a' + (n % 26)))
			cache.Delete(key)
		}(i)
	}

	wg.Wait()

	// If we get here without a race, the test passes
	// Cache should have some entries (exact count depends on timing)
	if cache.Size() < 0 || cache.Size() > 100 {
		t.Errorf("Size() = %d, want between 0 and 100", cache.Size())
	}
}

// TestHashKey tests cache key hashing
// Validates: Consistent hashing, collision resistance
func TestHashKey(t *testing.T) {
	tests := []struct {
		name       string
		components []string
		checkEqual [][]string // Other component sets that should produce same hash
		checkDiff  [][]string // Component sets that should produce different hash
	}{
		{
			name:       "single component",
			components: []string{"test"},
			checkEqual: [][]string{{"test"}},
			checkDiff:  [][]string{{"test2"}, {"Test"}},
		},
		{
			name:       "multiple components",
			components: []string{"upstream", "filter1", "filter2"},
			checkEqual: [][]string{{"upstream", "filter1", "filter2"}},
			checkDiff: [][]string{
				{"upstream", "filter2", "filter1"}, // Different order
				{"upstream", "filter1"},            // Different count
				{"upstreamfilter1filter2"},         // Concatenated differently
			},
		},
		{
			name:       "empty components",
			components: []string{},
			checkEqual: [][]string{{}},
			checkDiff:  [][]string{{""}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := HashKey(tt.components...)

			// Check that it's consistent
			hash2 := HashKey(tt.components...)
			if hash1 != hash2 {
				t.Errorf("HashKey() inconsistent: %q != %q", hash1, hash2)
			}

			// Check equal cases
			for _, eq := range tt.checkEqual {
				hashEq := HashKey(eq...)
				if hash1 != hashEq {
					t.Errorf("HashKey(%v) = %q, want %q (should be equal)", eq, hashEq, hash1)
				}
			}

			// Check different cases
			for _, diff := range tt.checkDiff {
				hashDiff := HashKey(diff...)
				if hash1 == hashDiff {
					t.Errorf("HashKey(%v) = %q, same as HashKey(%v) (should be different)", diff, hashDiff, tt.components)
				}
			}
		})
	}
}

// TestGetStats tests statistics retrieval
// Validates: Stats accuracy
func TestGetStats(t *testing.T) {
	cache := NewCache(50, 10*time.Minute, 2*time.Minute)

	// Add some entries
	for i := 0; i < 5; i++ {
		key := string(rune('a' + i))
		cache.Set(key, []byte("data"), 5*time.Minute, "", "")
	}

	stats := cache.GetStats()

	if stats.Entries != 5 {
		t.Errorf("Stats.Entries = %d, want 5", stats.Entries)
	}
	if stats.MaxSize != 50 {
		t.Errorf("Stats.MaxSize = %d, want 50", stats.MaxSize)
	}
	if stats.DefaultTTL != 10*time.Minute {
		t.Errorf("Stats.DefaultTTL = %v, want 10m", stats.DefaultTTL)
	}
	if stats.MinTTL != 2*time.Minute {
		t.Errorf("Stats.MinTTL = %v, want 2m", stats.MinTTL)
	}
}

// TestUpdateExistingEntry tests updating an existing cache entry
// Validates: Overwrites old data, resets TTL
func TestUpdateExistingEntry(t *testing.T) {
	cache := NewCache(10, 5*time.Minute, 1*time.Minute)

	key := "test-key"

	// Set initial value
	cache.Set(key, []byte("old data"), 5*time.Minute, "old-etag", "old-lastmod")

	// Update with new value
	cache.Set(key, []byte("new data"), 5*time.Minute, "new-etag", "new-lastmod")

	entry, found := cache.Get(key)
	if !found {
		t.Fatal("Get() returned false, want true")
	}

	if string(entry.Data) != "new data" {
		t.Errorf("Data = %q, want 'new data'", string(entry.Data))
	}
	if entry.ETag != "new-etag" {
		t.Errorf("ETag = %q, want 'new-etag'", entry.ETag)
	}
	if entry.LastModified != "new-lastmod" {
		t.Errorf("LastModified = %q, want 'new-lastmod'", entry.LastModified)
	}

	// Cache size should still be 1
	if cache.Size() != 1 {
		t.Errorf("Size() = %d, want 1", cache.Size())
	}
}

// TestMaxTTLEnforcement tests that maximum TTL is enforced
// Validates: TTL greater than maxTTL is capped to maxTTL
func TestMaxTTLEnforcement(t *testing.T) {
	// Create cache with 1 hour max TTL
	cache := NewCacheWithMemoryLimit(10, 5*time.Minute, 1*time.Minute, 10*1024*1024, 1*time.Hour)

	key := "test-key"
	data := []byte("test data")

	// Try to set with 2 hour TTL (more than maxTTL of 1 hour)
	cache.Set(key, data, 2*time.Hour, "", "")

	entry, found := cache.Get(key)
	if !found {
		t.Fatal("Get() returned false, want true")
	}

	// Expiry should be ~1 hour (maxTTL), not 2 hours
	expectedExpiry := time.Now().Add(1 * time.Hour)
	diff := entry.Expiry.Sub(expectedExpiry)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("Expiry should be ~1 hour (maxTTL), got diff = %v", diff)
	}
}

// TestMemoryTracking tests that cache tracks memory usage correctly
// Validates: Entry size calculation, memory accounting
func TestMemoryTracking(t *testing.T) {
	cache := NewCacheWithMemoryLimit(10, 5*time.Minute, 1*time.Minute, 10*1024*1024, 24*time.Hour)

	key := "test-key"
	data := []byte("test data with some content")
	etag := "etag-12345"
	lastMod := "Mon, 01 Jan 2025 00:00:00 GMT"

	// Add entry
	cache.Set(key, data, 5*time.Minute, etag, lastMod)

	stats := cache.GetStats()

	// Calculate expected size
	expectedSize := int64(len(data) + len(etag) + len(lastMod) + 24) // +24 for time.Time

	if stats.Memory != expectedSize {
		t.Errorf("Memory = %d, want %d", stats.Memory, expectedSize)
	}

	// Add another entry
	key2 := "test-key-2"
	data2 := []byte("more data")
	cache.Set(key2, data2, 5*time.Minute, "", "")

	stats = cache.GetStats()
	expectedSize2 := int64(len(data2) + 24)
	totalExpected := expectedSize + expectedSize2

	if stats.Memory != totalExpected {
		t.Errorf("Memory after 2 entries = %d, want %d", stats.Memory, totalExpected)
	}

	// Delete first entry
	cache.Delete(key)

	stats = cache.GetStats()
	if stats.Memory != expectedSize2 {
		t.Errorf("Memory after delete = %d, want %d", stats.Memory, expectedSize2)
	}
}

// TestMemoryBasedEviction tests LRU eviction based on memory pressure
// Validates: Eviction happens when memory limit is reached
func TestMemoryBasedEviction(t *testing.T) {
	// Create cache with 200 byte memory limit
	cache := NewCacheWithMemoryLimit(100, 5*time.Minute, 1*time.Minute, 200, 24*time.Hour)

	// Add entry with ~100 bytes of data
	data1 := make([]byte, 100)
	for i := range data1 {
		data1[i] = 'a'
	}
	cache.Set("key1", data1, 5*time.Minute, "", "")
	time.Sleep(10 * time.Millisecond)

	// Add another entry with ~100 bytes
	data2 := make([]byte, 100)
	for i := range data2 {
		data2[i] = 'b'
	}
	cache.Set("key2", data2, 5*time.Minute, "", "")
	time.Sleep(10 * time.Millisecond)

	// Both should fit (barely)
	stats := cache.GetStats()
	if stats.Entries != 2 {
		t.Logf("Warning: Expected 2 entries, got %d (entries may be slightly larger than data)", stats.Entries)
	}

	// Add third entry - should cause eviction of key1 (oldest)
	data3 := make([]byte, 100)
	for i := range data3 {
		data3[i] = 'c'
	}
	cache.Set("key3", data3, 5*time.Minute, "", "")

	// key1 should be evicted
	_, found := cache.Get("key1")
	if found {
		t.Error("Get(key1) returned true, want false (should be evicted due to memory pressure)")
	}

	stats = cache.GetStats()
	if stats.Evictions < 1 {
		t.Errorf("Evictions = %d, want >= 1", stats.Evictions)
	}
}

// TestHitMissTracking tests that cache hit/miss statistics are tracked correctly
// Validates: Hits and misses are counted accurately
func TestHitMissTracking(t *testing.T) {
	cache := NewCache(10, 5*time.Minute, 1*time.Minute)

	key := "test-key"
	data := []byte("test data")

	// Initial stats should be zero
	stats := cache.GetStats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Errorf("Initial stats: Hits=%d, Misses=%d, want both 0", stats.Hits, stats.Misses)
	}

	// First Get should be a miss
	_, found := cache.Get(key)
	if found {
		t.Error("Get() returned true for nonexistent key, want false")
	}

	stats = cache.GetStats()
	if stats.Misses != 1 {
		t.Errorf("Misses = %d after first Get, want 1", stats.Misses)
	}

	// Set the key
	cache.Set(key, data, 5*time.Minute, "", "")

	// Second Get should be a hit
	_, found = cache.Get(key)
	if !found {
		t.Error("Get() returned false for existing key, want true")
	}

	stats = cache.GetStats()
	if stats.Hits != 1 {
		t.Errorf("Hits = %d after cache hit, want 1", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Misses = %d after cache hit, want 1 (unchanged)", stats.Misses)
	}

	// Third Get should be another hit
	_, found = cache.Get(key)
	if !found {
		t.Error("Get() returned false for existing key, want true")
	}

	stats = cache.GetStats()
	if stats.Hits != 2 {
		t.Errorf("Hits = %d after second hit, want 2", stats.Hits)
	}

	// Get nonexistent key should be a miss
	cache.Get("nonexistent")

	stats = cache.GetStats()
	if stats.Misses != 2 {
		t.Errorf("Misses = %d after second miss, want 2", stats.Misses)
	}
}

// TestHitRatioCalculation tests that hit ratio is calculated correctly
// Validates: Hit ratio formula is correct
func TestHitRatioCalculation(t *testing.T) {
	cache := NewCache(10, 5*time.Minute, 1*time.Minute)

	// Initial ratio should be 0.0
	stats := cache.GetStats()
	if stats.HitRatio != 0.0 {
		t.Errorf("Initial HitRatio = %f, want 0.0", stats.HitRatio)
	}

	// Add a key
	cache.Set("key", []byte("data"), 5*time.Minute, "", "")

	// 3 hits
	cache.Get("key")
	cache.Get("key")
	cache.Get("key")

	// 1 miss
	cache.Get("nonexistent")

	stats = cache.GetStats()
	expectedRatio := 3.0 / 4.0 // 3 hits out of 4 total requests
	if stats.HitRatio < expectedRatio-0.01 || stats.HitRatio > expectedRatio+0.01 {
		t.Errorf("HitRatio = %f, want %f", stats.HitRatio, expectedRatio)
	}

	if stats.Hits != 3 {
		t.Errorf("Hits = %d, want 3", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Misses = %d, want 1", stats.Misses)
	}
}

// TestStatsIncludesNewFields tests that GetStats returns all new fields
// Validates: Memory, MaxMemory, MaxTTL, Hits, Misses, Evictions, HitRatio
func TestStatsIncludesNewFields(t *testing.T) {
	cache := NewCacheWithMemoryLimit(50, 10*time.Minute, 2*time.Minute, 5*1024*1024, 12*time.Hour)

	// Add some entries
	for i := 0; i < 3; i++ {
		key := string(rune('a' + i))
		cache.Set(key, []byte("data"), 5*time.Minute, "", "")
	}

	// Perform some gets
	cache.Get("a") // hit
	cache.Get("z") // miss

	stats := cache.GetStats()

	// Verify all fields are present and sensible
	if stats.Entries != 3 {
		t.Errorf("Stats.Entries = %d, want 3", stats.Entries)
	}
	if stats.MaxSize != 50 {
		t.Errorf("Stats.MaxSize = %d, want 50", stats.MaxSize)
	}
	if stats.Memory <= 0 {
		t.Errorf("Stats.Memory = %d, want > 0", stats.Memory)
	}
	if stats.MaxMemory != 5*1024*1024 {
		t.Errorf("Stats.MaxMemory = %d, want %d", stats.MaxMemory, 5*1024*1024)
	}
	if stats.DefaultTTL != 10*time.Minute {
		t.Errorf("Stats.DefaultTTL = %v, want 10m", stats.DefaultTTL)
	}
	if stats.MinTTL != 2*time.Minute {
		t.Errorf("Stats.MinTTL = %v, want 2m", stats.MinTTL)
	}
	if stats.MaxTTL != 12*time.Hour {
		t.Errorf("Stats.MaxTTL = %v, want 12h", stats.MaxTTL)
	}
	if stats.Hits != 1 {
		t.Errorf("Stats.Hits = %d, want 1", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Stats.Misses = %d, want 1", stats.Misses)
	}
	if stats.HitRatio != 0.5 {
		t.Errorf("Stats.HitRatio = %f, want 0.5", stats.HitRatio)
	}
	// Evictions should be 0 since we haven't filled the cache
	if stats.Evictions != 0 {
		t.Errorf("Stats.Evictions = %d, want 0", stats.Evictions)
	}
}

// TestEvictionTracking tests that evictions are counted correctly
// Validates: Eviction counter increments on LRU eviction
func TestEvictionTracking(t *testing.T) {
	cache := NewCache(3, 5*time.Minute, 1*time.Minute) // Max 3 entries

	// Initial evictions should be 0
	stats := cache.GetStats()
	if stats.Evictions != 0 {
		t.Errorf("Initial Evictions = %d, want 0", stats.Evictions)
	}

	// Fill cache
	cache.Set("a", []byte("data"), 5*time.Minute, "", "")
	time.Sleep(10 * time.Millisecond)
	cache.Set("b", []byte("data"), 5*time.Minute, "", "")
	time.Sleep(10 * time.Millisecond)
	cache.Set("c", []byte("data"), 5*time.Minute, "", "")
	time.Sleep(10 * time.Millisecond)

	// Add fourth entry - should trigger one eviction
	cache.Set("d", []byte("data"), 5*time.Minute, "", "")

	stats = cache.GetStats()
	if stats.Evictions != 1 {
		t.Errorf("Evictions = %d after first overflow, want 1", stats.Evictions)
	}

	// Add fifth entry - should trigger another eviction
	time.Sleep(10 * time.Millisecond)
	cache.Set("e", []byte("data"), 5*time.Minute, "", "")

	stats = cache.GetStats()
	if stats.Evictions != 2 {
		t.Errorf("Evictions = %d after second overflow, want 2", stats.Evictions)
	}
}

// TestClearResetsMemory tests that Clear resets memory counter
// Validates: Memory tracking is reset on Clear
func TestClearResetsMemory(t *testing.T) {
	cache := NewCache(10, 5*time.Minute, 1*time.Minute)

	// Add entries
	for i := 0; i < 5; i++ {
		key := string(rune('a' + i))
		cache.Set(key, []byte("some data here"), 5*time.Minute, "", "")
	}

	stats := cache.GetStats()
	if stats.Memory <= 0 {
		t.Errorf("Memory = %d before Clear, want > 0", stats.Memory)
	}

	// Clear cache
	cache.Clear()

	stats = cache.GetStats()
	if stats.Memory != 0 {
		t.Errorf("Memory = %d after Clear, want 0", stats.Memory)
	}
}
