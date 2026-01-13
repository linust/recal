package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Upstream UpstreamConfig `yaml:"upstream"`
	Cache    CacheConfig    `yaml:"cache"`
	Regex    RegexConfig    `yaml:"regex"`
	Filters  FiltersConfig  `yaml:"filters"`
	Feeds    FeedsConfig    `yaml:"feeds"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
	BaseURL      string        `yaml:"base_url"`
}

// UpstreamConfig holds upstream feed configuration
type UpstreamConfig struct {
	DefaultURL string        `yaml:"default_url"`
	Timeout    time.Duration `yaml:"timeout"`
}

// CacheConfig holds caching configuration
type CacheConfig struct {
	MaxSize        int           `yaml:"max_size"`
	MaxMemory      int64         `yaml:"max_memory"`      // Maximum memory in bytes
	DefaultTTL     time.Duration `yaml:"default_ttl"`
	MinOutputCache time.Duration `yaml:"min_output_cache"`
	MaxTTL         time.Duration `yaml:"max_ttl"`          // Maximum TTL allowed
}

// RegexConfig holds regex execution configuration
type RegexConfig struct {
	MaxExecutionTime time.Duration `yaml:"max_execution_time"`
}

// FiltersConfig holds special filter configurations
type FiltersConfig struct {
	Grade         GradeFilterConfig  `yaml:"grade"`
	Lodge         LodgeFilterConfig  `yaml:"lodge"`
	ConfirmedOnly SimpleFilterConfig `yaml:"confirmed_only"`
	Installt      SimpleFilterConfig `yaml:"installt"`
}

// GradeFilterConfig holds Grade filter configuration
type GradeFilterConfig struct {
	Field           string `yaml:"field"`
	PatternTemplate string `yaml:"pattern_template"`
}

// LodgeFilterConfig holds Lodge filter configuration
type LodgeFilterConfig struct {
	Field    string                 `yaml:"field"`
	Names    []string               `yaml:"names"`
	Patterns map[string]PatternSpec `yaml:"patterns"`
}

// PatternSpec holds a pattern template specification
type PatternSpec struct {
	Template string `yaml:"template"`
}

// SimpleFilterConfig holds simple filter configuration
type SimpleFilterConfig struct {
	Field       string `yaml:"field"`
	Pattern     string `yaml:"pattern"`
	Description string `yaml:"description"`
}

// FeedsConfig holds named feeds configuration
type FeedsConfig struct {
	StoragePath    string        `yaml:"storage_path"`     // Where to store feed data
	CacheMaxAge    time.Duration `yaml:"cache_max_age"`    // How long to cache feeds in memory
	RetentionDays  int           `yaml:"retention_days"`   // Auto-delete after N days of inactivity (0 = never)
}

// Load loads configuration from a YAML file with environment variable overrides
func Load(configPath string) (*Config, error) {
	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(&cfg)

	// Validate configuration
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the configuration
func applyEnvOverrides(cfg *Config) {
	if port := os.Getenv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Server.Port = p
		}
	}

	if baseURL := os.Getenv("BASE_URL"); baseURL != "" {
		cfg.Server.BaseURL = baseURL
	}

	if url := os.Getenv("DEFAULT_UPSTREAM"); url != "" {
		cfg.Upstream.DefaultURL = url
	}

	if maxSize := os.Getenv("CACHE_MAX_SIZE"); maxSize != "" {
		if size, err := strconv.Atoi(maxSize); err == nil {
			cfg.Cache.MaxSize = size
		}
	}

	if ttl := os.Getenv("CACHE_DEFAULT_TTL"); ttl != "" {
		if d, err := time.ParseDuration(ttl); err == nil {
			cfg.Cache.DefaultTTL = d
		}
	}

	if minCache := os.Getenv("CACHE_MIN_OUTPUT"); minCache != "" {
		if d, err := time.ParseDuration(minCache); err == nil {
			cfg.Cache.MinOutputCache = d
		}
	}

	if timeout := os.Getenv("UPSTREAM_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			cfg.Upstream.Timeout = d
		}
	}

	if maxRegex := os.Getenv("MAX_REGEX_TIME"); maxRegex != "" {
		if d, err := time.ParseDuration(maxRegex); err == nil {
			cfg.Regex.MaxExecutionTime = d
		}
	}
}

// validate validates the configuration
func validate(cfg *Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	if cfg.Server.BaseURL == "" {
		return fmt.Errorf("server base URL cannot be empty")
	}

	if cfg.Upstream.DefaultURL == "" {
		return fmt.Errorf("upstream default URL cannot be empty")
	}

	if cfg.Cache.MaxSize <= 0 {
		return fmt.Errorf("cache max size must be positive")
	}

	if cfg.Cache.DefaultTTL <= 0 {
		return fmt.Errorf("cache default TTL must be positive")
	}

	if cfg.Cache.MinOutputCache <= 0 {
		return fmt.Errorf("cache min output cache must be positive")
	}

	if cfg.Cache.MaxMemory <= 0 {
		return fmt.Errorf("cache max memory must be positive")
	}

	if cfg.Cache.MaxTTL <= 0 {
		return fmt.Errorf("cache max TTL must be positive")
	}

	if cfg.Upstream.Timeout <= 0 {
		return fmt.Errorf("upstream timeout must be positive")
	}

	if cfg.Regex.MaxExecutionTime <= 0 {
		return fmt.Errorf("regex max execution time must be positive")
	}

	// Validate filter configurations
	if cfg.Filters.Grade.Field == "" {
		return fmt.Errorf("grade filter field cannot be empty")
	}

	if cfg.Filters.Grade.PatternTemplate == "" {
		return fmt.Errorf("grade filter pattern template cannot be empty")
	}

	if cfg.Filters.Lodge.Field == "" {
		return fmt.Errorf("lodge filter field cannot be empty")
	}

	if cfg.Filters.Lodge.Patterns == nil {
		return fmt.Errorf("lodge filter patterns cannot be nil")
	}

	if _, ok := cfg.Filters.Lodge.Patterns["default"]; !ok {
		return fmt.Errorf("lodge filter must have a default pattern")
	}

	// Validate feeds configuration
	if cfg.Feeds.StoragePath == "" {
		// Set default if not specified
		cfg.Feeds.StoragePath = "./data/feeds"
	}

	if cfg.Feeds.CacheMaxAge <= 0 {
		// Set default if not specified
		cfg.Feeds.CacheMaxAge = 15 * time.Minute
	}

	return nil
}

// GetLodgePattern returns the pattern template for a given lodge name
// It also handles normalized lodge names (e.g., "Orebro" -> "Örebro")
// Returns (patternTemplate, canonicalName)
func (c *Config) GetLodgePattern(lodgeName string) (string, string) {
	// Try exact match first
	if spec, ok := c.Filters.Lodge.Patterns[lodgeName]; ok {
		return spec.Template, lodgeName
	}

	// Try to find a match using denormalized name
	// Check if any canonical lodge name normalizes to this input
	for _, canonicalName := range c.Filters.Lodge.Names {
		if NormalizeSwedish(canonicalName) == lodgeName {
			// Found a match, use the canonical name
			if spec, ok := c.Filters.Lodge.Patterns[canonicalName]; ok {
				return spec.Template, canonicalName
			}
			// If no specific pattern, will fall through to default
			return c.Filters.Lodge.Patterns["default"].Template, canonicalName
		}
	}

	return c.Filters.Lodge.Patterns["default"].Template, lodgeName
}

// NormalizeSwedish normalizes Swedish characters to ASCII equivalents for cleaner URLs
// Ö/ö -> O/o, Ä/ä -> A/a, Å/å -> A/a
func NormalizeSwedish(s string) string {
	replacer := strings.NewReplacer(
		"Ö", "O",
		"ö", "o",
		"Ä", "A",
		"ä", "a",
		"Å", "A",
		"å", "a",
	)
	return replacer.Replace(s)
}
