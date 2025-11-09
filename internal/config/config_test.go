package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadConfig tests loading a valid configuration file
// Validates: YAML parsing, struct mapping, default values
func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	configContent := `
server:
  port: 8080
  read_timeout: 15s
  write_timeout: 15s
  idle_timeout: 60s
  base_url: "http://localhost:8080"

upstream:
  default_url: "https://example.com/calendar.ics"
  timeout: 30s

cache:
  max_size: 100
  max_memory: 20971520
  default_ttl: 5m
  min_output_cache: 15m
  max_ttl: 24h

regex:
  max_execution_time: 1s

filters:
  grade:
    field: "SUMMARY"
    pattern_template: "Grade: [%s]"

  lodge:
    field: "SUMMARY"
    patterns:
      "Moderlogen":
        template: "PB, %s:"
      "Göta":
        template: "%s PB:"
      default:
        template: "%s PB"

  confirmed_only:
    field: "STATUS"
    pattern: "CONFIRMED"
    description: "Only confirmed events"

  installt:
    field: "SUMMARY"
    pattern: "INSTÄLLT"
    description: "Remove cancelled events"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Validate server config
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 15*time.Second {
		t.Errorf("Server.ReadTimeout = %v, want 15s", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != 15*time.Second {
		t.Errorf("Server.WriteTimeout = %v, want 15s", cfg.Server.WriteTimeout)
	}
	if cfg.Server.IdleTimeout != 60*time.Second {
		t.Errorf("Server.IdleTimeout = %v, want 60s", cfg.Server.IdleTimeout)
	}

	// Validate upstream config
	if cfg.Upstream.DefaultURL != "https://example.com/calendar.ics" {
		t.Errorf("Upstream.DefaultURL = %q, want https://example.com/calendar.ics", cfg.Upstream.DefaultURL)
	}
	if cfg.Upstream.Timeout != 30*time.Second {
		t.Errorf("Upstream.Timeout = %v, want 30s", cfg.Upstream.Timeout)
	}

	// Validate cache config
	if cfg.Cache.MaxSize != 100 {
		t.Errorf("Cache.MaxSize = %d, want 100", cfg.Cache.MaxSize)
	}
	if cfg.Cache.MaxMemory != 20971520 {
		t.Errorf("Cache.MaxMemory = %d, want 20971520", cfg.Cache.MaxMemory)
	}
	if cfg.Cache.DefaultTTL != 5*time.Minute {
		t.Errorf("Cache.DefaultTTL = %v, want 5m", cfg.Cache.DefaultTTL)
	}
	if cfg.Cache.MinOutputCache != 15*time.Minute {
		t.Errorf("Cache.MinOutputCache = %v, want 15m", cfg.Cache.MinOutputCache)
	}
	if cfg.Cache.MaxTTL != 24*time.Hour {
		t.Errorf("Cache.MaxTTL = %v, want 24h", cfg.Cache.MaxTTL)
	}

	// Validate regex config
	if cfg.Regex.MaxExecutionTime != 1*time.Second {
		t.Errorf("Regex.MaxExecutionTime = %v, want 1s", cfg.Regex.MaxExecutionTime)
	}

	// Validate filter configs
	if cfg.Filters.Grade.Field != "SUMMARY" {
		t.Errorf("Filters.Grade.Field = %q, want SUMMARY", cfg.Filters.Grade.Field)
	}
	if cfg.Filters.Grade.PatternTemplate != "Grade: [%s]" {
		t.Errorf("Filters.Grade.PatternTemplate = %q, want 'Grade: [%%s]'", cfg.Filters.Grade.PatternTemplate)
	}

	if cfg.Filters.Lodge.Field != "SUMMARY" {
		t.Errorf("Filters.Lodge.Field = %q, want SUMMARY", cfg.Filters.Lodge.Field)
	}
}

// TestEnvOverrides tests environment variable overrides
// Validates: Environment variable parsing, override precedence
func TestEnvOverrides(t *testing.T) {
	configContent := `
server:
  port: 8080
  read_timeout: 15s
  write_timeout: 15s
  idle_timeout: 60s
  base_url: "http://localhost:8080"

upstream:
  default_url: "https://example.com/calendar.ics"
  timeout: 30s

cache:
  max_size: 100
  max_memory: 20971520
  default_ttl: 5m
  min_output_cache: 15m
  max_ttl: 24h

regex:
  max_execution_time: 1s

filters:
  grade:
    field: "SUMMARY"
    pattern_template: "Grade: [%s]"
  lodge:
    field: "SUMMARY"
    patterns:
      default:
        template: "%s PB"
  confirmed_only:
    field: "STATUS"
    pattern: "CONFIRMED"
  installt:
    field: "SUMMARY"
    pattern: "INSTÄLLT"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	// Set environment variables
	testEnv := map[string]string{
		"PORT":              "9090",
		"DEFAULT_UPSTREAM":  "https://override.com/feed.ics",
		"CACHE_MAX_SIZE":    "200",
		"CACHE_DEFAULT_TTL": "10m",
		"CACHE_MIN_OUTPUT":  "30m",
		"UPSTREAM_TIMEOUT":  "60s",
		"MAX_REGEX_TIME":    "2s",
	}

	for k, v := range testEnv {
		t.Setenv(k, v)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Validate overrides
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090 (from PORT env)", cfg.Server.Port)
	}
	if cfg.Upstream.DefaultURL != "https://override.com/feed.ics" {
		t.Errorf("Upstream.DefaultURL = %q, want https://override.com/feed.ics (from DEFAULT_UPSTREAM env)", cfg.Upstream.DefaultURL)
	}
	if cfg.Cache.MaxSize != 200 {
		t.Errorf("Cache.MaxSize = %d, want 200 (from CACHE_MAX_SIZE env)", cfg.Cache.MaxSize)
	}
	if cfg.Cache.DefaultTTL != 10*time.Minute {
		t.Errorf("Cache.DefaultTTL = %v, want 10m (from CACHE_DEFAULT_TTL env)", cfg.Cache.DefaultTTL)
	}
	if cfg.Cache.MinOutputCache != 30*time.Minute {
		t.Errorf("Cache.MinOutputCache = %v, want 30m (from CACHE_MIN_OUTPUT env)", cfg.Cache.MinOutputCache)
	}
	if cfg.Upstream.Timeout != 60*time.Second {
		t.Errorf("Upstream.Timeout = %v, want 60s (from UPSTREAM_TIMEOUT env)", cfg.Upstream.Timeout)
	}
	if cfg.Regex.MaxExecutionTime != 2*time.Second {
		t.Errorf("Regex.MaxExecutionTime = %v, want 2s (from MAX_REGEX_TIME env)", cfg.Regex.MaxExecutionTime)
	}
}

// TestValidation tests configuration validation
// Validates: Invalid port, empty URL, negative values, missing required fields
func TestValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		errContains string
	}{
		{
			name: "invalid port - too high",
			config: `
server:
  port: 99999
  read_timeout: 15s
  write_timeout: 15s
  idle_timeout: 60s
upstream:
  default_url: "https://example.com/calendar.ics"
  timeout: 30s
cache:
  max_size: 100
  max_memory: 20971520
  default_ttl: 5m
  min_output_cache: 15m
  max_ttl: 24h
regex:
  max_execution_time: 1s
filters:
  grade:
    field: "SUMMARY"
    pattern_template: "Grade: [%s]"
  lodge:
    field: "SUMMARY"
    patterns:
      default:
        template: "%s PB"
  confirmed_only:
    field: "STATUS"
    pattern: "CONFIRMED"
  installt:
    field: "SUMMARY"
    pattern: "INSTÄLLT"
`,
			errContains: "invalid server port",
		},
		{
			name: "invalid port - zero",
			config: `
server:
  port: 0
  read_timeout: 15s
  write_timeout: 15s
  idle_timeout: 60s
upstream:
  default_url: "https://example.com/calendar.ics"
  timeout: 30s
cache:
  max_size: 100
  max_memory: 20971520
  default_ttl: 5m
  min_output_cache: 15m
  max_ttl: 24h
regex:
  max_execution_time: 1s
filters:
  grade:
    field: "SUMMARY"
    pattern_template: "Grade: [%s]"
  lodge:
    field: "SUMMARY"
    patterns:
      default:
        template: "%s PB"
  confirmed_only:
    field: "STATUS"
    pattern: "CONFIRMED"
  installt:
    field: "SUMMARY"
    pattern: "INSTÄLLT"
`,
			errContains: "invalid server port",
		},
		{
			name: "empty upstream URL",
			config: `
server:
  port: 8080
  read_timeout: 15s
  write_timeout: 15s
  idle_timeout: 60s
  base_url: "http://localhost:8080"
upstream:
  default_url: ""
  timeout: 30s
cache:
  max_size: 100
  max_memory: 20971520
  default_ttl: 5m
  min_output_cache: 15m
  max_ttl: 24h
regex:
  max_execution_time: 1s
filters:
  grade:
    field: "SUMMARY"
    pattern_template: "Grade: [%s]"
  lodge:
    field: "SUMMARY"
    patterns:
      default:
        template: "%s PB"
  confirmed_only:
    field: "STATUS"
    pattern: "CONFIRMED"
  installt:
    field: "SUMMARY"
    pattern: "INSTÄLLT"
`,
			errContains: "upstream default URL cannot be empty",
		},
		{
			name: "negative cache size",
			config: `
server:
  port: 8080
  read_timeout: 15s
  write_timeout: 15s
  idle_timeout: 60s
  base_url: "http://localhost:8080"
upstream:
  default_url: "https://example.com/calendar.ics"
  timeout: 30s
cache:
  max_size: -1
  default_ttl: 5m
  min_output_cache: 15m
regex:
  max_execution_time: 1s
filters:
  grade:
    field: "SUMMARY"
    pattern_template: "Grade: [%s]"
  lodge:
    field: "SUMMARY"
    patterns:
      default:
        template: "%s PB"
  confirmed_only:
    field: "STATUS"
    pattern: "CONFIRMED"
  installt:
    field: "SUMMARY"
    pattern: "INSTÄLLT"
`,
			errContains: "cache max size must be positive",
		},
		{
			name: "missing loge default pattern",
			config: `
server:
  port: 8080
  read_timeout: 15s
  write_timeout: 15s
  idle_timeout: 60s
  base_url: "http://localhost:8080"
upstream:
  default_url: "https://example.com/calendar.ics"
  timeout: 30s
cache:
  max_size: 100
  max_memory: 20971520
  default_ttl: 5m
  min_output_cache: 15m
  max_ttl: 24h
regex:
  max_execution_time: 1s
filters:
  grade:
    field: "SUMMARY"
    pattern_template: "Grade: [%s]"
  lodge:
    field: "SUMMARY"
    patterns:
      "Moderlogen":
        template: "PB, %s:"
  confirmed_only:
    field: "STATUS"
    pattern: "CONFIRMED"
  installt:
    field: "SUMMARY"
    pattern: "INSTÄLLT"
`,
			errContains: "lodge filter must have a default pattern",
		},
		{
			name: "negative max memory",
			config: `
server:
  port: 8080
  read_timeout: 15s
  write_timeout: 15s
  idle_timeout: 60s
  base_url: "http://localhost:8080"
upstream:
  default_url: "https://example.com/calendar.ics"
  timeout: 30s
cache:
  max_size: 100
  max_memory: -1
  default_ttl: 5m
  min_output_cache: 15m
  max_ttl: 24h
regex:
  max_execution_time: 1s
filters:
  grade:
    field: "SUMMARY"
    pattern_template: "Grade: [%s]"
  lodge:
    field: "SUMMARY"
    patterns:
      default:
        template: "%s PB"
  confirmed_only:
    field: "STATUS"
    pattern: "CONFIRMED"
  installt:
    field: "SUMMARY"
    pattern: "INSTÄLLT"
`,
			errContains: "cache max memory must be positive",
		},
		{
			name: "zero max TTL",
			config: `
server:
  port: 8080
  read_timeout: 15s
  write_timeout: 15s
  idle_timeout: 60s
  base_url: "http://localhost:8080"
upstream:
  default_url: "https://example.com/calendar.ics"
  timeout: 30s
cache:
  max_size: 100
  max_memory: 20971520
  default_ttl: 5m
  min_output_cache: 15m
  max_ttl: 0s
regex:
  max_execution_time: 1s
filters:
  grade:
    field: "SUMMARY"
    pattern_template: "Grade: [%s]"
  lodge:
    field: "SUMMARY"
    patterns:
      default:
        template: "%s PB"
  confirmed_only:
    field: "STATUS"
    pattern: "CONFIRMED"
  installt:
    field: "SUMMARY"
    pattern: "INSTÄLLT"
`,
			errContains: "cache max TTL must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.config), 0644); err != nil {
				t.Fatalf("Failed to create temp config file: %v", err)
			}

			_, err := Load(configPath)
			if err == nil {
				t.Fatalf("Load() succeeded, want error containing %q", tt.errContains)
			}
			if !contains(err.Error(), tt.errContains) {
				t.Errorf("Load() error = %q, want error containing %q", err.Error(), tt.errContains)
			}
		})
	}
}

// TestGetLodgePattern tests the GetLodgePattern method
// Validates: Pattern lookup, default fallback, special cases
func TestGetLodgePattern(t *testing.T) {
	cfg := &Config{
		Filters: FiltersConfig{
			Lodge: LodgeFilterConfig{
				Field: "SUMMARY",
				Patterns: map[string]PatternSpec{
					"Moderlogen": {Template: "PB, %s:"},
					"Göta":       {Template: "%s PB:"},
					"default":    {Template: "%s PB"},
				},
			},
		},
	}

	tests := []struct {
		name    string
		lodge   string
		want    string
		comment string
	}{
		{
			name:    "explicit pattern - Moderlogen",
			lodge:   "Moderlogen",
			want:    "PB, %s:",
			comment: "Special pattern for Moderlogen",
		},
		{
			name:    "explicit pattern - Göta",
			lodge:   "Göta",
			want:    "%s PB:",
			comment: "Explicit pattern for Göta",
		},
		{
			name:    "default fallback",
			lodge:   "Unknown",
			want:    "%s PB",
			comment: "Falls back to default pattern for unknown lodge",
		},
		{
			name:    "default fallback - empty",
			lodge:   "",
			want:    "%s PB",
			comment: "Falls back to default pattern for empty name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.GetLodgePattern(tt.lodge)
			if got != tt.want {
				t.Errorf("GetLodgePattern(%q) = %q, want %q (%s)", tt.lodge, got, tt.want, tt.comment)
			}
		})
	}
}

// TestLoadNonexistentFile tests loading a nonexistent config file
// Validates: File not found error handling
func TestLoadNonexistentFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("Load() succeeded, want error for nonexistent file")
	}
	if !contains(err.Error(), "failed to read config file") {
		t.Errorf("Load() error = %q, want error containing 'failed to read config file'", err.Error())
	}
}

// TestLoadInvalidYAML tests loading an invalid YAML file
// Validates: YAML parse error handling
func TestLoadInvalidYAML(t *testing.T) {
	invalidYAML := `
server:
  port: 8080
  invalid: [unclosed
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("Load() succeeded, want error for invalid YAML")
	}
	if !contains(err.Error(), "failed to parse config file") {
		t.Errorf("Load() error = %q, want error containing 'failed to parse config file'", err.Error())
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
