package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	htmlutil "html"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/linus/recal/internal/config"
	"github.com/linus/recal/internal/feeds"
	"github.com/linus/recal/internal/filter"
	"github.com/linus/recal/internal/parser"
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
	// Pass calendar name via context so ServeHTTP can use it for X-WR-CALNAME
	r.URL.Path = "/query"
	r.URL.RawQuery = queryString
	ctx := context.WithValue(r.Context(), calendarNameKey, feed.Description)
	s.ServeHTTP(w, r.WithContext(ctx))
}

// SlugDebug shows the technical debug page for a named feed (admin tool)
func (s *Server) SlugDebug(w http.ResponseWriter, r *http.Request) {
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
	r.URL.Path = "/query/debug"
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

// SlugPreview shows a user-friendly preview of included/excluded events for a named feed
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
	r.URL.RawQuery = queryString

	// Parse parameters and fetch upstream
	params, err := parseParams(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid parameters: %v", err), http.StatusBadRequest)
		return
	}

	if params.Upstream == "" {
		params.Upstream = s.cfg.Upstream.DefaultURL
	}

	// Fetch upstream feed
	upstreamData, _, err := s.fetchUpstream(r.Context(), params.Upstream)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch upstream: %v", err), http.StatusBadGateway)
		return
	}

	// Parse iCal
	cal, err := parser.Parse(bytes.NewReader(upstreamData))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse iCal: %v", err), http.StatusInternalServerError)
		return
	}

	// Apply filters to determine which events are included/excluded
	engine := filter.NewEngine(s.cfg)
	if err := s.buildFilters(engine, params); err != nil {
		http.Error(w, fmt.Sprintf("Failed to build filters: %v", err), http.StatusBadRequest)
		return
	}

	filteredCal, _ := engine.Apply(cal)

	// Build sets for quick lookup
	includedUIDs := make(map[string]bool)
	for _, e := range filteredCal.Events {
		includedUIDs[e.UID] = true
	}

	// Separate events into included and excluded
	var includedEvents, excludedEvents []*parser.Event
	for _, e := range cal.Events {
		if includedUIDs[e.UID] {
			includedEvents = append(includedEvents, e)
		} else {
			excludedEvents = append(excludedEvents, e)
		}
	}

	// Sort by date (parse DTStart)
	sortEventsByDate(includedEvents)
	sortEventsByDate(excludedEvents)

	// Generate the preview HTML
	preferSwedish := prefersSwedish(r)
	backURL := "/feed/" + slug + "/edit"

	html := s.generatePreviewHTML(feed.Description, includedEvents, excludedEvents, backURL, preferSwedish)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}

// sortEventsByDate sorts events by their start date
func sortEventsByDate(events []*parser.Event) {
	sort.Slice(events, func(i, j int) bool {
		ti := parseEventDate(events[i].DTStart)
		tj := parseEventDate(events[j].DTStart)
		return ti.Before(tj)
	})
}

// parseEventDate parses an iCal date/datetime string
func parseEventDate(dtStart string) time.Time {
	// Try various formats
	formats := []string{
		"20060102T150405Z",
		"20060102T150405",
		"20060102",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, dtStart); err == nil {
			return t
		}
	}
	return time.Time{}
}

// generatePreviewHTML generates the user-friendly preview HTML
func (s *Server) generatePreviewHTML(feedName string, included, excluded []*parser.Event, backURL string, preferSwedish bool) string {
	// Labels
	pageTitle := "Calendar Preview"
	feedNameLabel := "Feed"
	includedTitle := "Events in your calendar"
	excludedTitle := "Events filtered out"
	showingLabel := "showing"
	ofLabel := "of"
	noEventsLabel := "No events"
	backText := "← Back to settings"

	if preferSwedish {
		pageTitle = "Kalenderförhandsvisning"
		feedNameLabel = "Feed"
		includedTitle = "Händelser i din kalender"
		excludedTitle = "Bortfiltrerade händelser"
		showingLabel = "visar"
		ofLabel = "av"
		noEventsLabel = "Inga händelser"
		backText = "← Tillbaka till inställningar"
	}

	langAttr := "en"
	if preferSwedish {
		langAttr = "sv"
	}

	// Limit display to 20 events initially
	displayLimit := 20

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf(`<!DOCTYPE html>
<html lang="%s">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s - ReCal</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100vh;
            padding: 20px;
        }
        .container {
            background: white;
            max-width: 900px;
            margin: 0 auto;
            border-radius: 12px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            padding: 40px;
        }
        h1 { color: #333; font-size: 28px; margin-bottom: 10px; }
        .feed-name { color: #666; font-size: 16px; margin-bottom: 30px; }
        .section { margin-bottom: 30px; }
        .section-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 15px;
            padding-bottom: 10px;
            border-bottom: 2px solid #e9ecef;
        }
        .section-title {
            font-size: 18px;
            font-weight: 600;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .section-title.included { color: #28a745; }
        .section-title.excluded { color: #dc3545; }
        .count { font-size: 14px; color: #666; }
        .event-list {
            max-height: 400px;
            overflow-y: auto;
            border: 1px solid #e9ecef;
            border-radius: 8px;
        }
        .event-item {
            padding: 12px 15px;
            border-bottom: 1px solid #f0f0f0;
            display: flex;
            gap: 15px;
            align-items: flex-start;
        }
        .event-item:last-child { border-bottom: none; }
        .event-item:hover { background: #f8f9fa; }
        .event-date {
            min-width: 100px;
            font-size: 13px;
            color: #666;
            font-weight: 500;
        }
        .event-summary {
            flex: 1;
            font-size: 14px;
            color: #333;
        }
        .no-events {
            padding: 40px;
            text-align: center;
            color: #666;
        }
        .back-link {
            display: inline-block;
            margin-top: 30px;
            padding: 12px 24px;
            background: #667eea;
            color: white;
            text-decoration: none;
            border-radius: 6px;
            font-weight: 600;
            transition: all 0.2s;
        }
        .back-link:hover {
            background: #5568d3;
            transform: translateY(-1px);
        }
        .hidden { display: none; }
        .show-more-btn {
            display: block;
            width: 100%%;
            padding: 10px;
            background: #f8f9fa;
            border: 1px solid #e9ecef;
            border-top: none;
            border-radius: 0 0 8px 8px;
            cursor: pointer;
            color: #667eea;
            font-weight: 600;
            font-size: 14px;
        }
        .show-more-btn:hover { background: #e9ecef; }
    </style>
</head>
<body>
    <div class="container">
        <h1>%s</h1>
        <p class="feed-name">%s: %s</p>
`, langAttr, pageTitle, pageTitle, feedNameLabel, htmlutil.EscapeString(feedName)))

	// Included events section
	buf.WriteString(fmt.Sprintf(`
        <div class="section">
            <div class="section-header">
                <span class="section-title included">✅ %s</span>
                <span class="count">%s %d %s %d</span>
            </div>
`, includedTitle, showingLabel, min(displayLimit, len(included)), ofLabel, len(included)))

	if len(included) == 0 {
		buf.WriteString(fmt.Sprintf(`            <div class="event-list"><div class="no-events">%s</div></div>
`, noEventsLabel))
	} else {
		buf.WriteString(`            <div class="event-list" id="included-list">
`)
		for i, e := range included {
			hiddenClass := ""
			if i >= displayLimit {
				hiddenClass = " hidden"
			}
			dateStr := formatEventDate(e.DTStart, preferSwedish)
			buf.WriteString(fmt.Sprintf(`                <div class="event-item%s">
                    <span class="event-date">%s</span>
                    <span class="event-summary">%s</span>
                </div>
`, hiddenClass, dateStr, htmlutil.EscapeString(e.Summary)))
		}
		buf.WriteString(`            </div>
`)
		if len(included) > displayLimit {
			showMoreText := "Show all"
			if preferSwedish {
				showMoreText = "Visa alla"
			}
			buf.WriteString(fmt.Sprintf(`            <button class="show-more-btn" onclick="showAll('included-list', this)">%s (%d)</button>
`, showMoreText, len(included)))
		}
	}
	buf.WriteString(`        </div>
`)

	// Excluded events section
	buf.WriteString(fmt.Sprintf(`
        <div class="section">
            <div class="section-header">
                <span class="section-title excluded">❌ %s</span>
                <span class="count">%s %d %s %d</span>
            </div>
`, excludedTitle, showingLabel, min(displayLimit, len(excluded)), ofLabel, len(excluded)))

	if len(excluded) == 0 {
		buf.WriteString(fmt.Sprintf(`            <div class="event-list"><div class="no-events">%s</div></div>
`, noEventsLabel))
	} else {
		buf.WriteString(`            <div class="event-list" id="excluded-list">
`)
		for i, e := range excluded {
			hiddenClass := ""
			if i >= displayLimit {
				hiddenClass = " hidden"
			}
			dateStr := formatEventDate(e.DTStart, preferSwedish)
			buf.WriteString(fmt.Sprintf(`                <div class="event-item%s">
                    <span class="event-date">%s</span>
                    <span class="event-summary">%s</span>
                </div>
`, hiddenClass, dateStr, htmlutil.EscapeString(e.Summary)))
		}
		buf.WriteString(`            </div>
`)
		if len(excluded) > displayLimit {
			showMoreText := "Show all"
			if preferSwedish {
				showMoreText = "Visa alla"
			}
			buf.WriteString(fmt.Sprintf(`            <button class="show-more-btn" onclick="showAll('excluded-list', this)">%s (%d)</button>
`, showMoreText, len(excluded)))
		}
	}
	buf.WriteString(`        </div>
`)

	// Back link and script
	buf.WriteString(fmt.Sprintf(`
        <a href="%s" class="back-link">%s</a>
    </div>

    <script>
        function showAll(listId, btn) {
            document.querySelectorAll('#' + listId + ' .hidden').forEach(el => {
                el.classList.remove('hidden');
            });
            btn.style.display = 'none';
        }
    </script>
</body>
</html>`, htmlutil.EscapeString(backURL), backText))

	return buf.String()
}

// formatEventDate formats an iCal date for display
func formatEventDate(dtStart string, preferSwedish bool) string {
	t := parseEventDate(dtStart)
	if t.IsZero() {
		return dtStart
	}
	if preferSwedish {
		return t.Format("2 jan 2006")
	}
	return t.Format("Jan 2, 2006")
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

	// Update feed (admin can set owner)
	feed, err := s.feedManager.UpdateWithOwner(slug, req.Description, req.Filters, req.Owner)
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
