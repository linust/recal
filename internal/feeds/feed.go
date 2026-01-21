package feeds

import (
	"time"
)

// NamedFeed represents a saved filter configuration with a persistent UUID slug
type NamedFeed struct {
	Slug        string            `json:"slug"`         // UUID v4
	Description string            `json:"description"`  // User-provided description
	CreatedAt   time.Time         `json:"created_at"`   // Creation timestamp
	UpdatedAt   time.Time         `json:"updated_at"`   // Last update timestamp
	Filters     map[string]string `json:"filters"`      // Filter parameters (e.g., {"Grad": "3", "Loge": "Göta"})
	AccessCount int64             `json:"access_count"` // Number of times accessed
	LastAccess  time.Time         `json:"last_access"`  // Last access timestamp
	Owner       string            `json:"owner"`        // Optional: user identifier (for future multi-tenancy)
}

// FeedCreateRequest represents the request to create a new feed
type FeedCreateRequest struct {
	Description string            `json:"description"` // User-provided description
	Filters     map[string]string `json:"filters"`     // Filter parameters
}

// FeedUpdateRequest represents the request to update an existing feed
type FeedUpdateRequest struct {
	Description string            `json:"description,omitempty"` // Updated description (optional)
	Filters     map[string]string `json:"filters,omitempty"`     // Updated filters (optional)
	Owner       *string           `json:"owner,omitempty"`       // Updated owner (optional, admin only)
}

// FeedResponse represents the response when creating or getting a feed
type FeedResponse struct {
	Slug        string            `json:"slug"`
	Description string            `json:"description"`
	URL         string            `json:"url"`
	ConfigURL   string            `json:"config_url"`
	PreviewURL  string            `json:"preview_url,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at,omitempty"`
	Filters     map[string]string `json:"filters,omitempty"`
	AccessCount int64             `json:"access_count,omitempty"`
	LastAccess  time.Time         `json:"last_access,omitempty"`
	Owner       string            `json:"owner,omitempty"`
}

// FeedListResponse represents the response for listing all feeds
type FeedListResponse struct {
	Feeds []FeedSummary `json:"feeds"`
	Total int           `json:"total"`
}

// FeedSummary represents a summary of a feed for listing
type FeedSummary struct {
	Slug        string            `json:"slug"`
	Description string            `json:"description"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	AccessCount int64             `json:"access_count"`
	LastAccess  time.Time         `json:"last_access"`
	Owner       string            `json:"owner,omitempty"`
	Filters     map[string]string `json:"filters,omitempty"`
}

// FeedStats represents statistics about a feed
type FeedStats struct {
	Slug        string    `json:"slug"`
	AccessCount int64     `json:"access_count"`
	LastAccess  time.Time `json:"last_access"`
	CreatedAt   time.Time `json:"created_at"`
	AgeDays     int       `json:"age_days"`
}

// Validate validates a feed creation request
func (r *FeedCreateRequest) Validate() error {
	if r.Description == "" {
		return ErrEmptyDescription
	}
	if len(r.Description) > 500 {
		return ErrDescriptionTooLong
	}
	if len(r.Filters) == 0 {
		return ErrNoFilters
	}
	return nil
}

// Validate validates a feed update request
func (r *FeedUpdateRequest) Validate() error {
	if r.Description != "" && len(r.Description) > 500 {
		return ErrDescriptionTooLong
	}
	return nil
}
