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
	"github.com/linus/recal/internal/filter"
	"github.com/linus/recal/internal/metrics"
	"github.com/linus/recal/internal/parser"
)

// Server is the HTTP server for the ReCal application
type Server struct {
	cfg            *config.Config
	upstreamCache  *cache.Cache
	filteredCache  *cache.Cache
	fetcher        *fetcher.Fetcher
	requestMetrics *metrics.RequestMetrics
	startTime      time.Time
}

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

	// Create cache key for filtered result
	cacheKey := createCacheKey(params)

	// Check filtered cache first
	if entry, found := s.filteredCache.Get(cacheKey); found {
		s.serveFromCache(w, entry, false)
		return
	}

	// Fetch upstream feed
	upstreamData, upstreamTTL, err := s.fetchUpstream(r.Context(), params.Upstream)
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

	filteredCal, _ := engine.Apply(cal)

	// Serialize iCal
	var buf bytes.Buffer
	if err := filteredCal.Serialize(&buf); err != nil {
		http.Error(w, fmt.Sprintf("Failed to serialize iCal: %v", err), http.StatusInternalServerError)
		return
	}
	output := buf.Bytes()

	// Cache the result
	s.filteredCache.Set(cacheKey, output, upstreamTTL, "", "")

	// Set cache headers for client
	cacheDuration := upstreamTTL
	if cacheDuration < s.cfg.Cache.MinOutputCache {
		cacheDuration = s.cfg.Cache.MinOutputCache
	}
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(cacheDuration.Seconds())))
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(output)
}

// DebugHTTP handles HTTP requests for debug mode (HTML output)
func (s *Server) DebugHTTP(w http.ResponseWriter, r *http.Request) {
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
	if params.Upstream == "" && len(params.Filters) == 0 &&
		params.SpecialFilters.Grad == "" && params.SpecialFilters.Loge == "" &&
		!params.SpecialFilters.RemoveUnconfirmed && !params.SpecialFilters.RemoveInstallt {
		http.Error(w, "No filters specified. Use /debug?pattern=... or other filter parameters.", http.StatusBadRequest)
		return
	}

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

	// Generate debug HTML
	output := s.generateDebugHTML(originalCal, filteredCal, matches, engine)

	// No caching for debug mode
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(output))
}

// DebugRedirect redirects /debug to /filter/preview for backward compatibility
func (s *Server) DebugRedirect(w http.ResponseWriter, r *http.Request) {
	// Build new URL with same query parameters
	newURL := "/filter/preview"
	if r.URL.RawQuery != "" {
		newURL += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, newURL, http.StatusMovedPermanently)
}

// Health handles health check requests
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	stats := s.upstreamCache.GetStats()
	filteredStats := s.filteredCache.GetStats()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","upstream_cache":%d,"filtered_cache":%d}`,
		stats.Entries, filteredStats.Entries)
}

// Status handles status page requests with metrics and cache statistics
func (s *Server) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
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
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ReCal - Status</title>
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
    <h1>ReCal - Status</h1>

    <h2>Request Metrics</h2>
    <div class="stats-grid">
        <div class="stat-card">
            <div class="stat-label">Last 5 Minutes</div>
            <div class="stat-value">%d</div>
            <div class="stat-detail">requests</div>
        </div>
        <div class="stat-card">
            <div class="stat-label">Last Hour</div>
            <div class="stat-value">%d</div>
            <div class="stat-detail">requests</div>
        </div>
        <div class="stat-card">
            <div class="stat-label">Last 24 Hours</div>
            <div class="stat-value">%d</div>
            <div class="stat-detail">requests</div>
        </div>
        <div class="stat-card">
            <div class="stat-label">Uptime</div>
            <div class="stat-value">%s</div>
            <div class="stat-detail">since start</div>
        </div>
    </div>

    <h2>Upstream Cache</h2>
    <table>
        <tr><th>Metric</th><th>Value</th></tr>
        <tr><td>Entries</td><td>%d / %d</td></tr>
        <tr><td>Memory</td><td>%s / %s</td></tr>
        <tr><td>Hits</td><td>%d</td></tr>
        <tr><td>Misses</td><td>%d</td></tr>
        <tr><td>Hit Ratio</td><td class="%s">%.1f%%</td></tr>
        <tr><td>Evictions</td><td>%d</td></tr>
        <tr><td>Default TTL</td><td>%s</td></tr>
        <tr><td>Min TTL</td><td>%s</td></tr>
        <tr><td>Max TTL</td><td>%s</td></tr>
    </table>

    <h2>Filtered Cache</h2>
    <table>
        <tr><th>Metric</th><th>Value</th></tr>
        <tr><td>Entries</td><td>%d / %d</td></tr>
        <tr><td>Memory</td><td>%s / %s</td></tr>
        <tr><td>Hits</td><td>%d</td></tr>
        <tr><td>Misses</td><td>%d</td></tr>
        <tr><td>Hit Ratio</td><td class="%s">%.1f%%</td></tr>
        <tr><td>Evictions</td><td>%d</td></tr>
        <tr><td>Default TTL</td><td>%s</td></tr>
        <tr><td>Min TTL</td><td>%s</td></tr>
        <tr><td>Max TTL</td><td>%s</td></tr>
    </table>

    <p style="margin-top: 40px; text-align: center;">
        <a href="/">← Back to Configuration</a> |
        <a href="/health">Health Check (JSON)</a>
    </p>
</body>
</html>`,
		req5m, req1h, req24h,
		formatDuration(uptime),
		upstreamStats.Entries, upstreamStats.MaxSize,
		formatBytes(upstreamStats.Memory), formatBytes(upstreamStats.MaxMemory),
		upstreamStats.Hits, upstreamStats.Misses,
		hitRatioClass(upstreamStats.HitRatio), upstreamStats.HitRatio*100,
		upstreamStats.Evictions,
		upstreamStats.DefaultTTL, upstreamStats.MinTTL, upstreamStats.MaxTTL,
		filteredStats.Entries, filteredStats.MaxSize,
		formatBytes(filteredStats.Memory), formatBytes(filteredStats.MaxMemory),
		filteredStats.Hits, filteredStats.Misses,
		hitRatioClass(filteredStats.HitRatio), filteredStats.HitRatio*100,
		filteredStats.Evictions,
		filteredStats.DefaultTTL, filteredStats.MinTTL, filteredStats.MaxTTL)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
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
	w.Write(entry.Data)
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
		if err := engine.AddGradFilter(params.SpecialFilters.Grad); err != nil {
			return fmt.Errorf("grad filter error: %w", err)
		}
	}

	if params.SpecialFilters.Loge != "" {
		if err := engine.AddLogeFilter(params.SpecialFilters.Loge); err != nil {
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
func (s *Server) generateDebugHTML(original, filtered *parser.Calendar, matches []filter.MatchResult, engine *filter.Engine) string {
	stats := filter.GetStats(original, filtered)

	html := `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>ReCal Debug</title>
	<style>
		body { font-family: Arial, sans-serif; margin: 20px; }
		h1 { color: #333; }
		h2 { color: #666; margin-top: 30px; }
		.stats { background: #f0f0f0; padding: 15px; border-radius: 5px; }
		.stats p { margin: 5px 0; }
		.filter { background: #e8f4f8; padding: 10px; margin: 5px 0; border-left: 3px solid #0066cc; }
		.match { background: #fff3cd; padding: 10px; margin: 10px 0; border-left: 3px solid #ffc107; }
		.event { background: #d4edda; padding: 10px; margin: 10px 0; border-left: 3px solid #28a745; }
		code { background: #f5f5f5; padding: 2px 5px; border-radius: 3px; }
	</style>
</head>
<body>
	<h1>ReCal Debug Report</h1>

	<div class="stats">
		<h2>Summary Statistics</h2>
		<p><strong>Total events in upstream:</strong> ` + strconv.Itoa(stats.TotalEvents) + `</p>
		<p><strong>Events in filtered output:</strong> ` + strconv.Itoa(stats.FilteredEvents) + `</p>
		<p><strong>Events removed:</strong> ` + strconv.Itoa(stats.RemovedEvents) + `</p>
	</div>

	<h2>Active Filters</h2>`

	filters := engine.GetFilters()
	if len(filters) == 0 {
		html += `<p>No filters applied</p>`
	} else {
		for i, f := range filters {
			invertStr := ""
			if f.Invert {
				invertStr = " (inverted - keeps matching)"
			}
			html += fmt.Sprintf(`<div class="filter"><strong>Filter %d:</strong> %s<br><strong>Fields:</strong> %v%s</div>`,
				i+1, htmlutil.EscapeString(f.Raw), f.Fields, invertStr)
		}
	}

	html += `<h2>Removed Events</h2>`

	if len(matches) == 0 {
		html += `<p>No events were removed</p>`
	} else {
		// Group matches by event UID
		matchesByUID := make(map[string][]filter.MatchResult)
		for _, m := range matches {
			matchesByUID[m.EventUID] = append(matchesByUID[m.EventUID], m)
		}

		for uid, eventMatches := range matchesByUID {
			html += `<div class="match">`
			html += `<p><strong>Event:</strong> ` + htmlutil.EscapeString(eventMatches[0].EventSummary) + `</p>`
			html += `<p><strong>UID:</strong> <code>` + htmlutil.EscapeString(uid) + `</code></p>`
			html += `<p><strong>Matched filters:</strong></p><ul>`
			for _, m := range eventMatches {
				html += `<li>Field <code>` + htmlutil.EscapeString(m.Field) + `</code> matched filter <code>` + htmlutil.EscapeString(m.FilterRaw) + `</code></li>`
			}
			html += `</ul></div>`
		}
	}

	html += `<h2>Sample Filtered Events</h2>`

	if len(filtered.Events) == 0 {
		html += `<p>No events in filtered output</p>`
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
			html += `<p>... and ` + strconv.Itoa(len(filtered.Events)-limit) + ` more events</p>`
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

	// Parse template with base URL
	tmpl, err := template.New("config").Parse(configPageTemplate)
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

	// Fetch and parse upstream feed
	ctx := r.Context()
	upstreamData, _, err := s.fetchUpstream(ctx, s.cfg.Upstream.DefaultURL)
	if err != nil {
		http.Error(w, "Failed to fetch upstream", http.StatusBadGateway)
		log.Printf("Failed to fetch upstream for lodges: %v", err)
		return
	}

	cal, err := parser.Parse(bytes.NewReader(upstreamData))
	if err != nil {
		http.Error(w, "Failed to parse calendar", http.StatusInternalServerError)
		log.Printf("Failed to parse calendar for lodges: %v", err)
		return
	}

	// Extract unique lodge names
	lodgeMap := make(map[string]bool)
	for _, event := range cal.Events {
		// Pattern: "{LodgeName} PB:" or special cases
		if strings.Contains(event.Summary, " PB:") {
			parts := strings.Split(event.Summary, " PB:")
			if len(parts) > 0 {
				lodge := strings.TrimSpace(parts[0])
				// Remove any prefix before the lodge name (e.g., "Grad 4, ")
				if idx := strings.LastIndex(lodge, ", "); idx != -1 {
					lodge = strings.TrimSpace(lodge[idx+2:])
				}
				// Remove "INSTÄLLT: " prefix if present
				lodge = strings.TrimPrefix(lodge, "INSTÄLLT: ")
				if lodge != "" {
					lodgeMap[lodge] = true
				}
			}
		}
		// Special case: Moderlogen
		if strings.Contains(event.Summary, "PB, Moderlogen:") {
			lodgeMap["Moderlogen"] = true
		}
	}

	// Convert to array and sort with Swedish collation
	lodges := make([]string, 0, len(lodgeMap))
	for lodge := range lodgeMap {
		lodges = append(lodges, lodge)
	}
	sortSwedish(lodges)

	// Return JSON
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=900") // Cache for 15 minutes
	_ = json.NewEncoder(w).Encode(map[string][]string{"lodges": lodges})
}

// sortSwedish sorts strings using Swedish alphabetical order (å, ä, ö after z)
func sortSwedish(strings []string) {
	sort.Slice(strings, func(i, j int) bool {
		return compareSwedish(strings[i], strings[j]) < 0
	})
}

// compareSwedish compares two strings using Swedish collation rules
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func compareSwedish(a, b string) int {
	// Swedish alphabet order: a-z, å, ä, ö
	// Convert to lowercase for comparison
	a = strings.ToLower(a)
	b = strings.ToLower(b)

	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		aVal := getSwedishValue(rune(a[i]))
		bVal := getSwedishValue(rune(b[i]))
		if aVal != bVal {
			if aVal < bVal {
				return -1
			}
			return 1
		}
	}

	// If all compared chars are equal, shorter string comes first
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

// getSwedishValue returns a sort value for Swedish characters
// Regular a-z get their ASCII values, å/ä/ö come after z
func getSwedishValue(r rune) int {
	switch r {
	case 'å':
		return 'z' + 1
	case 'ä':
		return 'z' + 2
	case 'ö':
		return 'z' + 3
	default:
		return int(r)
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.ConfigPage)
	mux.HandleFunc("/filter", s.ServeHTTP)
	mux.HandleFunc("/filter/preview", s.DebugHTTP)
	mux.HandleFunc("/debug", s.DebugRedirect)
	mux.HandleFunc("/status", s.Status)
	mux.HandleFunc("/api/lodges", s.GetLodges)
	mux.HandleFunc("/health", s.Health)

	addr := fmt.Sprintf(":%d", s.cfg.Server.Port)
	log.Printf("Starting server on %s", addr)
	log.Printf("Endpoints: / /filter /filter/preview /debug (redirect) /status /api/lodges /health")

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  s.cfg.Server.ReadTimeout,
		WriteTimeout: s.cfg.Server.WriteTimeout,
		IdleTimeout:  s.cfg.Server.IdleTimeout,
	}

	return server.ListenAndServe()
}
