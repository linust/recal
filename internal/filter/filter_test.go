package filter

import (
	"testing"

	"github.com/linus/recal/internal/config"
	"github.com/linus/recal/internal/parser"
)

// getTestConfig returns a test configuration
func getTestConfig() *config.Config {
	return &config.Config{
		Filters: config.FiltersConfig{
			Grad: config.GradFilterConfig{
				Field:           "SUMMARY",
				PatternTemplate: "Grad %s", // Matches "Grad 1", "Grad 4", etc
			},
			Loge: config.LogeFilterConfig{
				Field: "SUMMARY",
				Patterns: map[string]config.PatternSpec{
					"Moderlogen": {Template: "PB, %s:"},
					"Göta":       {Template: "%s PB:"},
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

// TestAddFilter tests adding basic filters
// Validates: Filter addition, regex compilation, error handling
func TestAddFilter(t *testing.T) {
	cfg := getTestConfig()
	engine := NewEngine(cfg)

	// Add valid filter
	err := engine.AddFilter([]string{"SUMMARY"}, "Meeting")
	if err != nil {
		t.Fatalf("AddFilter() failed: %v", err)
	}

	if len(engine.filters) != 1 {
		t.Fatalf("Expected 1 filter, got %d", len(engine.filters))
	}

	// Test invalid regex
	err = engine.AddFilter([]string{"SUMMARY"}, "[invalid(")
	if err == nil {
		t.Error("AddFilter() with invalid regex should fail")
	}

	// Test empty pattern
	err = engine.AddFilter([]string{"SUMMARY"}, "")
	if err == nil {
		t.Error("AddFilter() with empty pattern should fail")
	}
}

// TestAddGradFilter tests the Grad special filter
// Validates: Grad pattern expansion, number extraction
func TestAddGradFilter(t *testing.T) {
	cfg := getTestConfig()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		comment string
	}{
		{
			name:    "single grade",
			input:   "1",
			wantErr: false,
			comment: "Single grade number should work",
		},
		{
			name:    "multiple grades",
			input:   "1,2,3",
			wantErr: false,
			comment: "Comma-separated grades should work",
		},
		{
			name:    "grades with spaces",
			input:   "1, 2, 3",
			wantErr: false,
			comment: "Grades with spaces should be cleaned",
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
			comment: "Empty input should fail",
		},
		{
			name:    "no valid grades",
			input:   "abc",
			wantErr: true,
			comment: "Non-numeric input should fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(cfg)
			err := engine.AddGradFilter(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddGradFilter(%q) error = %v, wantErr %v (%s)", tt.input, err, tt.wantErr, tt.comment)
			}
		})
	}
}

// TestAddLogeFilter tests the Loge special filter
// Validates: Lodge pattern expansion, multiple lodges, OR logic
func TestAddLogeFilter(t *testing.T) {
	cfg := getTestConfig()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		comment string
	}{
		{
			name:    "single lodge - explicit",
			input:   "Moderlogen",
			wantErr: false,
			comment: "Single lodge with explicit pattern should work",
		},
		{
			name:    "single lodge - default",
			input:   "Unknown",
			wantErr: false,
			comment: "Unknown lodge should use default pattern",
		},
		{
			name:    "multiple lodges",
			input:   "Göta,Moderlogen",
			wantErr: false,
			comment: "Multiple lodges should be combined with OR",
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
			comment: "Empty input should fail",
		},
		{
			name:    "only commas",
			input:   ",,,",
			wantErr: true,
			comment: "Only separators should fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(cfg)
			err := engine.AddLogeFilter(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddLogeFilter(%q) error = %v, wantErr %v (%s)", tt.input, err, tt.wantErr, tt.comment)
			}
		})
	}
}

// TestApplyNoFilters tests filtering with no filters
// Validates: All events should pass through unchanged
func TestApplyNoFilters(t *testing.T) {
	cfg := getTestConfig()
	engine := NewEngine(cfg)

	cal := &parser.Calendar{
		Events: []*parser.Event{
			{UID: "1", Summary: "Event 1"},
			{UID: "2", Summary: "Event 2"},
			{UID: "3", Summary: "Event 3"},
		},
	}

	filtered, matches := engine.Apply(cal)

	if len(filtered.Events) != 3 {
		t.Errorf("Expected 3 events, got %d (no filters should keep all events)", len(filtered.Events))
	}

	if len(matches) != 0 {
		t.Errorf("Expected 0 matches, got %d (no filters should produce no matches)", len(matches))
	}
}

// TestApplyBasicFilter tests basic regex filtering
// Validates: Pattern matching, event removal
func TestApplyBasicFilter(t *testing.T) {
	cfg := getTestConfig()
	engine := NewEngine(cfg)

	err := engine.AddFilter([]string{"SUMMARY"}, "Meeting")
	if err != nil {
		t.Fatalf("AddFilter() failed: %v", err)
	}

	cal := &parser.Calendar{
		Events: []*parser.Event{
			{UID: "1", Summary: "Test Meeting"},
			{UID: "2", Summary: "Regular Event"},
			{UID: "3", Summary: "Another Meeting"},
		},
	}

	filtered, matches := engine.Apply(cal)

	// Should remove the 2 events with "Meeting" in summary
	if len(filtered.Events) != 1 {
		t.Errorf("Expected 1 event after filtering, got %d", len(filtered.Events))
	}

	if filtered.Events[0].UID != "2" {
		t.Errorf("Expected event UID=2, got UID=%s", filtered.Events[0].UID)
	}

	if len(matches) != 2 {
		t.Errorf("Expected 2 matches, got %d", len(matches))
	}
}

// TestApplyGradFilter tests the Grad filter with threshold behavior
// Validates: Grad threshold filtering, removes grades above threshold
func TestApplyGradFilter(t *testing.T) {
	cfg := getTestConfig()
	engine := NewEngine(cfg)

	// Grad=2 means keep grades 1-2, filter out grades 3-10
	err := engine.AddGradFilter("2")
	if err != nil {
		t.Fatalf("AddGradFilter() failed: %v", err)
	}

	cal := &parser.Calendar{
		Events: []*parser.Event{
			{UID: "1", Summary: "Göta PB: Grad 1"},
			{UID: "2", Summary: "Borås PB: Grad 2"},
			{UID: "3", Summary: "Göta PB: Grad 3"},
			{UID: "4", Summary: "Göta PB: Grad 7"},
			{UID: "5", Summary: "Regular Event"},
		},
	}

	filtered, matches := engine.Apply(cal)

	// Should remove events with Grad 3 and Grad 7 (grades above threshold of 2)
	if len(filtered.Events) != 3 {
		t.Errorf("Expected 3 events after filtering, got %d", len(filtered.Events))
	}

	if len(matches) != 2 {
		t.Errorf("Expected 2 matches (Grad 3 and 7), got %d", len(matches))
	}

	// Check that the remaining events are correct (Grad 1, Grad 2, and Regular Event)
	remainingUIDs := map[string]bool{}
	for _, e := range filtered.Events {
		remainingUIDs[e.UID] = true
	}

	if !remainingUIDs["1"] || !remainingUIDs["2"] || !remainingUIDs["5"] {
		t.Error("Expected events 1, 2, and 5 to remain after filtering")
	}
}

// TestApplyLogeFilter tests the Loge filter
// Validates: Lodge pattern matching with special patterns
func TestApplyLogeFilter(t *testing.T) {
	cfg := getTestConfig()
	engine := NewEngine(cfg)

	err := engine.AddLogeFilter("Moderlogen,Göta")
	if err != nil {
		t.Fatalf("AddLogeFilter() failed: %v", err)
	}

	cal := &parser.Calendar{
		Events: []*parser.Event{
			{UID: "1", Summary: "PB, Moderlogen: Annual Meeting"},
			{UID: "2", Summary: "Göta PB: Monthly Event"},
			{UID: "3", Summary: "Other Lodge PB: Meeting"},
			{UID: "4", Summary: "Regular Event"},
		},
	}

	filtered, matches := engine.Apply(cal)

	// Should remove events matching Moderlogen and Göta patterns
	if len(filtered.Events) != 2 {
		t.Errorf("Expected 2 events after filtering, got %d", len(filtered.Events))
	}

	if len(matches) != 2 {
		t.Errorf("Expected 2 matches (Moderlogen and Göta), got %d", len(matches))
	}

	// Check that the remaining events are correct
	remainingUIDs := map[string]bool{}
	for _, e := range filtered.Events {
		remainingUIDs[e.UID] = true
	}

	if !remainingUIDs["3"] || !remainingUIDs["4"] {
		t.Error("Expected events 3 and 4 to remain after filtering")
	}
}

// TestApplyConfirmedOnlyFilter tests the ConfirmedOnly inverted filter
// Validates: Inverted filter logic (keep matching, remove non-matching)
func TestApplyConfirmedOnlyFilter(t *testing.T) {
	cfg := getTestConfig()
	engine := NewEngine(cfg)

	err := engine.AddConfirmedOnlyFilter()
	if err != nil {
		t.Fatalf("AddConfirmedOnlyFilter() failed: %v", err)
	}

	cal := &parser.Calendar{
		Events: []*parser.Event{
			{UID: "1", Summary: "Event 1", Status: "CONFIRMED"},
			{UID: "2", Summary: "Event 2", Status: "TENTATIVE"},
			{UID: "3", Summary: "Event 3", Status: "CONFIRMED"},
			{UID: "4", Summary: "Event 4", Status: ""},
		},
	}

	filtered, _ := engine.Apply(cal)

	// Should KEEP only CONFIRMED events (inverted filter)
	if len(filtered.Events) != 2 {
		t.Errorf("Expected 2 CONFIRMED events, got %d", len(filtered.Events))
	}

	for _, e := range filtered.Events {
		if e.Status != "CONFIRMED" {
			t.Errorf("Event %s has status %q, want CONFIRMED", e.UID, e.Status)
		}
	}
}

// TestApplyInstalltFilter tests the Installt filter
// Validates: Removal of cancelled events
func TestApplyInstalltFilter(t *testing.T) {
	cfg := getTestConfig()
	engine := NewEngine(cfg)

	err := engine.AddInstalltFilter()
	if err != nil {
		t.Fatalf("AddInstalltFilter() failed: %v", err)
	}

	cal := &parser.Calendar{
		Events: []*parser.Event{
			{UID: "1", Summary: "Regular Event"},
			{UID: "2", Summary: "INSTÄLLT: Cancelled Event"},
			{UID: "3", Summary: "Another Event"},
			{UID: "4", Summary: "INSTÄLLT: Also Cancelled"},
		},
	}

	filtered, matches := engine.Apply(cal)

	// Should remove events with "INSTÄLLT" in summary
	if len(filtered.Events) != 2 {
		t.Errorf("Expected 2 events after filtering, got %d", len(filtered.Events))
	}

	if len(matches) != 2 {
		t.Errorf("Expected 2 matches (cancelled events), got %d", len(matches))
	}

	for _, e := range filtered.Events {
		if e.UID == "2" || e.UID == "4" {
			t.Errorf("Cancelled event %s should have been removed", e.UID)
		}
	}
}

// TestApplyMultipleFilters tests combining multiple filters
// Validates: AND logic, correct filtering with multiple rules
func TestApplyMultipleFilters(t *testing.T) {
	cfg := getTestConfig()
	engine := NewEngine(cfg)

	// Add multiple filters
	err := engine.AddInstalltFilter()
	if err != nil {
		t.Fatalf("AddInstalltFilter() failed: %v", err)
	}

	// Grad=1 means keep grades 1, filter out grades 2-10
	err = engine.AddGradFilter("1")
	if err != nil {
		t.Fatalf("AddGradFilter() failed: %v", err)
	}

	cal := &parser.Calendar{
		Events: []*parser.Event{
			{UID: "1", Summary: "Regular Event"},
			{UID: "2", Summary: "INSTÄLLT: Cancelled"},
			{UID: "3", Summary: "Göta PB: Grad 1"},
			{UID: "4", Summary: "Göta PB: Grad 2"},
			{UID: "5", Summary: "INSTÄLLT: Göta PB: Grad 2"},
		},
	}

	filtered, matches := engine.Apply(cal)

	// Should remove:
	// - Event 2 (INSTÄLLT)
	// - Event 4 (Grad 2, above threshold)
	// - Event 5 (both INSTÄLLT and Grad 2)
	// Should keep: Event 1 (regular) and Event 3 (Grad 1, at threshold)
	if len(filtered.Events) != 2 {
		t.Errorf("Expected 2 events after filtering, got %d", len(filtered.Events))
	}

	// Check that events 1 and 3 remain
	remainingUIDs := map[string]bool{}
	for _, e := range filtered.Events {
		remainingUIDs[e.UID] = true
	}

	if !remainingUIDs["1"] || !remainingUIDs["3"] {
		t.Error("Expected events 1 and 3 to remain after filtering")
	}

	// Event 5 matches both filters, but we should only count unique events
	if len(matches) < 3 {
		t.Errorf("Expected at least 3 matches, got %d", len(matches))
	}
}

// TestApplyMultipleFieldsFilter tests filtering across multiple fields
// Validates: OR logic for fields within a single filter
func TestApplyMultipleFieldsFilter(t *testing.T) {
	cfg := getTestConfig()
	engine := NewEngine(cfg)

	// Filter that matches in either SUMMARY or DESCRIPTION
	err := engine.AddFilter([]string{"SUMMARY", "DESCRIPTION"}, "important")
	if err != nil {
		t.Fatalf("AddFilter() failed: %v", err)
	}

	cal := &parser.Calendar{
		Events: []*parser.Event{
			{UID: "1", Summary: "Regular Event", Description: "Normal"},
			{UID: "2", Summary: "important Meeting", Description: "Normal"},
			{UID: "3", Summary: "Regular Event", Description: "important details"},
			{UID: "4", Summary: "important", Description: "important"},
		},
	}

	filtered, matches := engine.Apply(cal)

	// Should remove events 2, 3, and 4 (all have "important" in SUMMARY or DESCRIPTION)
	if len(filtered.Events) != 1 {
		t.Errorf("Expected 1 event after filtering, got %d", len(filtered.Events))
	}

	if filtered.Events[0].UID != "1" {
		t.Errorf("Expected event 1, got event %s", filtered.Events[0].UID)
	}

	if len(matches) != 3 {
		t.Errorf("Expected 3 matches, got %d", len(matches))
	}
}

// TestGetStats tests statistics calculation
// Validates: Correct counting of events
func TestGetStats(t *testing.T) {
	original := &parser.Calendar{
		Events: []*parser.Event{
			{UID: "1"},
			{UID: "2"},
			{UID: "3"},
			{UID: "4"},
			{UID: "5"},
		},
	}

	filtered := &parser.Calendar{
		Events: []*parser.Event{
			{UID: "1"},
			{UID: "3"},
		},
	}

	stats := GetStats(original, filtered)

	if stats.TotalEvents != 5 {
		t.Errorf("TotalEvents = %d, want 5", stats.TotalEvents)
	}
	if stats.FilteredEvents != 2 {
		t.Errorf("FilteredEvents = %d, want 2", stats.FilteredEvents)
	}
	if stats.RemovedEvents != 3 {
		t.Errorf("RemovedEvents = %d, want 3", stats.RemovedEvents)
	}
}
