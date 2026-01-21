package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	htmlutil "html"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/linus/recal/internal/cache"
	"github.com/linus/recal/internal/config"
	"github.com/linus/recal/internal/fetcher"
	"github.com/linus/recal/internal/feeds"
	"github.com/linus/recal/internal/filter"
	"github.com/linus/recal/internal/metrics"
	"github.com/linus/recal/internal/parser"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// Server is the HTTP server for the ReCal application
type Server struct {
	cfg            *config.Config
	upstreamCache  *cache.Cache
	filteredCache  *cache.Cache
	fetcher        *fetcher.Fetcher
	requestMetrics *metrics.RequestMetrics
	startTime      time.Time
	feedManager    *feeds.Manager
}

// Context keys for passing data between handlers
type contextKey string

const calendarNameKey contextKey = "calendarName"

// New creates a new server
func New(cfg *config.Config) *Server {
	// Check if SSRF protection should be disabled (for testing only)
	// This allows CI tests to access localhost for test data
	var f *fetcher.Fetcher
	if os.Getenv("DISABLE_SSRF_PROTECTION") == "true" {
		log.Println("WARNING: SSRF protection disabled (test mode)")
		f = fetcher.NewTestFetcher(cfg)
	} else {
		f = fetcher.NewFetcher(cfg)
	}

	// Initialize feed store and manager
	feedStore, err := feeds.NewFileStore(cfg.Feeds.StoragePath)
	if err != nil {
		log.Fatalf("Failed to initialize feed store: %v", err)
	}
	feedManager := feeds.NewManager(feedStore, cfg.Feeds.CacheMaxAge)

	return &Server{
		cfg: cfg,
		upstreamCache: cache.NewCacheWithMemoryLimit(
			cfg.Cache.MaxSize,
			cfg.Cache.DefaultTTL,
			cfg.Cache.MinOutputCache,
			cfg.Cache.MaxMemory,
			cfg.Cache.MaxTTL,
		),
		filteredCache: cache.NewCacheWithMemoryLimit(
			cfg.Cache.MaxSize*2, // Filtered cache can be larger
			cfg.Cache.DefaultTTL,
			cfg.Cache.MinOutputCache,
			cfg.Cache.MaxMemory*2, // Double memory for filtered cache
			cfg.Cache.MaxTTL,
		),
		fetcher:        f,
		requestMetrics: metrics.NewRequestMetrics(),
		startTime:      time.Now(),
		feedManager:    feedManager,
	}
}

// ServeHTTP handles HTTP requests for filtered iCal feeds
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Record request metrics
	s.requestMetrics.RecordRequest()

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters (debug parameter ignored on /filter endpoint)
	params, err := parseParams(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid parameters: %v", err), http.StatusBadRequest)
		return
	}
	params.Debug = false // Enforce non-debug mode on /filter

	// Check if configure parameter is set - redirect to config page with params
	if _, hasConfig := r.URL.Query()["configure"]; hasConfig {
		// Build query string without the "configure" parameter
		q := r.URL.Query()
		q.Del("configure")
		queryStr := q.Encode()
		redirectURL := "/"
		if queryStr != "" {
			redirectURL += "?" + queryStr
		}
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	// Use default upstream URL if none specified
	if params.Upstream == "" {
		params.Upstream = s.cfg.Upstream.DefaultURL
	}

	// If no filters specified and no upstream available, show configuration page
	if params.Upstream == "" && len(params.Filters) == 0 &&
		params.SpecialFilters.Grad == "" && params.SpecialFilters.Loge == "" &&
		!params.SpecialFilters.RemoveUnconfirmed && !params.SpecialFilters.RemoveInstallt {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Create base cache key (without content hash)
	baseCacheKey := createCacheKey(params)

	// Fetch upstream feed (this also checks upstream cache)
	upstreamData, _, err := s.fetchUpstream(r.Context(), params.Upstream)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch upstream: %v", err), http.StatusBadGateway)
		return
	}

	// Get upstream content hash for content-based cache invalidation
	upstreamContentHash, _ := s.upstreamCache.GetContentHash(params.Upstream)

	// Create filtered cache key including the upstream content hash
	// This ensures filtered cache is invalidated when upstream content changes
	cacheKey := baseCacheKey
	if upstreamContentHash != "" {
		cacheKey = baseCacheKey + ":" + upstreamContentHash[:16] // Use first 16 chars of hash
	}

	// Check filtered cache
	if entry, found := s.filteredCache.Get(cacheKey); found {
		s.serveFromCache(w, entry, false)
		return
	}

	// Parse iCal
	cal, err := parser.Parse(bytes.NewReader(upstreamData))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse iCal: %v", err), http.StatusInternalServerError)
		return
	}

	// Apply filters
	engine := filter.NewEngine(s.cfg)
	if err := s.buildFilters(engine, params); err != nil {
		http.Error(w, fmt.Sprintf("Failed to build filters: %v", err), http.StatusBadRequest)
		return
	}

	filteredCal, _ := engine.Apply(cal)

	// Get calendar name from context (set by SlugFeed for named feeds)
	calendarName := ""
	if name, ok := r.Context().Value(calendarNameKey).(string); ok {
		calendarName = name
	}

	// Serialize iCal with optional calendar name
	var buf bytes.Buffer
	if err := filteredCal.SerializeWithName(&buf, calendarName); err != nil {
		http.Error(w, fmt.Sprintf("Failed to serialize iCal: %v", err), http.StatusInternalServerError)
		return
	}
	output := buf.Bytes()

	// Cache the result with the content-hash-based key
	// Use MinOutputCache as the TTL since the hash ensures invalidation when content changes
	s.filteredCache.Set(cacheKey, output, s.cfg.Cache.MinOutputCache, "", "")

	// Set cache headers for client - use MinOutputCache since content-hash ensures freshness
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(s.cfg.Cache.MinOutputCache.Seconds())))
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(output)
}

// DebugHTTP handles HTTP requests for debug mode (HTML output)
func (s *Server) DebugHTTP(w http.ResponseWriter, r *http.Request) {
	// Build back URL - default to main config page with query string
	backURL := "/"
	if r.URL.RawQuery != "" {
		backURL += "?" + r.URL.RawQuery
	}

	s.serveDebugPage(w, r, backURL)
}

// serveDebugPage generates and serves the debug/preview page with a custom back URL
func (s *Server) serveDebugPage(w http.ResponseWriter, r *http.Request, backURL string) {
	// Record request metrics
	s.requestMetrics.RecordRequest()

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	params, err := parseParams(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid parameters: %v", err), http.StatusBadRequest)
		return
	}
	params.Debug = true // Force debug mode on /debug endpoint

	// If no filters specified and no upstream, show error
	// Use default upstream URL if none specified
	if params.Upstream == "" {
		params.Upstream = s.cfg.Upstream.DefaultURL
	}

	// Fetch upstream feed (no caching for debug mode)
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

	// Apply filters
	engine := filter.NewEngine(s.cfg)
	if err := s.buildFilters(engine, params); err != nil {
		http.Error(w, fmt.Sprintf("Failed to build filters: %v", err), http.StatusBadRequest)
		return
	}

	originalCal := cal
	filteredCal, matches := engine.Apply(cal)

	// Detect language preference from Accept-Language header
	preferSwedish := prefersSwedish(r)

	// Generate debug HTML
	output := s.generateDebugHTML(originalCal, filteredCal, matches, engine, backURL, preferSwedish)

	// No caching for debug mode
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(output))
}

// Version info variables (set by main package)
var (
	ServerVersion   = "dev"
	ServerBuildTime = "unknown"
	ServerGitCommit = "unknown"
)

// Version returns version information
func (s *Server) Version(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if _, err := fmt.Fprintf(w, `{"version":"%s","build_time":"%s","git_commit":"%s"}`,
		ServerVersion, ServerBuildTime, ServerGitCommit); err != nil {
		log.Printf("Failed to write version response: %v", err)
	}
}

// Health handles health check requests
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	stats := s.upstreamCache.GetStats()
	filteredStats := s.filteredCache.GetStats()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"status":"ok","upstream_cache":%d,"filtered_cache":%d}`,
		stats.Entries, filteredStats.Entries)
}

// Status handles status page requests with metrics and cache statistics
func (s *Server) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	preferSwedish := prefersSwedish(r)
	langAttr := "en"
	pageTitle := "ReCal - Status"
	requestMetricsTitle := "Request Metrics"
	last5Label := "Last 5 Minutes"
	lastHourLabel := "Last Hour"
	last24Label := "Last 24 Hours"
	uptimeLabel := "Uptime"
	requestsLabel := "requests"
	sinceStartLabel := "since start"
	upstreamTitle := "Upstream Cache"
	filteredTitle := "Filtered Cache"
	tableMetric := "Metric"
	tableValue := "Value"
	entriesLabel := "Entries"
	memoryLabel := "Memory"
	hitsLabel := "Hits"
	missesLabel := "Misses"
	hitRatioLabel := "Hit Ratio"
	evictionsLabel := "Evictions"
	defaultTTLLabel := "Default TTL"
	minTTLLabel := "Min TTL"
	maxTTLLabel := "Max TTL"
	backConfigLabel := "← Back to Configuration"
	healthCheckLabel := "Health Check (JSON)"

	if preferSwedish {
		langAttr = "sv"
		pageTitle = "ReCal - Status"
		requestMetricsTitle = "Trafikmätning"
		last5Label = "Senaste 5 minuterna"
		lastHourLabel = "Senaste timmen"
		last24Label = "Senaste 24 timmarna"
		uptimeLabel = "Upptid"
		requestsLabel = "förfrågningar"
		sinceStartLabel = "sedan start"
		upstreamTitle = "Uppströmscache"
		filteredTitle = "Filtrerad cache"
		tableMetric = "Mätvärde"
		tableValue = "Värde"
		entriesLabel = "Poster"
		memoryLabel = "Minne"
		hitsLabel = "Träffar"
		missesLabel = "Missar"
		hitRatioLabel = "Träffkvot"
		evictionsLabel = "Utrensningar"
		defaultTTLLabel = "Standard-TTL"
		minTTLLabel = "Min-TTL"
		maxTTLLabel = "Max-TTL"
		backConfigLabel = "← Tillbaka till konfiguration"
		healthCheckLabel = "Hälsokontroll (JSON)"
	}

	// Get request metrics
	req5m, req1h, req24h := s.requestMetrics.GetStats()

	// Get cache statistics
	upstreamStats := s.upstreamCache.GetStats()
	filteredStats := s.filteredCache.GetStats()

	// Calculate uptime
	uptime := time.Since(s.startTime)

	// Generate HTML
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="%s">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            max-width: 1200px;
            margin: 40px auto;
            padding: 20px;
            background: #f5f5f5;
        }
        h1 { color: #333; }
        h2 { color: #666; margin-top: 30px; }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
            margin: 20px 0;
        }
        .stat-card {
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .stat-label {
            font-size: 14px;
            color: #666;
            margin-bottom: 5px;
        }
        .stat-value {
            font-size: 32px;
            font-weight: bold;
            color: #333;
        }
        .stat-detail {
            font-size: 12px;
            color: #999;
            margin-top: 5px;
        }
        table {
            width: 100%%;
            background: white;
            border-radius: 8px;
            overflow: hidden;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        th, td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #eee;
        }
        th {
            background: #f8f8f8;
            font-weight: 600;
            color: #666;
        }
        .metric-good { color: #28a745; }
        .metric-warning { color: #ffc107; }
        .metric-bad { color: #dc3545; }
        a {
            color: #007bff;
            text-decoration: none;
        }
        a:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
    <h1>%s</h1>

    <h2>%s</h2>
    <div class="stats-grid">
        <div class="stat-card">
            <div class="stat-label">%s</div>
            <div class="stat-value">%d</div>
            <div class="stat-detail">%s</div>
        </div>
        <div class="stat-card">
            <div class="stat-label">%s</div>
            <div class="stat-value">%d</div>
            <div class="stat-detail">%s</div>
        </div>
        <div class="stat-card">
            <div class="stat-label">%s</div>
            <div class="stat-value">%d</div>
            <div class="stat-detail">%s</div>
        </div>
        <div class="stat-card">
            <div class="stat-label">%s</div>
            <div class="stat-value">%s</div>
            <div class="stat-detail">%s</div>
        </div>
    </div>

    <h2>%s</h2>
    <table>
        <tr><th>%s</th><th>%s</th></tr>
        <tr><td>%s</td><td>%d / %d</td></tr>
        <tr><td>%s</td><td>%s / %s</td></tr>
        <tr><td>%s</td><td>%d</td></tr>
        <tr><td>%s</td><td>%d</td></tr>
        <tr><td>%s</td><td class="%s">%.1f%%</td></tr>
        <tr><td>%s</td><td>%d</td></tr>
        <tr><td>%s</td><td>%s</td></tr>
        <tr><td>%s</td><td>%s</td></tr>
        <tr><td>%s</td><td>%s</td></tr>
    </table>

    <h2>%s</h2>
    <table>
        <tr><th>%s</th><th>%s</th></tr>
        <tr><td>%s</td><td>%d / %d</td></tr>
        <tr><td>%s</td><td>%s / %s</td></tr>
        <tr><td>%s</td><td>%d</td></tr>
        <tr><td>%s</td><td>%d</td></tr>
        <tr><td>%s</td><td class="%s">%.1f%%</td></tr>
        <tr><td>%s</td><td>%d</td></tr>
        <tr><td>%s</td><td>%s</td></tr>
        <tr><td>%s</td><td>%s</td></tr>
        <tr><td>%s</td><td>%s</td></tr>
    </table>

    <p style="margin-top: 40px; text-align: center;">
        <a href="/">%s</a> |
        <a href="/health">%s</a>
    </p>
</body>
</html>`,
		langAttr,
		pageTitle,
		pageTitle,
		requestMetricsTitle,
		last5Label, req5m, requestsLabel,
		lastHourLabel, req1h, requestsLabel,
		last24Label, req24h, requestsLabel,
		uptimeLabel, formatDuration(uptime), sinceStartLabel,
		upstreamTitle,
		tableMetric, tableValue,
		entriesLabel, upstreamStats.Entries, upstreamStats.MaxSize,
		memoryLabel, formatBytes(upstreamStats.Memory), formatBytes(upstreamStats.MaxMemory),
		hitsLabel, upstreamStats.Hits,
		missesLabel, upstreamStats.Misses,
		hitRatioLabel, hitRatioClass(upstreamStats.HitRatio), upstreamStats.HitRatio*100,
		evictionsLabel, upstreamStats.Evictions,
		defaultTTLLabel, upstreamStats.DefaultTTL,
		minTTLLabel, upstreamStats.MinTTL,
		maxTTLLabel, upstreamStats.MaxTTL,
		filteredTitle,
		tableMetric, tableValue,
		entriesLabel, filteredStats.Entries, filteredStats.MaxSize,
		memoryLabel, formatBytes(filteredStats.Memory), formatBytes(filteredStats.MaxMemory),
		hitsLabel, filteredStats.Hits,
		missesLabel, filteredStats.Misses,
		hitRatioLabel, hitRatioClass(filteredStats.HitRatio), filteredStats.HitRatio*100,
		evictionsLabel, filteredStats.Evictions,
		defaultTTLLabel, filteredStats.DefaultTTL,
		minTTLLabel, filteredStats.MinTTL,
		maxTTLLabel, filteredStats.MaxTTL,
		backConfigLabel, healthCheckLabel)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}

// formatBytes formats bytes as human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatDuration formats duration as human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

// hitRatioClass returns CSS class based on hit ratio
func hitRatioClass(ratio float64) string {
	if ratio >= 0.8 {
		return "metric-good"
	}
	if ratio >= 0.5 {
		return "metric-warning"
	}
	return "metric-bad"
}

// fetchUpstream fetches the upstream feed, using cache if available
func (s *Server) fetchUpstream(ctx context.Context, upstreamURL string) ([]byte, time.Duration, error) {
	// Check upstream cache
	if entry, found := s.upstreamCache.Get(upstreamURL); found {
		// Try conditional request
		resp, notModified, err := s.fetcher.FetchConditional(ctx, upstreamURL, entry.ETag, entry.LastModified)
		if err != nil {
			return nil, 0, err
		}

		if notModified {
			// Use cached data
			return entry.Data, time.Until(entry.Expiry), nil
		}

		// Content modified, use new data
		ttl := fetcher.ParseCacheHeaders(resp.CacheControl, resp.Expires)
		if ttl == 0 {
			ttl = s.cfg.Cache.DefaultTTL
		}

		s.upstreamCache.Set(upstreamURL, resp.Body, ttl, resp.ETag, resp.LastModified)
		return resp.Body, ttl, nil
	}

	// No cache entry, fetch fresh
	resp, err := s.fetcher.Fetch(ctx, upstreamURL)
	if err != nil {
		return nil, 0, err
	}

	ttl := fetcher.ParseCacheHeaders(resp.CacheControl, resp.Expires)
	if ttl == 0 {
		ttl = s.cfg.Cache.DefaultTTL
	}

	s.upstreamCache.Set(upstreamURL, resp.Body, ttl, resp.ETag, resp.LastModified)
	return resp.Body, ttl, nil
}

// serveFromCache serves a response from cache
func (s *Server) serveFromCache(w http.ResponseWriter, entry *cache.Entry, debug bool) {
	contentType := "text/calendar; charset=utf-8"
	if debug {
		contentType = "text/html; charset=utf-8"
	}

	cacheDuration := time.Until(entry.Expiry)
	if cacheDuration < 0 {
		cacheDuration = 0
	}

	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(cacheDuration.Seconds())))
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Cache", "HIT")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(entry.Data)
}

// Params represents parsed URL parameters
type Params struct {
	Upstream       string
	Filters        []FilterParam
	SpecialFilters SpecialFilters
	Debug          bool
}

// FilterParam represents a single filter (field + pattern)
type FilterParam struct {
	Fields  []string
	Pattern string
}

// SpecialFilters represents special filter parameters
type SpecialFilters struct {
	Grad              string
	Loge              string
	RemoveUnconfirmed bool
	RemoveInstallt    bool
}

// parseParams parses URL query parameters
func parseParams(r *http.Request) (*Params, error) {
	q := r.URL.Query()

	params := &Params{
		Upstream: q.Get("upstream"),
		Debug:    q.Get("debug") == "true" || q.Get("debug") == "1",
	}

	// If no upstream specified, use default from config
	// We'll need to pass config here, but for now leave empty to be filled by caller

	// Parse basic filters (field + pattern, field1 + pattern1, etc.)
	// First check for non-indexed filter
	if pattern := q.Get("pattern"); pattern != "" {
		fieldStr := q.Get("field")
		if fieldStr == "" {
			fieldStr = "SUMMARY,DESCRIPTION" // Default fields
		}
		params.Filters = append(params.Filters, FilterParam{
			Fields:  parseFieldList(fieldStr),
			Pattern: pattern,
		})
	}

	// Check for indexed filters (field1/pattern1, field2/pattern2, etc.)
	for i := 1; i <= 20; i++ { // Support up to 20 indexed filters
		fieldKey := fmt.Sprintf("field%d", i)
		patternKey := fmt.Sprintf("pattern%d", i)

		pattern := q.Get(patternKey)
		if pattern == "" {
			continue
		}

		fieldStr := q.Get(fieldKey)
		if fieldStr == "" {
			fieldStr = "SUMMARY,DESCRIPTION"
		}

		params.Filters = append(params.Filters, FilterParam{
			Fields:  parseFieldList(fieldStr),
			Pattern: pattern,
		})
	}

	// Parse special filters
	params.SpecialFilters.Grad = q.Get("Grad")
	params.SpecialFilters.Loge = q.Get("Loge")

	// Boolean parameters: presence means true, or explicit value
	// Support: ?RemoveUnconfirmed or ?RemoveUnconfirmed=true or ?RemoveUnconfirmed=1
	params.SpecialFilters.RemoveUnconfirmed = parseBoolParam(q, "RemoveUnconfirmed")
	params.SpecialFilters.RemoveInstallt = parseBoolParam(q, "RemoveInstallt")

	return params, nil
}

// parseBoolParam checks if a boolean parameter is present or set to true
// Returns true if: parameter exists without value, or value is "true" or "1"
func parseBoolParam(q map[string][]string, key string) bool {
	values, exists := q[key]
	if !exists {
		return false
	}
	// If parameter exists but has no value, or is empty string, treat as true
	if len(values) == 0 || values[0] == "" {
		return true
	}
	// Check explicit true values
	val := values[0]
	return val == "true" || val == "1"
}

// parseFieldList parses a comma-separated list of field names
func parseFieldList(fieldStr string) []string {
	var fields []string
	for _, f := range splitByComma(fieldStr) {
		f = trimSpace(f)
		if f != "" {
			fields = append(fields, f)
		}
	}
	return fields
}

// splitByComma splits a string by commas
func splitByComma(s string) []string {
	if s == "" {
		return []string{""}
	}

	var result []string
	current := ""
	for _, ch := range s {
		if ch == ',' {
			result = append(result, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	result = append(result, current)
	return result
}

// trimSpace trims leading and trailing whitespace
func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && isSpace(s[start]) {
		start++
	}

	for end > start && isSpace(s[end-1]) {
		end--
	}

	return s[start:end]
}

// isSpace checks if a byte is a space character
func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// createCacheKey creates a cache key from parameters
func createCacheKey(params *Params) string {
	components := []string{params.Upstream}

	// Add filters
	for _, f := range params.Filters {
		components = append(components, f.Fields...)
		components = append(components, f.Pattern)
	}

	// Add special filters
	if params.SpecialFilters.Grad != "" {
		components = append(components, "Grad:"+params.SpecialFilters.Grad)
	}
	if params.SpecialFilters.Loge != "" {
		components = append(components, "Loge:"+params.SpecialFilters.Loge)
	}
	if params.SpecialFilters.RemoveUnconfirmed {
		components = append(components, "RemoveUnconfirmed:true")
	}
	if params.SpecialFilters.RemoveInstallt {
		components = append(components, "RemoveInstallt:true")
	}

	// Add debug flag
	if params.Debug {
		components = append(components, "debug:true")
	}

	return cache.HashKey(components...)
}

// buildFilters builds filter engine from parameters
func (s *Server) buildFilters(engine *filter.Engine, params *Params) error {
	// Add basic filters
	for _, f := range params.Filters {
		if err := engine.AddFilter(f.Fields, f.Pattern); err != nil {
			return fmt.Errorf("filter error: %w", err)
		}
	}

	// Add special filters
	if params.SpecialFilters.Grad != "" {
		if err := engine.AddGradeFilter(params.SpecialFilters.Grad); err != nil {
			return fmt.Errorf("grad filter error: %w", err)
		}
	}

	if params.SpecialFilters.Loge != "" {
		if err := engine.AddLodgeFilter(params.SpecialFilters.Loge); err != nil {
			return fmt.Errorf("loge filter error: %w", err)
		}
	}

	if params.SpecialFilters.RemoveUnconfirmed {
		if err := engine.AddConfirmedOnlyFilter(); err != nil {
			return fmt.Errorf("remove unconfirmed filter error: %w", err)
		}
	}

	if params.SpecialFilters.RemoveInstallt {
		if err := engine.AddInstalltFilter(); err != nil {
			return fmt.Errorf("remove installt filter error: %w", err)
		}
	}

	return nil
}

// generateDebugHTML generates debug mode HTML output
// backURL is the URL to go back to (e.g., "/" for main page, "/feed/{slug}/edit" for feed edit page)
// preferSwedish indicates whether to use Swedish text based on Accept-Language header
func (s *Server) generateDebugHTML(original, filtered *parser.Calendar, matches []filter.MatchResult, engine *filter.Engine, backURL string, preferSwedish bool) string {
	stats := filter.GetStats(original, filtered)

	// Determine back button text based on language preference
	backText := "← Back to configuration"
	if preferSwedish {
		backText = "← Tillbaka till konfiguration"
	}

	pageTitle := "ReCal Debug"
	reportTitle := "ReCal Debug Report"
	summaryTitle := "Summary Statistics"
	totalEventsLabel := "Total events in upstream:"
	filteredEventsLabel := "Events in filtered output:"
	removedEventsLabel := "Events removed:"
	activeFiltersTitle := "Active Filters"
	noFiltersText := "No filters applied"
	filterLabel := "Filter"
	fieldsLabel := "Fields:"
	invertSuffix := " (inverted - keeps matching)"
	removedEventsTitle := "Removed Events"
	noRemovedEventsText := "No events were removed"
	eventLabel := "Event:"
	uidLabel := "UID:"
	matchedFiltersLabel := "Matched filters:"
	fieldLabel := "Field"
	matchedFilterLabel := "matched filter"
	sampleEventsTitle := "Sample Filtered Events"
	noFilteredEventsText := "No events in filtered output"
	moreEventsPrefix := "... and "
	moreEventsSuffix := " more events"

	if preferSwedish {
		pageTitle = "ReCal Debugg"
		reportTitle = "ReCal Debugg-rapport"
		summaryTitle = "Sammanfattning"
		totalEventsLabel = "Totalt antal händelser i källflödet:"
		filteredEventsLabel = "Händelser i filtrerat resultat:"
		removedEventsLabel = "Borttagna händelser:"
		activeFiltersTitle = "Aktiva filter"
		noFiltersText = "Inga filter används"
		filterLabel = "Filter"
		fieldsLabel = "Fält:"
		invertSuffix = " (inverterat - behåller matchningar)"
		removedEventsTitle = "Borttagna händelser"
		noRemovedEventsText = "Inga händelser togs bort"
		eventLabel = "Händelse:"
		uidLabel = "UID:"
		matchedFiltersLabel = "Matchade filter:"
		fieldLabel = "Fält"
		matchedFilterLabel = "matchade filter"
		sampleEventsTitle = "Exempel på filtrerade händelser"
		noFilteredEventsText = "Inga händelser i filtrerat resultat"
		moreEventsPrefix = "... och "
		moreEventsSuffix = " fler händelser"
	}

	// Determine html lang attribute
	langAttr := "en"
	if preferSwedish {
		langAttr = "sv"
	}

	html := `<!DOCTYPE html>
<html lang="` + langAttr + `">
<head>
	<meta charset="UTF-8">
	<title>` + pageTitle + `</title>
	<style>
		body { font-family: Arial, sans-serif; margin: 20px; max-width: 1200px; margin: 20px auto; }
		h1 { color: #333; }
		h2 { color: #666; margin-top: 30px; }
		.back-link { display: inline-block; margin-bottom: 20px; padding: 10px 20px; background: #0066cc; color: white; text-decoration: none; border-radius: 4px; }
		.back-link:hover { background: #0052a3; }
		.stats { background: #f0f0f0; padding: 15px; border-radius: 5px; }
		.stats p { margin: 5px 0; }
		.filter { background: #e8f4f8; padding: 10px; margin: 5px 0; border-left: 3px solid #0066cc; }
		.match { background: #fff3cd; padding: 10px; margin: 10px 0; border-left: 3px solid #ffc107; }
		.event { background: #d4edda; padding: 10px; margin: 10px 0; border-left: 3px solid #28a745; }
		code { background: #f5f5f5; padding: 2px 5px; border-radius: 3px; }
	</style>
</head>
<body>
	<a href="` + htmlutil.EscapeString(backURL) + `" class="back-link">` + backText + `</a>
	<h1>` + reportTitle + `</h1>

	<div class="stats">
		<h2>` + summaryTitle + `</h2>
		<p><strong>` + totalEventsLabel + `</strong> ` + strconv.Itoa(stats.TotalEvents) + `</p>
		<p><strong>` + filteredEventsLabel + `</strong> ` + strconv.Itoa(stats.FilteredEvents) + `</p>
		<p><strong>` + removedEventsLabel + `</strong> ` + strconv.Itoa(stats.RemovedEvents) + `</p>
	</div>

	<h2>` + activeFiltersTitle + `</h2>`

	filters := engine.GetFilters()
	if len(filters) == 0 {
		html += `<p>` + noFiltersText + `</p>`
	} else {
		for i, f := range filters {
			invertStr := ""
			if f.Invert {
				invertStr = invertSuffix
			}
			html += fmt.Sprintf(`<div class="filter"><strong>%s %d:</strong> %s<br><strong>%s</strong> %v%s</div>`,
				filterLabel, i+1, htmlutil.EscapeString(f.Raw), fieldsLabel, f.Fields, invertStr)
		}
	}

	html += `<h2>` + removedEventsTitle + `</h2>`

	if len(matches) == 0 {
		html += `<p>` + noRemovedEventsText + `</p>`
	} else {
		// Group matches by event UID
		matchesByUID := make(map[string][]filter.MatchResult)
		for _, m := range matches {
			matchesByUID[m.EventUID] = append(matchesByUID[m.EventUID], m)
		}

		for uid, eventMatches := range matchesByUID {
			html += `<div class="match">`
			html += `<p><strong>` + eventLabel + `</strong> ` + htmlutil.EscapeString(eventMatches[0].EventSummary) + `</p>`
			html += `<p><strong>` + uidLabel + `</strong> <code>` + htmlutil.EscapeString(uid) + `</code></p>`
			html += `<p><strong>` + matchedFiltersLabel + `</strong></p><ul>`
			for _, m := range eventMatches {
				html += `<li>` + fieldLabel + ` <code>` + htmlutil.EscapeString(m.Field) + `</code> ` + matchedFilterLabel + ` <code>` + htmlutil.EscapeString(m.FilterRaw) + `</code></li>`
			}
			html += `</ul></div>`
		}
	}

	html += `<h2>` + sampleEventsTitle + `</h2>`

	if len(filtered.Events) == 0 {
		html += `<p>` + noFilteredEventsText + `</p>`
	} else {
		limit := 5
		if len(filtered.Events) < limit {
			limit = len(filtered.Events)
		}

		for i := 0; i < limit; i++ {
			event := filtered.Events[i]
			html += `<div class="event">`
			html += `<p><strong>` + htmlutil.EscapeString(event.Summary) + `</strong></p>`
			if event.Description != "" {
				desc := event.Description
				if len(desc) > 100 {
					desc = desc[:100] + "..."
				}
				html += `<p>` + htmlutil.EscapeString(desc) + `</p>`
			}
			html += `<p><code>` + htmlutil.EscapeString(event.DTStart) + ` - ` + htmlutil.EscapeString(event.DTEnd) + `</code></p>`
			html += `</div>`
		}

		if len(filtered.Events) > limit {
			html += `<p>` + moreEventsPrefix + strconv.Itoa(len(filtered.Events)-limit) + moreEventsSuffix + `</p>`
		}
	}

	html += `</body>
</html>`

	return html
}

// ConfigPage serves the web UI configuration page
func (s *Server) ConfigPage(w http.ResponseWriter, r *http.Request) {
	// Record request metrics
	s.requestMetrics.RecordRequest()

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	preferSwedish := prefersSwedish(r)
	templateSource := configPageTemplateEN
	if preferSwedish {
		templateSource = configPageTemplateSV
	}

	// Parse template with base URL
	tmpl, err := template.New("config").Parse(templateSource)
	if err != nil {
		http.Error(w, "Failed to parse template", http.StatusInternalServerError)
		log.Printf("Template parse error: %v", err)
		return
	}

	data := struct {
		BaseURL string
	}{
		BaseURL: s.cfg.Server.BaseURL,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Printf("Template execute error: %v", err)
		return
	}
}

// GetLodges returns a JSON list of unique lodge names from the upstream feed
func (s *Server) GetLodges(w http.ResponseWriter, r *http.Request) {
	// Record request metrics
	s.requestMetrics.RecordRequest()

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return canonical lodge list from config (already sorted in config)
	lodges := make([]string, len(s.cfg.Filters.Lodge.Names))
	copy(lodges, s.cfg.Filters.Lodge.Names)

	// Sort with Swedish collation
	sortSwedish(lodges)

	// Return JSON
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=900") // Cache for 15 minutes
	_ = json.NewEncoder(w).Encode(map[string][]string{"lodges": lodges})
}

// sortSwedish sorts strings using Swedish alphabetical order (å, ä, ö after z)
func sortSwedish(strings []string) {
	collator := collate.New(language.Swedish)
	sort.Slice(strings, func(i, j int) bool {
		return collator.CompareString(strings[i], strings[j]) < 0
	})
}

// routeSlugEndpoints routes /feed/* endpoints
func (s *Server) routeSlugEndpoints(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if strings.HasSuffix(path, "/edit") {
		s.SlugManage(w, r)
	} else if strings.HasSuffix(path, "/debug") {
		s.SlugDebug(w, r)
	} else if strings.HasSuffix(path, "/preview") {
		s.SlugPreview(w, r)
	} else {
		// Default: serve the feed
		s.SlugFeed(w, r)
	}
}

// routeAdminFeeds routes /admin/feeds endpoint
func (s *Server) routeAdminFeeds(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.AdminCreateFeed(w, r)
	case http.MethodGet:
		s.AdminListFeeds(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// routeAdminFeedsBySlug routes /admin/feeds/{slug} endpoints
func (s *Server) routeAdminFeedsBySlug(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if strings.HasSuffix(path, "/stats") {
		s.AdminGetFeedStats(w, r)
	} else {
		switch r.Method {
		case http.MethodGet:
			s.AdminGetFeed(w, r)
		case http.MethodPut:
			s.AdminUpdateFeed(w, r)
		case http.MethodDelete:
			s.AdminDeleteFeed(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// loggingMiddleware logs all incoming requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s %s (Host: %s, X-Forwarded-For: %s)",
			r.Method, r.URL.Path, r.URL.RawQuery, r.Host, r.Header.Get("X-Forwarded-For"))
		next.ServeHTTP(w, r)
	})
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.Version)  // Root shows version info
	mux.HandleFunc("/query", s.ServeHTTP)
	mux.HandleFunc("/query/debug", s.DebugHTTP)
	mux.HandleFunc("/status", s.Status)
	mux.HandleFunc("/api/lodges", s.GetLodges)
	mux.HandleFunc("/health", s.Health)
	mux.HandleFunc("/version", s.Version)

	// Admin dashboard (web UI)
	mux.HandleFunc("/admin", s.AdminPage)

	// Named feed endpoints (public)
	mux.HandleFunc("/feed/", s.routeSlugEndpoints)

	// Admin feed endpoints (protected by upstream auth)
	mux.HandleFunc("/admin/feeds", s.routeAdminFeeds)
	mux.HandleFunc("/admin/feeds/", s.routeAdminFeedsBySlug)

	addr := fmt.Sprintf(":%d", s.cfg.Server.Port)
	log.Printf("Starting server on %s", addr)
	log.Printf("Endpoints: / /query /query/debug /admin /status /api/lodges /health")
	log.Printf("Admin dashboard available at: %s/admin", s.cfg.Server.BaseURL)

	// Wrap with logging middleware
	loggingHandler := s.loggingMiddleware(mux)

	server := &http.Server{
		Addr:         addr,
		Handler:      loggingHandler,
		ReadTimeout:  s.cfg.Server.ReadTimeout,
		WriteTimeout: s.cfg.Server.WriteTimeout,
		IdleTimeout:  s.cfg.Server.IdleTimeout,
	}

	return server.ListenAndServe()
}
