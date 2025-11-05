package filter

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/linus/recal/internal/config"
	"github.com/linus/recal/internal/parser"
)

// Filter represents a single filter rule
type Filter struct {
	Fields  []string       // Fields to search in (e.g., ["SUMMARY", "DESCRIPTION"])
	Pattern *regexp.Regexp // Compiled regex pattern
	Raw     string         // Original pattern for display
	Invert  bool           // If true, keep matching events; if false, remove matching events
}

// MatchResult represents the result of a filter match
type MatchResult struct {
	EventUID     string
	EventSummary string
	FilterRaw    string
	Field        string
	MatchedText  string
}

// Engine is the filter engine that applies filters to events
type Engine struct {
	filters []Filter
	cfg     *config.Config
}

// NewEngine creates a new filter engine
func NewEngine(cfg *config.Config) *Engine {
	return &Engine{
		filters: []Filter{},
		cfg:     cfg,
	}
}

// AddFilter adds a basic filter
func (e *Engine) AddFilter(fields []string, pattern string) error {
	if pattern == "" {
		return fmt.Errorf("pattern cannot be empty")
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
	}

	e.filters = append(e.filters, Filter{
		Fields:  fields,
		Pattern: re,
		Raw:     pattern,
		Invert:  false,
	})

	return nil
}

// AddGradFilter adds a Grad filter (e.g., Grad=1,2,3 -> matches "Grad: [1]", "Grad: [2]", "Grad: [3]")
func (e *Engine) AddGradFilter(threshold string) error {
	if threshold == "" {
		return fmt.Errorf("threshold cannot be empty")
	}

	// Parse the threshold grade number
	maxGrade := 0
	for _, r := range threshold {
		if r >= '0' && r <= '9' {
			digit := int(r - '0')
			if digit > maxGrade {
				maxGrade = digit
			}
		}
	}

	if maxGrade == 0 {
		return fmt.Errorf("no valid grade threshold found in %q", threshold)
	}

	// Create a pattern that matches all grades ABOVE the threshold
	// E.g., for threshold=4, match Grad 5, Grad 6, Grad 7, Grad 8, Grad 9, Grad 10
	// This will filter OUT (remove) all grades above the threshold
	var patterns []string
	for grade := maxGrade + 1; grade <= 10; grade++ {
		pattern := fmt.Sprintf(e.cfg.Filters.Grad.PatternTemplate, fmt.Sprintf("%d", grade))
		patterns = append(patterns, pattern)
	}

	if len(patterns) == 0 {
		// If threshold is 10, no grades to filter out
		return nil
	}

	combinedPattern := "(" + strings.Join(patterns, "|") + ")"

	re, err := regexp.Compile(combinedPattern)
	if err != nil {
		return fmt.Errorf("failed to compile grad pattern %q: %w", combinedPattern, err)
	}

	e.filters = append(e.filters, Filter{
		Fields:  []string{e.cfg.Filters.Grad.Field},
		Pattern: re,
		Raw:     combinedPattern,
		Invert:  false,
	})

	return nil
}

// AddLogeFilter adds a Loge filter (e.g., Loge=Göta,Borås,Moderlogen)
func (e *Engine) AddLogeFilter(lodges string) error {
	if lodges == "" {
		return fmt.Errorf("lodges cannot be empty")
	}

	// Split lodge names
	lodgeNames := strings.Split(lodges, ",")
	var patterns []string

	for _, name := range lodgeNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		// Get the pattern template for this lodge
		template := e.cfg.GetLogePattern(name)

		// Replace %s with the lodge name
		pattern := strings.Replace(template, "%s", name, -1)
		patterns = append(patterns, regexp.QuoteMeta(pattern))
	}

	if len(patterns) == 0 {
		return fmt.Errorf("no valid lodge names found in %q", lodges)
	}

	// Combine patterns with OR
	combinedPattern := "(" + strings.Join(patterns, "|") + ")"

	re, err := regexp.Compile(combinedPattern)
	if err != nil {
		return fmt.Errorf("failed to compile loge pattern %q: %w", combinedPattern, err)
	}

	e.filters = append(e.filters, Filter{
		Fields:  []string{e.cfg.Filters.Loge.Field},
		Pattern: re,
		Raw:     combinedPattern,
		Invert:  false,
	})

	return nil
}

// AddConfirmedOnlyFilter adds the ConfirmedOnly filter (inverted - keeps matching events)
func (e *Engine) AddConfirmedOnlyFilter() error {
	re, err := regexp.Compile(e.cfg.Filters.ConfirmedOnly.Pattern)
	if err != nil {
		return fmt.Errorf("failed to compile confirmed_only pattern: %w", err)
	}

	e.filters = append(e.filters, Filter{
		Fields:  []string{e.cfg.Filters.ConfirmedOnly.Field},
		Pattern: re,
		Raw:     e.cfg.Filters.ConfirmedOnly.Pattern,
		Invert:  true, // Keep matching events
	})

	return nil
}

// AddInstalltFilter adds the Installt filter (removes events with "INSTÄLLT")
func (e *Engine) AddInstalltFilter() error {
	re, err := regexp.Compile(e.cfg.Filters.Installt.Pattern)
	if err != nil {
		return fmt.Errorf("failed to compile installt pattern: %w", err)
	}

	e.filters = append(e.filters, Filter{
		Fields:  []string{e.cfg.Filters.Installt.Field},
		Pattern: re,
		Raw:     e.cfg.Filters.Installt.Pattern,
		Invert:  false, // Remove matching events
	})

	return nil
}

// Apply applies all filters to a calendar and returns the filtered calendar
// Also returns match results for debug mode
func (e *Engine) Apply(cal *parser.Calendar) (*parser.Calendar, []MatchResult) {
	var filteredEvents []*parser.Event
	var matchResults []MatchResult

	for _, event := range cal.Events {
		keep := e.shouldKeepEvent(event, &matchResults)
		if keep {
			filteredEvents = append(filteredEvents, event)
		}
	}

	return &parser.Calendar{
		Events: filteredEvents,
		Raw:    cal.Raw,
	}, matchResults
}

// shouldKeepEvent determines if an event should be kept based on all filters
// Returns true if the event should be kept, false if it should be removed
func (e *Engine) shouldKeepEvent(event *parser.Event, matchResults *[]MatchResult) bool {
	// If no filters, keep everything
	if len(e.filters) == 0 {
		return true
	}

	// Apply each filter
	for _, filter := range e.filters {
		matched, field, matchedText := e.matchFilter(filter, event)

		if matched {
			// Record the match for debug mode
			*matchResults = append(*matchResults, MatchResult{
				EventUID:     event.UID,
				EventSummary: event.Summary,
				FilterRaw:    filter.Raw,
				Field:        field,
				MatchedText:  matchedText,
			})

			// If inverted filter (like ConfirmedOnly), we want to REMOVE events that DON'T match
			// So if it matches an inverted filter, keep going (don't remove yet)
			// If inverted filter doesn't match, remove the event
			if filter.Invert {
				// Match found on inverted filter - keep going to check other filters
				continue
			} else {
				// Match found on normal filter - remove this event
				return false
			}
		} else {
			// No match
			if filter.Invert {
				// Inverted filter didn't match - remove the event
				return false
			}
			// Normal filter didn't match - keep going to check other filters
		}
	}

	// If we get here, the event passed all filters
	return true
}

// matchFilter checks if a single filter matches an event
// Returns (matched, fieldName, matchedText)
func (e *Engine) matchFilter(filter Filter, event *parser.Event) (bool, string, string) {
	for _, field := range filter.Fields {
		value := event.GetField(field)
		if value == "" {
			continue
		}

		if filter.Pattern.MatchString(value) {
			return true, field, value
		}
	}
	return false, "", ""
}

// GetFilters returns all filters for display purposes
func (e *Engine) GetFilters() []Filter {
	return e.filters
}

// Stats returns statistics about the filtering
type Stats struct {
	TotalEvents    int
	FilteredEvents int
	RemovedEvents  int
}

// GetStats calculates statistics from the original and filtered calendars
func GetStats(original, filtered *parser.Calendar) Stats {
	return Stats{
		TotalEvents:    len(original.Events),
		FilteredEvents: len(filtered.Events),
		RemovedEvents:  len(original.Events) - len(filtered.Events),
	}
}
