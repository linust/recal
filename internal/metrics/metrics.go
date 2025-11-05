package metrics

import (
	"sync"
	"time"
)

// RequestMetrics tracks HTTP request statistics
type RequestMetrics struct {
	mu          sync.RWMutex
	requests5m  []time.Time // Last 5 minutes
	requests1h  []time.Time // Last 1 hour
	requests24h []time.Time // Last 24 hours
}

// NewRequestMetrics creates a new request metrics tracker
func NewRequestMetrics() *RequestMetrics {
	m := &RequestMetrics{
		requests5m:  make([]time.Time, 0),
		requests1h:  make([]time.Time, 0),
		requests24h: make([]time.Time, 0),
	}
	// Start background cleanup goroutine
	go m.cleanup()
	return m
}

// RecordRequest records a new request
func (m *RequestMetrics) RecordRequest() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.requests5m = append(m.requests5m, now)
	m.requests1h = append(m.requests1h, now)
	m.requests24h = append(m.requests24h, now)
}

// GetStats returns request counts for different time windows
func (m *RequestMetrics) GetStats() (count5m, count1h, count24h int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	cutoff5m := now.Add(-5 * time.Minute)
	cutoff1h := now.Add(-1 * time.Hour)
	cutoff24h := now.Add(-24 * time.Hour)

	// Count requests within each time window
	for _, t := range m.requests5m {
		if t.After(cutoff5m) {
			count5m++
		}
	}

	for _, t := range m.requests1h {
		if t.After(cutoff1h) {
			count1h++
		}
	}

	for _, t := range m.requests24h {
		if t.After(cutoff24h) {
			count24h++
		}
	}

	return
}

// cleanup removes old entries periodically
func (m *RequestMetrics) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()

		now := time.Now()
		cutoff5m := now.Add(-5 * time.Minute)
		cutoff1h := now.Add(-1 * time.Hour)
		cutoff24h := now.Add(-24 * time.Hour)

		// Clean 5 minute window
		m.requests5m = filterOldRequests(m.requests5m, cutoff5m)

		// Clean 1 hour window
		m.requests1h = filterOldRequests(m.requests1h, cutoff1h)

		// Clean 24 hour window
		m.requests24h = filterOldRequests(m.requests24h, cutoff24h)

		m.mu.Unlock()
	}
}

// filterOldRequests removes requests older than cutoff
func filterOldRequests(requests []time.Time, cutoff time.Time) []time.Time {
	filtered := make([]time.Time, 0, len(requests))
	for _, t := range requests {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// CacheMetrics tracks cache performance statistics
type CacheMetrics struct {
	mu    sync.RWMutex
	hits  int64
	misses int64
}

// NewCacheMetrics creates a new cache metrics tracker
func NewCacheMetrics() *CacheMetrics {
	return &CacheMetrics{}
}

// RecordHit records a cache hit
func (m *CacheMetrics) RecordHit() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hits++
}

// RecordMiss records a cache miss
func (m *CacheMetrics) RecordMiss() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.misses++
}

// GetStats returns cache hit/miss statistics
func (m *CacheMetrics) GetStats() (hits, misses int64, ratio float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hits = m.hits
	misses = m.misses
	total := hits + misses

	if total > 0 {
		ratio = float64(hits) / float64(total)
	}

	return
}

// Reset resets all metrics
func (m *CacheMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hits = 0
	m.misses = 0
}
