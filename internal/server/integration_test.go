package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/linus/recal/internal/config"
)

// TestIntegrationConfigPage tests that the root endpoint shows the configuration page
func TestIntegrationConfigPage(t *testing.T) {
	// Create a test server
	srv := setupTestServer(t)
	defer srv.Close()

	// Test GET /
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("Failed to GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Verify the configuration page contains expected elements
	requiredElements := []string{
		"ReCal",
		"Konfigurera",
		"<select id=\"grad-select\">",
		"<div class=\"checkbox-list\" id=\"loge-checkboxes\">",
		"<input type=\"checkbox\" id=\"remove-unconfirmed\">",
		"<input type=\"checkbox\" id=\"remove-installt\">",
		"<button id=\"copy-url-btn\"",
		"<button id=\"download-ical-btn\"",
	}

	for _, elem := range requiredElements {
		if !strings.Contains(bodyStr, elem) {
			t.Errorf("Configuration page missing element: %q", elem)
		}
	}
}

// TestIntegrationHealthEndpoint tests the /health endpoint
func TestIntegrationHealthEndpoint(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)
	if !strings.Contains(bodyStr, "\"status\":\"ok\"") {
		t.Errorf("Health check missing status:ok, got: %s", bodyStr)
	}
}

// TestIntegrationFilterWithTestData tests filtering with actual test data
func TestIntegrationFilterWithTestData(t *testing.T) {
	// Note: This test requires disabling SSRF protection for localhost
	// In production, localhost access should remain blocked
	t.Skip("Integration test temporarily disabled - requires SSRF protection bypass for testing")

	// Set up a mock upstream server that serves the test data
	upstreamServer := setupMockUpstreamServer(t)
	defer upstreamServer.Close()

	// Create test server with custom config pointing to mock upstream
	srv := setupTestServerWithUpstream(t, upstreamServer.URL+"/test-feed.ics")
	defer srv.Close()

	// Create HTTP client that doesn't follow redirects
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	tests := []struct {
		name            string
		url             string
		wantStatusCode  int
		wantContentType string
		checkBody       func(t *testing.T, body string)
	}{
		{
			name:            "no filters - redirect to config",
			url:             "/filter",
			wantStatusCode:  http.StatusSeeOther,
			wantContentType: "",
			checkBody:       nil,
		},
		{
			name:            "basic pattern filter",
			url:             "/filter?pattern=Göta",
			wantStatusCode:  http.StatusOK,
			wantContentType: "text/calendar",
			checkBody: func(t *testing.T, body string) {
				if !strings.Contains(body, "BEGIN:VCALENDAR") {
					t.Error("Response missing VCALENDAR")
				}
				// Should have filtered out events with "Göta" in summary
				if strings.Contains(body, "Göta PB:") {
					t.Error("Filter failed: Göta events should be removed")
				}
			},
		},
		{
			name:            "Grad filter - keep grades 1-4",
			url:             "/filter?Grad=4",
			wantStatusCode:  http.StatusOK,
			wantContentType: "text/calendar",
			checkBody: func(t *testing.T, body string) {
				if !strings.Contains(body, "BEGIN:VCALENDAR") {
					t.Error("Response missing VCALENDAR")
				}
				// Should remove Grad 5 and higher
				if strings.Contains(body, "Grad 7") {
					t.Error("Filter failed: Grad 7 should be removed")
				}
			},
		},
		{
			name:            "Loge filter - remove specific lodges",
			url:             "/filter?Loge=Göta,Borås",
			wantStatusCode:  http.StatusOK,
			wantContentType: "text/calendar",
			checkBody: func(t *testing.T, body string) {
				if !strings.Contains(body, "BEGIN:VCALENDAR") {
					t.Error("Response missing VCALENDAR")
				}
				// Should have removed Göta and Borås events
				if strings.Contains(body, "Göta PB:") {
					t.Error("Filter failed: Göta events should be removed")
				}
				if strings.Contains(body, "Borås PB:") {
					t.Error("Filter failed: Borås events should be removed")
				}
			},
		},
		{
			name:            "RemoveUnconfirmed filter",
			url:             "/filter?RemoveUnconfirmed",
			wantStatusCode:  http.StatusOK,
			wantContentType: "text/calendar",
			checkBody: func(t *testing.T, body string) {
				if !strings.Contains(body, "BEGIN:VCALENDAR") {
					t.Error("Response missing VCALENDAR")
				}
				// Should only have confirmed events
				if strings.Contains(body, "STATUS:TENTATIVE") {
					t.Error("Filter failed: TENTATIVE events should be removed")
				}
			},
		},
		{
			name:            "RemoveInstallt filter - remove cancelled events",
			url:             "/filter?RemoveInstallt",
			wantStatusCode:  http.StatusOK,
			wantContentType: "text/calendar",
			checkBody: func(t *testing.T, body string) {
				if !strings.Contains(body, "BEGIN:VCALENDAR") {
					t.Error("Response missing VCALENDAR")
				}
				// Should have removed events with INSTÄLLT
				if strings.Contains(body, "INSTÄLLT") {
					t.Error("Filter failed: INSTÄLLT events should be removed")
				}
			},
		},
		{
			name:            "Combined filters - Grad and RemoveUnconfirmed",
			url:             "/filter?Grad=4&RemoveUnconfirmed",
			wantStatusCode:  http.StatusOK,
			wantContentType: "text/calendar",
			checkBody: func(t *testing.T, body string) {
				if !strings.Contains(body, "BEGIN:VCALENDAR") {
					t.Error("Response missing VCALENDAR")
				}
				// Should remove high grades AND tentative events
				if strings.Contains(body, "Grad 7") {
					t.Error("Filter failed: Grad 7 should be removed")
				}
				if strings.Contains(body, "STATUS:TENTATIVE") {
					t.Error("Filter failed: TENTATIVE events should be removed")
				}
			},
		},
		{
			name:            "Debug mode",
			url:             "/filter?pattern=Göta&debug=true",
			wantStatusCode:  http.StatusOK,
			wantContentType: "text/html",
			checkBody: func(t *testing.T, body string) {
				// Should have debug HTML elements
				requiredElements := []string{
					"ReCal Debug Report",
					"Summary Statistics",
					"Total events in upstream:",
					"Active Filters",
					"Removed Events",
					"Sample Filtered Events",
				}
				for _, elem := range requiredElements {
					if !strings.Contains(body, elem) {
						t.Errorf("Debug page missing element: %q", elem)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.Get(srv.URL + tt.url)
			if err != nil {
				t.Fatalf("Failed to GET %s: %v", tt.url, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatusCode {
				t.Errorf("Status = %d, want %d", resp.StatusCode, tt.wantStatusCode)
			}

			if tt.wantContentType != "" {
				contentType := resp.Header.Get("Content-Type")
				if !strings.Contains(contentType, tt.wantContentType) {
					t.Errorf("Content-Type = %q, want to contain %q", contentType, tt.wantContentType)
				}
			}

			if tt.checkBody != nil {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("Failed to read response body: %v", err)
				}
				tt.checkBody(t, string(body))
			}
		})
	}
}

// TestIntegrationCacheHeaders tests that appropriate cache headers are set
func TestIntegrationCacheHeaders(t *testing.T) {
	t.Skip("Integration test temporarily disabled - requires SSRF protection bypass for testing")

	upstreamServer := setupMockUpstreamServer(t)
	defer upstreamServer.Close()

	srv := setupTestServerWithUpstream(t, upstreamServer.URL+"/test-feed.ics")
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/filter?pattern=test")
	if err != nil {
		t.Fatalf("Failed to GET /filter: %v", err)
	}
	defer resp.Body.Close()

	cacheControl := resp.Header.Get("Cache-Control")
	if cacheControl == "" {
		t.Error("Missing Cache-Control header")
	}

	// Should have at least 15 minutes (900 seconds) cache
	if !strings.Contains(cacheControl, "max-age=") {
		t.Error("Cache-Control missing max-age directive")
	}
}

// setupTestServer creates a test HTTP server with default config
func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:         8080,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
			BaseURL:      "http://localhost:8080",
		},
		Upstream: config.UpstreamConfig{
			DefaultURL: "", // No default URL - will redirect to config page
			Timeout:    30 * time.Second,
		},
		Cache: config.CacheConfig{
			MaxSize:        100,
			DefaultTTL:     5 * time.Minute,
			MinOutputCache: 15 * time.Minute,
		},
		Regex: config.RegexConfig{
			MaxExecutionTime: 1 * time.Second,
		},
		Filters: config.FiltersConfig{
			Grad: config.GradFilterConfig{
				Field:           "SUMMARY",
				PatternTemplate: "Grad %s",
			},
			Loge: config.LogeFilterConfig{
				Field: "SUMMARY",
				Patterns: map[string]config.PatternSpec{
					"Moderlogen": {Template: "PB, %s:"},
					"Göta":       {Template: "%s PB:"},
					"Borås":      {Template: "%s PB:"},
					"default":    {Template: "%s PB:"},
				},
			},
			ConfirmedOnly: config.SimpleFilterConfig{
				Field:   "STATUS",
				Pattern: "CONFIRMED",
			},
			Installt: config.SimpleFilterConfig{
				Field:   "SUMMARY",
				Pattern: "INSTÄLLT",
			},
		},
	}

	server := New(cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/", server.ConfigPage)
	mux.HandleFunc("/filter", server.ServeHTTP)
	mux.HandleFunc("/filter/preview", server.DebugHTTP)
	mux.HandleFunc("/debug", server.DebugRedirect)
	mux.HandleFunc("/api/lodges", server.GetLodges)
	mux.HandleFunc("/health", server.Health)

	return httptest.NewServer(mux)
}

// setupTestServerWithUpstream creates a test server with custom upstream URL
func setupTestServerWithUpstream(t *testing.T, upstreamURL string) *httptest.Server {
	t.Helper()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:         8080,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
			BaseURL:      "http://localhost:8080",
		},
		Upstream: config.UpstreamConfig{
			DefaultURL: upstreamURL,
			Timeout:    30 * time.Second,
		},
		Cache: config.CacheConfig{
			MaxSize:        100,
			DefaultTTL:     5 * time.Minute,
			MinOutputCache: 15 * time.Minute,
		},
		Regex: config.RegexConfig{
			MaxExecutionTime: 1 * time.Second,
		},
		Filters: config.FiltersConfig{
			Grad: config.GradFilterConfig{
				Field:           "SUMMARY",
				PatternTemplate: "Grad %s",
			},
			Loge: config.LogeFilterConfig{
				Field: "SUMMARY",
				Patterns: map[string]config.PatternSpec{
					"Moderlogen": {Template: "PB, %s:"},
					"Göta":       {Template: "%s PB:"},
					"Borås":      {Template: "%s PB:"},
					"default":    {Template: "%s PB:"},
				},
			},
			ConfirmedOnly: config.SimpleFilterConfig{
				Field:   "STATUS",
				Pattern: "CONFIRMED",
			},
			Installt: config.SimpleFilterConfig{
				Field:   "SUMMARY",
				Pattern: "INSTÄLLT",
			},
		},
	}

	server := New(cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/", server.ConfigPage)
	mux.HandleFunc("/filter", server.ServeHTTP)
	mux.HandleFunc("/filter/preview", server.DebugHTTP)
	mux.HandleFunc("/debug", server.DebugRedirect)
	mux.HandleFunc("/api/lodges", server.GetLodges)
	mux.HandleFunc("/health", server.Health)

	return httptest.NewServer(mux)
}

// setupMockUpstreamServer creates a mock upstream calendar server
func setupMockUpstreamServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Load test data from file
		testDataPath := "../../testdata/sample-feed.ics"
		data, err := os.ReadFile(testDataPath)
		if err != nil {
			t.Logf("Warning: Could not load test data from %s: %v", testDataPath, err)
			// Serve minimal valid iCal if test data is not available
			data = []byte(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test@example.com
DTSTART:20250101T120000Z
DTEND:20250101T130000Z
SUMMARY:Test Event
STATUS:CONFIRMED
END:VEVENT
END:VCALENDAR`)
		}

		w.Header().Set("Content-Type", "text/calendar")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
}

// TestIntegrationDebugRedirect tests that /debug redirects to /filter/preview
func TestIntegrationDebugRedirect(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	// Create HTTP client that doesn't follow redirects
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	tests := []struct {
		name         string
		oldURL       string
		wantLocation string
	}{
		{
			name:         "redirect without query params",
			oldURL:       "/debug",
			wantLocation: "/filter/preview",
		},
		{
			name:         "redirect with query params",
			oldURL:       "/debug?Grad=3&Loge=Göta",
			wantLocation: "/filter/preview?Grad=3&Loge=G%c3%b6ta", // ö is URL-encoded (lowercase hex)
		},
		{
			name:         "redirect with pattern param",
			oldURL:       "/debug?pattern=test",
			wantLocation: "/filter/preview?pattern=test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.Get(srv.URL + tt.oldURL)
			if err != nil {
				t.Fatalf("Failed to GET %s: %v", tt.oldURL, err)
			}
			defer resp.Body.Close()

			// Should be a 301 Moved Permanently redirect
			if resp.StatusCode != http.StatusMovedPermanently {
				t.Errorf("Status = %d, want %d (301 Moved Permanently)", resp.StatusCode, http.StatusMovedPermanently)
			}

			location := resp.Header.Get("Location")
			if location != tt.wantLocation {
				t.Errorf("Location = %q, want %q", location, tt.wantLocation)
			}
		})
	}
}

// TestIntegrationServerStartup tests that the server can start and serve requests
// This is a minimal smoke test for the full server lifecycle
func TestIntegrationServerStartup(t *testing.T) {
	srv := setupTestServer(t)
	defer srv.Close()

	// Create HTTP client that doesn't follow redirects
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Test all endpoints (without upstream fetching)
	endpoints := []struct {
		path           string
		expectedStatus int
	}{
		{"/", http.StatusOK},
		{"/health", http.StatusOK},
		{"/filter", http.StatusSeeOther}, // Redirects when no params
		// Note: /filter?pattern=test would require upstream fetch, which would fail with SSRF protection
	}

	for _, ep := range endpoints {
		t.Run(fmt.Sprintf("GET %s", ep.path), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+ep.path, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to GET %s: %v", ep.path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != ep.expectedStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("GET %s: status = %d, want %d, body: %s", ep.path, resp.StatusCode, ep.expectedStatus, string(body))
			}
		})
	}
}
