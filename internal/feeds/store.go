package feeds

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Store defines the interface for feed persistence
type Store interface {
	// Create creates a new feed with a generated UUID
	Create(description string, filters map[string]string) (*NamedFeed, error)

	// Get retrieves a feed by slug
	Get(slug string) (*NamedFeed, error)

	// Update updates an existing feed
	Update(slug string, description string, filters map[string]string) (*NamedFeed, error)

	// Delete removes a feed
	Delete(slug string) error

	// List returns all feeds
	List() ([]*NamedFeed, error)

	// IncrementAccessCount increments the access count and updates last access time
	IncrementAccessCount(slug string) error
}

// FileStore implements Store using file-based persistence
type FileStore struct {
	mu          sync.RWMutex
	storagePath string
}

// NewFileStore creates a new file-based store
func NewFileStore(storagePath string) (*FileStore, error) {
	// Create storage directory if it doesn't exist
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &FileStore{
		storagePath: storagePath,
	}, nil
}

// Create creates a new feed with a generated UUID
func (s *FileStore) Create(description string, filters map[string]string) (*NamedFeed, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate UUID v4
	slug := uuid.New().String()

	// Check if file already exists (extremely unlikely with UUID v4)
	feedPath := s.getFeedPath(slug)
	if _, err := os.Stat(feedPath); err == nil {
		return nil, ErrFeedAlreadyExists
	}

	// Create feed
	now := time.Now()
	feed := &NamedFeed{
		Slug:        slug,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
		Filters:     filters,
		AccessCount: 0,
		LastAccess:  time.Time{}, // Zero value
	}

	// Save to disk
	if err := s.saveFeed(feed); err != nil {
		return nil, fmt.Errorf("failed to save feed: %w", err)
	}

	return feed, nil
}

// Get retrieves a feed by slug
func (s *FileStore) Get(slug string) (*NamedFeed, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Validate UUID format
	if _, err := uuid.Parse(slug); err != nil {
		return nil, ErrInvalidUUID
	}

	feedPath := s.getFeedPath(slug)
	data, err := os.ReadFile(feedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFeedNotFound
		}
		return nil, fmt.Errorf("failed to read feed: %w", err)
	}

	var feed NamedFeed
	if err := json.Unmarshal(data, &feed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal feed: %w", err)
	}

	return &feed, nil
}

// Update updates an existing feed
func (s *FileStore) Update(slug string, description string, filters map[string]string) (*NamedFeed, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate UUID format
	if _, err := uuid.Parse(slug); err != nil {
		return nil, ErrInvalidUUID
	}

	// Get existing feed
	feedPath := s.getFeedPath(slug)
	data, err := os.ReadFile(feedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFeedNotFound
		}
		return nil, fmt.Errorf("failed to read feed: %w", err)
	}

	var feed NamedFeed
	if err := json.Unmarshal(data, &feed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal feed: %w", err)
	}

	// Update fields
	if description != "" {
		feed.Description = description
	}
	if len(filters) > 0 {
		feed.Filters = filters
	}
	feed.UpdatedAt = time.Now()

	// Save to disk
	if err := s.saveFeed(&feed); err != nil {
		return nil, fmt.Errorf("failed to save feed: %w", err)
	}

	return &feed, nil
}

// Delete removes a feed
func (s *FileStore) Delete(slug string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate UUID format
	if _, err := uuid.Parse(slug); err != nil {
		return ErrInvalidUUID
	}

	feedPath := s.getFeedPath(slug)
	if err := os.Remove(feedPath); err != nil {
		if os.IsNotExist(err) {
			return ErrFeedNotFound
		}
		return fmt.Errorf("failed to delete feed: %w", err)
	}

	return nil
}

// List returns all feeds
func (s *FileStore) List() ([]*NamedFeed, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.storagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read storage directory: %w", err)
	}

	var feeds []*NamedFeed
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		// Read feed
		data, err := os.ReadFile(filepath.Join(s.storagePath, entry.Name()))
		if err != nil {
			continue // Skip files that can't be read
		}

		var feed NamedFeed
		if err := json.Unmarshal(data, &feed); err != nil {
			continue // Skip invalid JSON
		}

		feeds = append(feeds, &feed)
	}

	return feeds, nil
}

// IncrementAccessCount increments the access count and updates last access time
func (s *FileStore) IncrementAccessCount(slug string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate UUID format
	if _, err := uuid.Parse(slug); err != nil {
		return ErrInvalidUUID
	}

	// Get existing feed
	feedPath := s.getFeedPath(slug)
	data, err := os.ReadFile(feedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrFeedNotFound
		}
		return fmt.Errorf("failed to read feed: %w", err)
	}

	var feed NamedFeed
	if err := json.Unmarshal(data, &feed); err != nil {
		return fmt.Errorf("failed to unmarshal feed: %w", err)
	}

	// Update access info
	feed.AccessCount++
	feed.LastAccess = time.Now()

	// Save to disk
	if err := s.saveFeed(&feed); err != nil {
		return fmt.Errorf("failed to save feed: %w", err)
	}

	return nil
}

// getFeedPath returns the filesystem path for a feed
func (s *FileStore) getFeedPath(slug string) string {
	return filepath.Join(s.storagePath, slug+".json")
}

// saveFeed saves a feed to disk (atomic write)
func (s *FileStore) saveFeed(feed *NamedFeed) error {
	data, err := json.MarshalIndent(feed, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal feed: %w", err)
	}

	// Write to temporary file first (atomic write pattern)
	feedPath := s.getFeedPath(feed.Slug)
	tmpPath := feedPath + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Rename temp file to final location (atomic on Unix)
	if err := os.Rename(tmpPath, feedPath); err != nil {
		os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}
