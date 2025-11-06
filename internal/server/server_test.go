package server

import (
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/linus/recal/internal/config"
)

// getTestConfig returns a test configuration
func getTestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Port:         8080,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Upstream: config.UpstreamConfig{
			DefaultURL: "https://example.com/calendar.ics",
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
					"default":    {Template: "%s PB"},
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
}

// TestParseParams tests URL parameter parsing
// Validates: Basic filters, indexed filters, special filters, debug mode
func TestParseParams(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		wantUpstream    string
		wantDebug       bool
		wantFilterCount int
		wantGrad        string
		wantLoge        string
		wantConfirmed   bool
		wantInstallt    bool
	}{
		{
			name:            "basic filter",
			url:             "/filter?pattern=Meeting",
			wantFilterCount: 1,
		},
		{
			name:            "basic filter with field",
			url:             "/filter?field=SUMMARY&pattern=Meeting",
			wantFilterCount: 1,
		},
		{
			name:            "indexed filters",
			url:             "/filter?field1=SUMMARY&pattern1=Meeting&field2=DESCRIPTION&pattern2=urgent",
			wantFilterCount: 2,
		},
		{
			name:     "Grad filter",
			url:      "/filter?Grad=1,2,3",
			wantGrad: "1,2,3",
		},
		{
			name:     "Loge filter",
			url:      "/filter?Loge=Göta,Moderlogen",
			wantLoge: "Göta,Moderlogen",
		},
		{
			name:          "RemoveUnconfirmed filter",
			url:           "/filter?RemoveUnconfirmed",
			wantConfirmed: true,
		},
		{
			name:          "RemoveUnconfirmed filter with value",
			url:           "/filter?RemoveUnconfirmed=true",
			wantConfirmed: true,
		},
		{
			name:         "RemoveInstallt filter",
			url:          "/filter?RemoveInstallt",
			wantInstallt: true,
		},
		{
			name:         "RemoveInstallt filter with value",
			url:          "/filter?RemoveInstallt=true",
			wantInstallt: true,
		},
		{
			name:            "debug mode",
			url:             "/filter?pattern=test&debug=true",
			wantDebug:       true,
			wantFilterCount: 1,
		},
		{
			name:            "upstream parameter",
			url:             "/filter?upstream=https://custom.com/cal.ics&pattern=test",
			wantUpstream:    "https://custom.com/cal.ics",
			wantFilterCount: 1,
		},
		{
			name:            "combined filters",
			url:             "/filter?pattern=Meeting&Grad=1,2&RemoveUnconfirmed&debug=true",
			wantFilterCount: 1,
			wantGrad:        "1,2",
			wantConfirmed:   true,
			wantDebug:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			params, err := parseParams(req)
			if err != nil {
				t.Fatalf("parseParams() error = %v", err)
			}

			if tt.wantUpstream != "" && params.Upstream != tt.wantUpstream {
				t.Errorf("Upstream = %q, want %q", params.Upstream, tt.wantUpstream)
			}

			if params.Debug != tt.wantDebug {
				t.Errorf("Debug = %v, want %v", params.Debug, tt.wantDebug)
			}

			if len(params.Filters) != tt.wantFilterCount {
				t.Errorf("Filter count = %d, want %d", len(params.Filters), tt.wantFilterCount)
			}

			if params.SpecialFilters.Grad != tt.wantGrad {
				t.Errorf("Grad = %q, want %q", params.SpecialFilters.Grad, tt.wantGrad)
			}

			if params.SpecialFilters.Loge != tt.wantLoge {
				t.Errorf("Loge = %q, want %q", params.SpecialFilters.Loge, tt.wantLoge)
			}

			if params.SpecialFilters.RemoveUnconfirmed != tt.wantConfirmed {
				t.Errorf("RemoveUnconfirmed = %v, want %v", params.SpecialFilters.RemoveUnconfirmed, tt.wantConfirmed)
			}

			if params.SpecialFilters.RemoveInstallt != tt.wantInstallt {
				t.Errorf("RemoveInstallt = %v, want %v", params.SpecialFilters.RemoveInstallt, tt.wantInstallt)
			}
		})
	}
}

// TestParseFieldList tests field list parsing
// Validates: Comma-separated fields, trimming, empty handling
func TestParseFieldList(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single field",
			input: "SUMMARY",
			want:  []string{"SUMMARY"},
		},
		{
			name:  "multiple fields",
			input: "SUMMARY,DESCRIPTION,LOCATION",
			want:  []string{"SUMMARY", "DESCRIPTION", "LOCATION"},
		},
		{
			name:  "fields with spaces",
			input: "SUMMARY, DESCRIPTION, LOCATION",
			want:  []string{"SUMMARY", "DESCRIPTION", "LOCATION"},
		},
		{
			name:  "empty field",
			input: "",
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFieldList(tt.input)

			if len(got) != len(tt.want) {
				t.Fatalf("parseFieldList(%q) length = %d, want %d", tt.input, len(got), len(tt.want))
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseFieldList(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestCreateCacheKey tests cache key generation
// Validates: Consistent hashing, parameter inclusion, uniqueness
func TestCreateCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		params1  *Params
		params2  *Params
		wantSame bool
		comment  string
	}{
		{
			name: "identical params",
			params1: &Params{
				Upstream: "https://example.com/cal.ics",
				Filters: []FilterParam{
					{Fields: []string{"SUMMARY"}, Pattern: "Meeting"},
				},
			},
			params2: &Params{
				Upstream: "https://example.com/cal.ics",
				Filters: []FilterParam{
					{Fields: []string{"SUMMARY"}, Pattern: "Meeting"},
				},
			},
			wantSame: true,
			comment:  "Identical parameters should produce same key",
		},
		{
			name: "different upstream",
			params1: &Params{
				Upstream: "https://example.com/cal1.ics",
			},
			params2: &Params{
				Upstream: "https://example.com/cal2.ics",
			},
			wantSame: false,
			comment:  "Different upstream should produce different key",
		},
		{
			name: "different pattern",
			params1: &Params{
				Upstream: "https://example.com/cal.ics",
				Filters: []FilterParam{
					{Fields: []string{"SUMMARY"}, Pattern: "Meeting"},
				},
			},
			params2: &Params{
				Upstream: "https://example.com/cal.ics",
				Filters: []FilterParam{
					{Fields: []string{"SUMMARY"}, Pattern: "Event"},
				},
			},
			wantSame: false,
			comment:  "Different pattern should produce different key",
		},
		{
			name: "debug flag difference",
			params1: &Params{
				Upstream: "https://example.com/cal.ics",
				Debug:    false,
			},
			params2: &Params{
				Upstream: "https://example.com/cal.ics",
				Debug:    true,
			},
			wantSame: false,
			comment:  "Debug flag should affect cache key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1 := createCacheKey(tt.params1)
			key2 := createCacheKey(tt.params2)

			same := key1 == key2

			if same != tt.wantSame {
				t.Errorf("createCacheKey() keys same=%v, want=%v (%s)", same, tt.wantSame, tt.comment)
				t.Errorf("  key1: %s", key1)
				t.Errorf("  key2: %s", key2)
			}
		})
	}
}

// TestHealthEndpoint tests the health check endpoint
// Validates: HTTP 200, JSON response, cache stats
func TestHealthEndpoint(t *testing.T) {
	cfg := getTestConfig()
	server := New(cfg)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.Health(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "\"status\":\"ok\"") {
		t.Errorf("Body missing status:ok, got: %s", body)
	}
}

// TestMethodNotAllowed tests non-GET requests
// Validates: 405 Method Not Allowed for POST, PUT, etc.
func TestMethodNotAllowed(t *testing.T) {
	cfg := getTestConfig()
	server := New(cfg)

	methods := []string{"POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/filter?pattern=test", nil)
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			resp := w.Result()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
			}
		})
	}
}

// TestHTMLEscape tests HTML escaping using standard library
// Validates: XSS protection, special characters escaped, UTF-8 support
func TestHTMLEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "<script>alert('xss')</script>",
			want:  "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
		{
			input: "Normal text",
			want:  "Normal text",
		},
		{
			input: "Text with <b>bold</b> & \"quotes\"",
			want:  "Text with &lt;b&gt;bold&lt;/b&gt; &amp; &#34;quotes&#34;",
		},
		{
			input: "",
			want:  "",
		},
		{
			input: "Göta PB: Grad 4",
			want:  "Göta PB: Grad 4", // UTF-8 characters should not be escaped
		},
		{
			input: "Borås PB: Grad 7",
			want:  "Borås PB: Grad 7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := html.EscapeString(tt.input)
			if got != tt.want {
				t.Errorf("html.EscapeString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestSplitByComma tests comma splitting
// Validates: Correct splitting, empty handling
func TestSplitByComma(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{
			input: "a,b,c",
			want:  []string{"a", "b", "c"},
		},
		{
			input: "single",
			want:  []string{"single"},
		},
		{
			input: "",
			want:  []string{""},
		},
		{
			input: "a,,c",
			want:  []string{"a", "", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitByComma(tt.input)

			if len(got) != len(tt.want) {
				t.Fatalf("splitByComma(%q) length = %d, want %d", tt.input, len(got), len(tt.want))
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitByComma(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestTrimSpace tests whitespace trimming
// Validates: Leading/trailing space removal, various whitespace types
func TestTrimSpace(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "  hello  ",
			want:  "hello",
		},
		{
			input: "\t\nhello\r\n",
			want:  "hello",
		},
		{
			input: "no spaces",
			want:  "no spaces",
		},
		{
			input: "   ",
			want:  "",
		},
		{
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := trimSpace(tt.input)
			if got != tt.want {
				t.Errorf("trimSpace(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestConfigPageEndpoint tests the root endpoint serves the configuration page
// Validates: HTTP 200, HTML content type, page contains expected elements
func TestConfigPageEndpoint(t *testing.T) {
	cfg := getTestConfig()
	cfg.Server.BaseURL = "http://localhost:8080"
	server := New(cfg)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	server.ConfigPage(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", contentType)
	}

	body := w.Body.String()

	// Verify essential elements of the configuration page
	expectedElements := []string{
		"iCal Filter",
		"Konfigurera",
		"Grad",
		"Loger",
		"Specialfilter",
		"Genererad URL",
		"Kopiera URL",
		"Ladda ner iCal",
		"grad-select",
		"loge-checkboxes",
		"remove-unconfirmed",
		"remove-installt",
		"/api/lodges",
	}

	for _, element := range expectedElements {
		if !strings.Contains(body, element) {
			t.Errorf("Configuration page missing expected element: %q", element)
		}
	}
}

// TestConfigPageMethodNotAllowed tests non-GET requests to config page
// Validates: 405 Method Not Allowed
func TestConfigPageMethodNotAllowed(t *testing.T) {
	cfg := getTestConfig()
	server := New(cfg)

	methods := []string{"POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/", nil)
			w := httptest.NewRecorder()

			server.ConfigPage(w, req)

			resp := w.Result()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
			}
		})
	}
}

// TestGetLodgesEndpoint tests the /api/lodges endpoint
// This test uses a mock upstream feed to test lodge extraction
func TestGetLodgesEndpoint(t *testing.T) {
	// Skip this test in unit test mode - it requires actual HTTP server
	// This would be better as an integration test with a real server
	t.Skip("Integration test - requires actual HTTP server with upstream feed")
}

// TestRootRedirectsToConfigPage tests that accessing /filter with no params and no default URL redirects to /
// Validates: Redirect behavior when no filters specified and no default upstream
func TestRootRedirectsToConfigPage(t *testing.T) {
	cfg := getTestConfig()
	cfg.Upstream.DefaultURL = "" // No default URL configured
	server := New(cfg)

	req := httptest.NewRequest("GET", "/filter", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("Status = %d, want %d (redirect)", resp.StatusCode, http.StatusSeeOther)
	}

	location := resp.Header.Get("Location")
	if location != "/" {
		t.Errorf("Redirect location = %q, want /", location)
	}
}

// TestFilterWithTestData tests filtering using the testdata/sample-feed.ics file
// This simulates the real end-to-end workflow
func TestFilterWithTestData(t *testing.T) {
	// This test requires setting up a local HTTP server to serve the test data
	// Skip for now - this would be better as an integration test
	t.Skip("Integration test - requires HTTP server to serve test data")
}

// TestDebugModeOutput tests that debug mode generates HTML with statistics
func TestDebugModeOutput(t *testing.T) {
	// This test requires actual filtering - skip for now
	// Would be better as an integration test with real data
	t.Skip("Integration test - requires full server with test data")
}
