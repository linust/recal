package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/linus/recal/internal/config"
)

// Response represents an HTTP response with caching metadata
type Response struct {
	Body         []byte
	StatusCode   int
	ETag         string
	LastModified string
	CacheControl string
	Expires      string
}

// Fetcher fetches upstream iCal feeds with HTTP cache support
type Fetcher struct {
	client            *http.Client
	cfg               *config.Config
	disableSSRFChecks bool // For testing only
}

// NewFetcher creates a new fetcher
func NewFetcher(cfg *config.Config) *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: cfg.Upstream.Timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Limit redirects to prevent redirect loops
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		cfg:               cfg,
		disableSSRFChecks: false,
	}
}

// NewTestFetcher creates a fetcher with SSRF checks disabled (for testing only)
func NewTestFetcher(cfg *config.Config) *Fetcher {
	f := NewFetcher(cfg)
	f.disableSSRFChecks = true
	return f
}

// Fetch fetches a URL and returns the response
func (f *Fetcher) Fetch(ctx context.Context, urlStr string) (*Response, error) {
	// Validate URL
	if err := f.validateURL(urlStr); err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent
	req.Header.Set("User-Agent", "iCal-Filter/1.0")

	// Execute request
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return &Response{
		Body:         body,
		StatusCode:   resp.StatusCode,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		CacheControl: resp.Header.Get("Cache-Control"),
		Expires:      resp.Header.Get("Expires"),
	}, nil
}

// FetchConditional fetches with conditional request headers (ETag/Last-Modified)
// Returns (response, notModified, error)
func (f *Fetcher) FetchConditional(ctx context.Context, urlStr string, etag string, lastModified string) (*Response, bool, error) {
	// Validate URL
	if err := f.validateURL(urlStr); err != nil {
		return nil, false, fmt.Errorf("invalid URL: %w", err)
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent
	req.Header.Set("User-Agent", "iCal-Filter/1.0")

	// Set conditional headers
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}

	// Execute request
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check for 304 Not Modified
	if resp.StatusCode == http.StatusNotModified {
		return nil, true, nil
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read response body: %w", err)
	}

	return &Response{
		Body:         body,
		StatusCode:   resp.StatusCode,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		CacheControl: resp.Header.Get("Cache-Control"),
		Expires:      resp.Header.Get("Expires"),
	}, false, nil
}

// validateURL validates and sanitizes a URL
func (f *Fetcher) validateURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Must be HTTP or HTTPS
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL must use HTTP or HTTPS scheme, got %q", parsedURL.Scheme)
	}

	// Skip SSRF checks if disabled (for testing)
	if f.disableSSRFChecks {
		return nil
	}

	// Check for SSRF: block private IP ranges
	// This is a basic check; for production, use a more comprehensive library
	host := parsedURL.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return fmt.Errorf("cannot access localhost")
	}

	// Block common private IP ranges (basic check)
	// In production, use a proper IP parsing library and check all RFC 1918 ranges
	if len(host) > 0 {
		// Check for 10.x.x.x, 192.168.x.x, 172.16-31.x.x
		if len(host) >= 3 && host[:3] == "10." {
			return fmt.Errorf("cannot access private IP addresses")
		}
		if len(host) >= 8 && host[:8] == "192.168." {
			return fmt.Errorf("cannot access private IP addresses")
		}
		if len(host) >= 7 && host[:4] == "172." {
			// Basic check for 172.16.0.0 - 172.31.255.255
			// This is simplified; proper implementation should parse the IP
			if len(host) >= 6 && host[4:6] >= "16" && host[4:6] <= "31" {
				return fmt.Errorf("cannot access private IP addresses")
			}
		}
	}

	return nil
}

// ParseCacheHeaders extracts TTL from cache headers
// Returns TTL duration, or 0 if no caching directives found
func ParseCacheHeaders(cacheControl string, expires string) time.Duration {
	// Try Cache-Control first (preferred)
	if cacheControl != "" {
		// Look for max-age directive
		// This is a simple parser; a production implementation should be more robust
		if len(cacheControl) > 8 && cacheControl[:8] == "max-age=" {
			var seconds int
			_, err := fmt.Sscanf(cacheControl[8:], "%d", &seconds)
			if err == nil && seconds > 0 {
				return time.Duration(seconds) * time.Second
			}
		}
		// Check for s-maxage (takes precedence for shared caches)
		if len(cacheControl) > 10 {
			var seconds int
			_, err := fmt.Sscanf(cacheControl, "s-maxage=%d", &seconds)
			if err == nil && seconds > 0 {
				return time.Duration(seconds) * time.Second
			}
		}
	}

	// Try Expires header
	if expires != "" {
		expiresTime, err := http.ParseTime(expires)
		if err == nil {
			ttl := time.Until(expiresTime)
			if ttl > 0 {
				return ttl
			}
		}
	}

	return 0
}
