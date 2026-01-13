# Task 1: Improve Configuration Page - Direct Open Links

## Status
✅ **COMPLETE** - Fully implemented

## Implementation Details

All features have been implemented in [internal/server/ui.go](../../internal/server/ui.go):
- ✅ Preview/Debug button with bidirectional navigation (lines 202-204, 347-352)
- ✅ Platform detection for calendar apps (lines 354-365)
- ✅ Apple Calendar integration (lines 385-393)
- ✅ Google Calendar integration (lines 396-403)
- ✅ Outlook.com integration (lines 406-413)
- ✅ Generic webcal:// link with clipboard support (lines 416-427)
- ✅ Back link on preview page (server.go:778)

## Overview
Add direct links from the configuration page to open the configured feed in various calendar applications and debug mode.

## Current State
- Configuration page at `/` allows users to build filter queries
- Users can copy URL or download iCal file
- No direct integration with calendar applications
- No quick link to debug mode

## Goals
1. **Bidirectional Debug Navigation**
   - Add link on config page to open current configuration in debug mode
   - Add link on debug page to return to config page with same parameters

2. **Calendar Application Integration**
   - Add "Open in..." buttons for popular calendar applications
   - Support platform-specific apps (macOS/iOS/Windows/Android)
   - Provide fallback for unsupported platforms

## Requirements

### Functional Requirements

**FR1: Debug Mode Links**
- [ ] Config page (`/`) must show "View in Debug Mode" button
- [ ] Debug page (`/debug` (redirects to `/query/preview`)) must show "Edit Configuration" button
- [ ] Both links preserve all current filter parameters
- [ ] Links are only shown when filters are active

**FR2: Calendar Application Links**
- [ ] Support "Open in Apple Calendar" (macOS/iOS)
  - Uses `webcal://` protocol
  - Subscribes to filtered feed URL
- [ ] Support "Open in Outlook"
  - Uses `outlook://` protocol (desktop)
  - Uses web link for Outlook.com
- [ ] Support "Add to Google Calendar"
  - Uses Google Calendar subscription URL format
  - Opens in new tab
- [ ] Support "Other Calendar Apps"
  - Generic `webcal://` link
  - Copy-to-clipboard functionality

**FR3: Platform Detection**
- [ ] Detect user's platform (macOS, iOS, Windows, Android, Other)
- [ ] Show relevant calendar apps for detected platform
- [ ] Provide "Show all options" to display all integrations

### Non-Functional Requirements

**NFR1: User Experience**
- Links must be clearly labeled and discoverable
- Buttons should use recognizable icons for each app
- Provide helpful tooltips explaining what each link does
- Mobile-friendly button sizing and layout

**NFR2: Security**
- All generated URLs must be properly encoded
- Validate that subscription URLs are safe
- Use HTTPS for webcal subscions (converts to webcals://)

**NFR3: Compatibility**
- Must work on all major browsers
- Gracefully handle unsupported calendar apps
- Provide clear feedback if action cannot be completed

## Technical Design

### UI Components

**Config Page Updates:**
```
┌─────────────────────────────────────────┐
│ ReCal - Configure Your Calendar Filter  │
├─────────────────────────────────────────┤
│                                         │
│ [Existing filter controls]             │
│                                         │
├─────────────────────────────────────────┤
│ Actions:                                │
│ ┌────────┐ ┌────────┐ ┌────────┐      │
│ │ Copy   │ │Download│ │ Debug  │      │
│ │  URL   │ │  iCal  │ │  Mode  │      │
│ └────────┘ └────────┘ └────────┘      │
│                                         │
│ Open in:                                │
│ ┌────────┐ ┌────────┐ ┌────────┐      │
│ │  📅    │ │  📧    │ │  🌐    │      │
│ │ Apple  │ │Outlook │ │ Google │      │
│ └────────┘ └────────┘ └────────┘      │
│ ┌────────────────┐                     │
│ │ Other Apps...  │                     │
│ └────────────────┘                     │
└─────────────────────────────────────────┘
```

**Debug Page Updates:**
```
┌─────────────────────────────────────────┐
│ ReCal Debug Report                      │
├─────────────────────────────────────────┤
│ ← Back to Configuration                 │
│                                         │
│ [Existing debug content]                │
└─────────────────────────────────────────┘
```

### URL Formats

**Apple Calendar (webcal):**
```
webcals://pb.thorsell.info/query?Grad=3&Loge=Göta
```

**Outlook Desktop:**
```
outlook://subscribe?url=https://pb.thorsell.info/query?Grad=3&Loge=Göta
```

**Google Calendar:**
```
https://calendar.google.com/calendar/render?cid=https://pb.thorsell.info/query?Grad=3&Loge=Göta
```

**Generic webcal:**
```
webcal://pb.thorsell.info/query?Grad=3&Loge=Göta
```

### Platform Detection

Use JavaScript to detect platform:
```javascript
const platform = {
  isMac: /Mac/.test(navigator.platform),
  isIOS: /iPhone|iPad|iPod/.test(navigator.platform),
  isWindows: /Win/.test(navigator.platform),
  isAndroid: /Android/.test(navigator.userAgent)
};
```

## Implementation Files

**Files to Modify:**
- `internal/server/server.go` - Update ConfigPage handler
- `internal/server/ui.go` - Add new UI components (if exists)
- `internal/server/templates/` - Update HTML templates

**New Files to Create:**
- None (all changes in existing files)

## Testing Requirements

**Test Cases:**
- [ ] Verify debug link appears on config page with filters
- [ ] Verify debug link preserves all parameters
- [ ] Verify config link appears on debug page
- [ ] Verify config link preserves all parameters
- [ ] Test webcal:// links on macOS
- [ ] Test outlook:// links on Windows
- [ ] Test Google Calendar links
- [ ] Verify URLs are properly encoded
- [ ] Test on mobile browsers
- [ ] Test platform detection logic

## Dependencies
- None - uses existing infrastructure

## Estimated Effort
- **Design/Planning:** 1 hour
- **Implementation:** 3-4 hours
- **Testing:** 1-2 hours
- **Total:** ~6 hours
