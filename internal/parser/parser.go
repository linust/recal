package parser

import (
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-ical"
)

// Event represents a parsed iCal event with relevant fields
type Event struct {
	UID         string
	Summary     string
	Description string
	Location    string
	Status      string
	DTStart     string
	DTEnd       string
	RawEvent    *ical.Event // Keep the raw event for full iCal generation
}

// Calendar represents a parsed iCal calendar
type Calendar struct {
	Events []*Event
	Raw    *ical.Calendar // Keep the raw calendar for metadata
}

// Parse parses an iCal feed from a reader
func Parse(r io.Reader) (*Calendar, error) {
	decoder := ical.NewDecoder(r)

	var calendar *ical.Calendar
	var err error

	// Decode the calendar
	calendar, err = decoder.Decode()
	if err != nil {
		return nil, fmt.Errorf("failed to decode iCal: %w", err)
	}

	// Extract events
	var events []*Event
	for _, component := range calendar.Children {
		if component.Name == ical.CompEvent {
			event, err := parseEvent(component)
			if err != nil {
				// Log the error but continue processing other events
				continue
			}
			events = append(events, event)
		}
	}

	return &Calendar{
		Events: events,
		Raw:    calendar,
	}, nil
}

// parseEvent converts an ical.Component to our Event struct
func parseEvent(component *ical.Component) (*Event, error) {
	event := &Event{
		RawEvent: ical.NewEvent(),
	}

	// Copy the entire component to our raw event
	event.RawEvent.Component = component

	// Extract commonly used fields using the Props helper
	if prop := component.Props.Get(ical.PropUID); prop != nil {
		event.UID = prop.Value
	}
	if prop := component.Props.Get(ical.PropSummary); prop != nil {
		event.Summary = prop.Value
	}
	if prop := component.Props.Get(ical.PropDescription); prop != nil {
		event.Description = prop.Value
	}
	if prop := component.Props.Get(ical.PropLocation); prop != nil {
		event.Location = prop.Value
	}
	if prop := component.Props.Get(ical.PropStatus); prop != nil {
		event.Status = prop.Value
	}
	if prop := component.Props.Get(ical.PropDateTimeStart); prop != nil {
		event.DTStart = prop.Value
	}
	if prop := component.Props.Get(ical.PropDateTimeEnd); prop != nil {
		event.DTEnd = prop.Value
	}

	return event, nil
}

// GetField returns the value of a field by name
// Field names are case-insensitive: SUMMARY, summary, Summary all work
func (e *Event) GetField(fieldName string) string {
	fieldName = strings.ToUpper(fieldName)

	switch fieldName {
	case "UID":
		return e.UID
	case "SUMMARY":
		return e.Summary
	case "DESCRIPTION":
		return e.Description
	case "LOCATION":
		return e.Location
	case "STATUS":
		return e.Status
	case "DTSTART":
		return e.DTStart
	case "DTEND":
		return e.DTEnd
	default:
		return ""
	}
}

// Serialize converts a Calendar back to iCal format
func (c *Calendar) Serialize(w io.Writer) error {
	// Create a new calendar with the same properties as the original
	outCal := ical.NewCalendar()

	// Copy calendar-level properties from the raw calendar
	if c.Raw != nil {
		// Copy all properties from the original calendar
		outCal.Props = c.Raw.Props
	} else {
		// Set default properties if we don't have the raw calendar
		outCal.Props.SetText(ical.PropVersion, "2.0")
		outCal.Props.SetText(ical.PropProductID, "-//iCal Filter//EN")
	}

	// Add all events
	for _, event := range c.Events {
		if event.RawEvent != nil && event.RawEvent.Component != nil {
			outCal.Children = append(outCal.Children, event.RawEvent.Component)
		}
	}

	// Encode to writer
	encoder := ical.NewEncoder(w)
	if err := encoder.Encode(outCal); err != nil {
		return fmt.Errorf("failed to encode iCal: %w", err)
	}

	return nil
}
