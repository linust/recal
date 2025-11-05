package parser

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestParse tests parsing a valid iCal feed
// Validates: iCal parsing, event extraction, field mapping
func TestParse(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//EN
BEGIN:VEVENT
UID:test1@example.com
DTSTART:20250115T180000Z
DTEND:20250115T190000Z
SUMMARY:Test Event
DESCRIPTION:Test Description
LOCATION:Test Location
STATUS:CONFIRMED
END:VEVENT
END:VCALENDAR`

	r := strings.NewReader(icalData)
	cal, err := Parse(r)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	if len(cal.Events) != 1 {
		t.Fatalf("Parse() got %d events, want 1", len(cal.Events))
	}

	event := cal.Events[0]
	if event.UID != "test1@example.com" {
		t.Errorf("Event.UID = %q, want test1@example.com", event.UID)
	}
	if event.Summary != "Test Event" {
		t.Errorf("Event.Summary = %q, want 'Test Event'", event.Summary)
	}
	if event.Description != "Test Description" {
		t.Errorf("Event.Description = %q, want 'Test Description'", event.Description)
	}
	if event.Location != "Test Location" {
		t.Errorf("Event.Location = %q, want 'Test Location'", event.Location)
	}
	if event.Status != "CONFIRMED" {
		t.Errorf("Event.Status = %q, want CONFIRMED", event.Status)
	}
}

// TestParseMultipleEvents tests parsing multiple events
// Validates: Multiple event extraction, ordering
func TestParseMultipleEvents(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//EN
BEGIN:VEVENT
UID:event1@example.com
SUMMARY:First Event
END:VEVENT
BEGIN:VEVENT
UID:event2@example.com
SUMMARY:Second Event
END:VEVENT
BEGIN:VEVENT
UID:event3@example.com
SUMMARY:Third Event
END:VEVENT
END:VCALENDAR`

	r := strings.NewReader(icalData)
	cal, err := Parse(r)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	if len(cal.Events) != 3 {
		t.Fatalf("Parse() got %d events, want 3", len(cal.Events))
	}

	expectedSummaries := []string{"First Event", "Second Event", "Third Event"}
	for i, event := range cal.Events {
		if event.Summary != expectedSummaries[i] {
			t.Errorf("Event[%d].Summary = %q, want %q", i, event.Summary, expectedSummaries[i])
		}
	}
}

// TestGetField tests the GetField method
// Validates: Field access, case-insensitivity, unknown fields
func TestGetField(t *testing.T) {
	event := &Event{
		UID:         "test@example.com",
		Summary:     "Test Summary",
		Description: "Test Description",
		Location:    "Test Location",
		Status:      "CONFIRMED",
		DTStart:     "20250115T180000Z",
		DTEnd:       "20250115T190000Z",
	}

	tests := []struct {
		field string
		want  string
	}{
		{"UID", "test@example.com"},
		{"uid", "test@example.com"},
		{"SUMMARY", "Test Summary"},
		{"summary", "Test Summary"},
		{"Summary", "Test Summary"},
		{"DESCRIPTION", "Test Description"},
		{"LOCATION", "Test Location"},
		{"STATUS", "CONFIRMED"},
		{"DTSTART", "20250115T180000Z"},
		{"DTEND", "20250115T190000Z"},
		{"UNKNOWN", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := event.GetField(tt.field)
		if got != tt.want {
			t.Errorf("GetField(%q) = %q, want %q", tt.field, got, tt.want)
		}
	}
}

// TestSerialize tests serializing a calendar back to iCal format
// Validates: iCal generation, round-trip parsing
func TestSerialize(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//EN
BEGIN:VEVENT
UID:test1@example.com
DTSTAMP:20250115T120000Z
DTSTART:20250115T180000Z
DTEND:20250115T190000Z
SUMMARY:Test Event
DESCRIPTION:Test Description
LOCATION:Test Location
STATUS:CONFIRMED
END:VEVENT
END:VCALENDAR`

	// Parse the calendar
	r := strings.NewReader(icalData)
	cal, err := Parse(r)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	// Serialize it back
	var buf bytes.Buffer
	if err := cal.Serialize(&buf); err != nil {
		t.Fatalf("Serialize() failed: %v", err)
	}

	// Parse the serialized version
	r2 := bytes.NewReader(buf.Bytes())
	cal2, err := Parse(r2)
	if err != nil {
		t.Fatalf("Parse() of serialized calendar failed: %v", err)
	}

	// Compare events
	if len(cal2.Events) != len(cal.Events) {
		t.Fatalf("Serialized calendar has %d events, want %d", len(cal2.Events), len(cal.Events))
	}

	for i := range cal.Events {
		if cal2.Events[i].UID != cal.Events[i].UID {
			t.Errorf("Event[%d].UID = %q, want %q", i, cal2.Events[i].UID, cal.Events[i].UID)
		}
		if cal2.Events[i].Summary != cal.Events[i].Summary {
			t.Errorf("Event[%d].Summary = %q, want %q", i, cal2.Events[i].Summary, cal.Events[i].Summary)
		}
	}
}

// TestParseTestDataFile tests parsing the sample test data file
// Validates: Real iCal file parsing, multiple events with various properties
func TestParseTestDataFile(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sample-feed.ics")
	if err != nil {
		t.Skipf("Skipping test: testdata file not found: %v", err)
	}

	r := bytes.NewReader(data)
	cal, err := Parse(r)
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	// The sample feed should have 8 events (real data from upstream)
	if len(cal.Events) != 8 {
		t.Fatalf("Parse() got %d events, want 8", len(cal.Events))
	}

	// Check specific events (using real UIDs and summaries from the feed)
	tests := []struct {
		index   int
		uid     string
		summary string
		status  string
	}{
		{0, "62ehjthta9qp3r3k7jluuncigk@google.com", "Göta PB: Grad 4", "CONFIRMED"},
		{1, "2mut3o6sv9ij5b07p409kjlbfo@google.com", "Göta PB: Grad 7", "CONFIRMED"},
		{2, "0d1rupmjvrgi4iquckjq1ogqls@google.com", "Borås PB: Grad 7", "CONFIRMED"},
		{5, "abvciphn179pu3rgb69rve4hvc@google.com", "INSTÄLLT: Borås PB: Grad 10", "TENTATIVE"},
		{6, "dde7tr19mt8nlbt2dr3qcgtcdo@google.com", "INSTÄLLT: Göta PB: Grad 1", "TENTATIVE"},
		{7, "rni3kof0rjg9tk6g8gie1tf148@google.com", "Vänersborg PB: Stora Rådet", "CONFIRMED"},
	}

	for _, tt := range tests {
		if tt.index >= len(cal.Events) {
			t.Errorf("Event[%d] not found", tt.index)
			continue
		}
		event := cal.Events[tt.index]
		if event.UID != tt.uid {
			t.Errorf("Event[%d].UID = %q, want %q", tt.index, event.UID, tt.uid)
		}
		if event.Summary != tt.summary {
			t.Errorf("Event[%d].Summary = %q, want %q", tt.index, event.Summary, tt.summary)
		}
		if event.Status != tt.status {
			t.Errorf("Event[%d].Status = %q, want %q", tt.index, event.Status, tt.status)
		}
	}
}

// TestParseInvalidICal tests parsing invalid iCal data
// Validates: Error handling for malformed input
func TestParseInvalidICal(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "missing END:VCALENDAR",
			data: `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test@example.com
SUMMARY:Test
END:VEVENT`,
		},
		{
			name: "empty input",
			data: "",
		},
		{
			name: "not iCal format",
			data: "This is not iCal data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.data)
			_, err := Parse(r)
			if err == nil {
				t.Errorf("Parse() succeeded for invalid data %q, want error", tt.name)
			}
		})
	}
}

// TestSerializeEmptyCalendar tests serializing an empty calendar
// Validates: Handling of calendar with no events (should fail per RFC 5545)
func TestSerializeEmptyCalendar(t *testing.T) {
	cal := &Calendar{
		Events: []*Event{},
	}

	var buf bytes.Buffer
	err := cal.Serialize(&buf)
	// The iCal library should reject empty calendars as they're not valid per RFC 5545
	// A calendar must have at least one component
	if err == nil {
		t.Error("Serialize() succeeded for empty calendar, want error (RFC 5545 requires at least one component)")
	}
}
