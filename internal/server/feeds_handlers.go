package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/linus/recal/internal/config"
	"github.com/linus/recal/internal/feeds"
)

// SlugFeed serves the iCal feed for a named feed
func (s *Server) SlugFeed(w http.ResponseWriter, r *http.Request) {
	// Record request metrics
	s.requestMetrics.RecordRequest()

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract slug from path
	slug := extractSlugFromPath(r.URL.Path, "/feed/")
	if slug == "" {
		http.Error(w, "Invalid slug", http.StatusBadRequest)
		return
	}

	// Get feed from manager
	feed, err := s.feedManager.Get(slug)
	if err != nil {
		if err == feeds.ErrFeedNotFound || err == feeds.ErrInvalidUUID {
			http.Error(w, "Feed not found", http.StatusNotFound)
			return
		}
		log.Printf("Error getting feed %s: %v", slug, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Increment access count (async, don't block on errors)
	go func() {
		if err := s.feedManager.IncrementAccessCount(slug); err != nil {
			log.Printf("Error incrementing access count for feed %s: %v", slug, err)
		}
	}()

	// Build query string from filters and redirect to /query
	queryString := s.buildQueryStringFromFilters(feed.Filters)

	// Internal redirect by updating the request URL and calling ServeHTTP
	r.URL.Path = "/query"
	r.URL.RawQuery = queryString
	s.ServeHTTP(w, r)
}

// SlugPreview shows the debug/preview page for a named feed
func (s *Server) SlugPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract slug from path
	slug := extractSlugFromPath(r.URL.Path, "/feed/")
	if slug == "" {
		http.Error(w, "Invalid slug", http.StatusBadRequest)
		return
	}

	// Get feed from manager
	feed, err := s.feedManager.Get(slug)
	if err != nil {
		if err == feeds.ErrFeedNotFound || err == feeds.ErrInvalidUUID {
			http.Error(w, "Feed not found", http.StatusNotFound)
			return
		}
		log.Printf("Error getting feed %s: %v", slug, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build query string from filters
	queryString := s.buildQueryStringFromFilters(feed.Filters)

	// Update the request URL for parameter parsing
	r.URL.Path = "/query/preview"
	r.URL.RawQuery = queryString

	// Determine back URL based on referer - go to admin if came from there, otherwise to feed edit page
	backURL := "/feed/" + slug + "/edit"
	if referer := r.Header.Get("Referer"); referer != "" {
		if strings.Contains(referer, "/admin") {
			backURL = "/admin"
		}
	}

	s.serveDebugPage(w, r, backURL)
}

// AdminCreateFeed creates a new named feed (admin endpoint)
func (s *Server) AdminCreateFeed(w http.ResponseWriter, r *http.Request) {
	// Record request metrics
	s.requestMetrics.RecordRequest()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req feeds.FeedCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Validation error: %v", err), http.StatusBadRequest)
		return
	}

	// Create feed
	feed, err := s.feedManager.Create(req.Description, req.Filters)
	if err != nil {
		log.Printf("Error creating feed: %v", err)
		http.Error(w, "Failed to create feed", http.StatusInternalServerError)
		return
	}

	// Build response
	response := feeds.ToResponse(feed, s.cfg.Server.BaseURL)

	// Return JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode create feed response: %v", err)
	}
}

// AdminListFeeds lists all named feeds with pagination and search (admin endpoint)
func (s *Server) AdminListFeeds(w http.ResponseWriter, r *http.Request) {
	// Record request metrics
	s.requestMetrics.RecordRequest()

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters for pagination and search
	query := r.URL.Query()
	page := 1
	pageSize := 50
	searchQuery := strings.TrimSpace(query.Get("q"))

	if pageStr := query.Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if sizeStr := query.Get("page_size"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 && s <= 200 {
			pageSize = s
		}
	}

	// Get all feeds
	allFeeds, err := s.feedManager.List()
	if err != nil {
		log.Printf("Error listing feeds: %v", err)
		http.Error(w, "Failed to list feeds", http.StatusInternalServerError)
		return
	}

	// Filter by search query if provided
	var filteredFeeds []*feeds.NamedFeed
	if searchQuery != "" {
		searchLower := strings.ToLower(searchQuery)
		for _, feed := range allFeeds {
			// Search in description and filter values
			if strings.Contains(strings.ToLower(feed.Description), searchLower) ||
				strings.Contains(strings.ToLower(feed.Slug), searchLower) {
				filteredFeeds = append(filteredFeeds, feed)
				continue
			}
			// Search in filter values
			for _, v := range feed.Filters {
				if strings.Contains(strings.ToLower(v), searchLower) {
					filteredFeeds = append(filteredFeeds, feed)
					break
				}
			}
		}
	} else {
		filteredFeeds = allFeeds
	}

	totalFiltered := len(filteredFeeds)

	// Sort by most recently accessed (or created if never accessed)
	sort.Slice(filteredFeeds, func(i, j int) bool {
		iTime := filteredFeeds[i].LastAccess
		if iTime.IsZero() {
			iTime = filteredFeeds[i].CreatedAt
		}
		jTime := filteredFeeds[j].LastAccess
		if jTime.IsZero() {
			jTime = filteredFeeds[j].CreatedAt
		}
		return iTime.After(jTime)
	})

	// Apply pagination
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= len(filteredFeeds) {
		start = 0
		end = 0
	}
	if end > len(filteredFeeds) {
		end = len(filteredFeeds)
	}

	pagedFeeds := filteredFeeds[start:end]

	// Build summaries
	summaries := make([]feeds.FeedSummary, len(pagedFeeds))
	for i, feed := range pagedFeeds {
		summaries[i] = feeds.ToSummary(feed)
	}

	// Build response with pagination metadata
	response := map[string]interface{}{
		"feeds":       summaries,
		"total":       len(allFeeds),
		"filtered":    totalFiltered,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (totalFiltered + pageSize - 1) / pageSize,
	}

	// Return JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode feeds list response: %v", err)
	}
}

// AdminGetFeed gets details of a specific feed (admin endpoint)
func (s *Server) AdminGetFeed(w http.ResponseWriter, r *http.Request) {
	// Record request metrics
	s.requestMetrics.RecordRequest()

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract slug from path
	slug := extractSlugFromPath(r.URL.Path, "/admin/feeds/")
	if slug == "" {
		http.Error(w, "Invalid slug", http.StatusBadRequest)
		return
	}

	// Get feed
	feed, err := s.feedManager.Get(slug)
	if err != nil {
		if err == feeds.ErrFeedNotFound || err == feeds.ErrInvalidUUID {
			http.Error(w, "Feed not found", http.StatusNotFound)
			return
		}
		log.Printf("Error getting feed %s: %v", slug, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build response
	response := feeds.ToResponse(feed, s.cfg.Server.BaseURL)

	// Return JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode feed response: %v", err)
	}
}

// AdminUpdateFeed updates a feed (admin endpoint)
func (s *Server) AdminUpdateFeed(w http.ResponseWriter, r *http.Request) {
	// Record request metrics
	s.requestMetrics.RecordRequest()

	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract slug from path
	slug := extractSlugFromPath(r.URL.Path, "/admin/feeds/")
	if slug == "" {
		http.Error(w, "Invalid slug", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req feeds.FeedUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Validation error: %v", err), http.StatusBadRequest)
		return
	}

	// Update feed
	feed, err := s.feedManager.Update(slug, req.Description, req.Filters)
	if err != nil {
		if err == feeds.ErrFeedNotFound || err == feeds.ErrInvalidUUID {
			http.Error(w, "Feed not found", http.StatusNotFound)
			return
		}
		log.Printf("Error updating feed %s: %v", slug, err)
		http.Error(w, "Failed to update feed", http.StatusInternalServerError)
		return
	}

	// Build response
	response := feeds.ToResponse(feed, s.cfg.Server.BaseURL)

	// Return JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode updated feed response: %v", err)
	}
}

// AdminDeleteFeed deletes a feed (admin endpoint)
func (s *Server) AdminDeleteFeed(w http.ResponseWriter, r *http.Request) {
	// Record request metrics
	s.requestMetrics.RecordRequest()

	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract slug from path
	slug := extractSlugFromPath(r.URL.Path, "/admin/feeds/")
	if slug == "" {
		http.Error(w, "Invalid slug", http.StatusBadRequest)
		return
	}

	// Delete feed
	if err := s.feedManager.Delete(slug); err != nil {
		if err == feeds.ErrFeedNotFound || err == feeds.ErrInvalidUUID {
			http.Error(w, "Feed not found", http.StatusNotFound)
			return
		}
		log.Printf("Error deleting feed %s: %v", slug, err)
		http.Error(w, "Failed to delete feed", http.StatusInternalServerError)
		return
	}

	// Return 204 No Content
	w.WriteHeader(http.StatusNoContent)
}

// AdminGetFeedStats gets statistics for a feed (admin endpoint)
func (s *Server) AdminGetFeedStats(w http.ResponseWriter, r *http.Request) {
	// Record request metrics
	s.requestMetrics.RecordRequest()

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract slug from path
	slug := extractSlugFromPath(r.URL.Path, "/admin/feeds/")
	// Remove /stats suffix
	slug = strings.TrimSuffix(slug, "/stats")
	if slug == "" {
		http.Error(w, "Invalid slug", http.StatusBadRequest)
		return
	}

	// Get stats
	stats, err := s.feedManager.GetStats(slug)
	if err != nil {
		if err == feeds.ErrFeedNotFound || err == feeds.ErrInvalidUUID {
			http.Error(w, "Feed not found", http.StatusNotFound)
			return
		}
		log.Printf("Error getting stats for feed %s: %v", slug, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Return JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.Printf("Failed to encode feed stats response: %v", err)
	}
}

// extractSlugFromPath extracts the slug from a URL path
// Example: "/feed/a1b2c3d4-..." with prefix "/feed/" returns "a1b2c3d4-..."
func extractSlugFromPath(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}

	// Remove prefix
	slug := strings.TrimPrefix(path, prefix)

	// Remove trailing slashes and path segments
	if idx := strings.Index(slug, "/"); idx > 0 {
		slug = slug[:idx]
	}

	return strings.TrimSpace(slug)
}

// prefersSwedish checks if the client prefers Swedish based on Accept-Language header
func prefersSwedish(r *http.Request) bool {
	acceptLang := r.Header.Get("Accept-Language")
	if acceptLang == "" {
		return false
	}

	// Simple check: if "sv" appears before "en" or if "sv" is present and "en" is not
	svIdx := strings.Index(strings.ToLower(acceptLang), "sv")
	enIdx := strings.Index(strings.ToLower(acceptLang), "en")

	if svIdx == -1 {
		return false // Swedish not in preferences
	}
	if enIdx == -1 {
		return true // Swedish present, English not
	}
	return svIdx < enIdx // Swedish appears before English
}

// buildQueryStringFromFilters builds a query string from filter parameters
func (s *Server) buildQueryStringFromFilters(filters map[string]string) string {
	var parts []string

	for key, value := range filters {
		if value != "" {
			// Normalize Swedish characters in Loge filter for cleaner URLs
			if key == "Loge" {
				value = config.NormalizeSwedish(value)
			}
			parts = append(parts, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return strings.Join(parts, "&")
}
