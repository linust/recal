package feeds

import (
	"fmt"
	"log"
	"time"
)

// Manager manages named feeds with caching
type Manager struct {
	store Store
	cache *Cache
}

// NewManager creates a new feed manager
func NewManager(store Store, cacheMaxAge time.Duration) *Manager {
	m := &Manager{
		store: store,
		cache: NewCache(cacheMaxAge),
	}

	// Start background cleanup goroutine
	go m.cleanupLoop()

	return m
}

// Create creates a new named feed
func (m *Manager) Create(description string, filters map[string]string) (*NamedFeed, error) {
	// Validate
	req := FeedCreateRequest{
		Description: description,
		Filters:     filters,
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Create in store
	feed, err := m.store.Create(description, filters)
	if err != nil {
		return nil, err
	}

	// Cache the new feed
	m.cache.Set(feed.Slug, feed)

	log.Printf("Created feed: slug=%s description=%q", feed.Slug, feed.Description)
	return feed, nil
}

// Get retrieves a feed by slug (with caching)
func (m *Manager) Get(slug string) (*NamedFeed, error) {
	// Try cache first
	if feed, ok := m.cache.Get(slug); ok {
		return feed, nil
	}

	// Cache miss - load from store
	feed, err := m.store.Get(slug)
	if err != nil {
		return nil, err
	}

	// Cache for next time
	m.cache.Set(slug, feed)

	return feed, nil
}

// Update updates a feed
func (m *Manager) Update(slug string, description string, filters map[string]string) (*NamedFeed, error) {
	return m.UpdateWithOwner(slug, description, filters, nil)
}

// UpdateWithOwner updates a feed including the owner field
func (m *Manager) UpdateWithOwner(slug string, description string, filters map[string]string, owner *string) (*NamedFeed, error) {
	// Validate
	req := FeedUpdateRequest{
		Description: description,
		Filters:     filters,
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Update in store
	feed, err := m.store.Update(slug, description, filters, owner)
	if err != nil {
		return nil, err
	}

	// Invalidate cache (will be reloaded on next access)
	m.cache.Invalidate(slug)

	log.Printf("Updated feed: slug=%s", slug)
	return feed, nil
}

// Delete deletes a feed
func (m *Manager) Delete(slug string) error {
	// Delete from store
	if err := m.store.Delete(slug); err != nil {
		return err
	}

	// Invalidate cache
	m.cache.Invalidate(slug)

	log.Printf("Deleted feed: slug=%s", slug)
	return nil
}

// List returns all feeds
func (m *Manager) List() ([]*NamedFeed, error) {
	return m.store.List()
}

// IncrementAccessCount increments the access count for a feed
func (m *Manager) IncrementAccessCount(slug string) error {
	// Update in store
	if err := m.store.IncrementAccessCount(slug); err != nil {
		return err
	}

	// Invalidate cache (will be reloaded on next access with updated count)
	m.cache.Invalidate(slug)

	return nil
}

// GetStats returns statistics for a feed
func (m *Manager) GetStats(slug string) (*FeedStats, error) {
	feed, err := m.Get(slug)
	if err != nil {
		return nil, err
	}

	ageDays := int(time.Since(feed.CreatedAt).Hours() / 24)

	return &FeedStats{
		Slug:        feed.Slug,
		AccessCount: feed.AccessCount,
		LastAccess:  feed.LastAccess,
		CreatedAt:   feed.CreatedAt,
		AgeDays:     ageDays,
	}, nil
}

// cleanupLoop periodically cleans expired cache entries
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		removed := m.cache.CleanExpired()
		if removed > 0 {
			log.Printf("Cleaned %d expired feed cache entries", removed)
		}
	}
}

// ToResponse converts a NamedFeed to a FeedResponse with URLs
func ToResponse(feed *NamedFeed, baseURL string) *FeedResponse {
	return &FeedResponse{
		Slug:        feed.Slug,
		Description: feed.Description,
		URL:         fmt.Sprintf("%s/feed/%s", baseURL, feed.Slug),
		ConfigURL:   fmt.Sprintf("%s/feed/%s/config", baseURL, feed.Slug),
		PreviewURL:  fmt.Sprintf("%s/feed/%s/preview", baseURL, feed.Slug),
		CreatedAt:   feed.CreatedAt,
		UpdatedAt:   feed.UpdatedAt,
		Filters:     feed.Filters,
		AccessCount: feed.AccessCount,
		LastAccess:  feed.LastAccess,
		Owner:       feed.Owner,
	}
}

// ToSummary converts a NamedFeed to a FeedSummary
func ToSummary(feed *NamedFeed) FeedSummary {
	return FeedSummary{
		Slug:        feed.Slug,
		Description: feed.Description,
		CreatedAt:   feed.CreatedAt,
		UpdatedAt:   feed.UpdatedAt,
		AccessCount: feed.AccessCount,
		LastAccess:  feed.LastAccess,
		Owner:       feed.Owner,
		Filters:     feed.Filters,
	}
}
