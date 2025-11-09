# Task Specifications

This document contains detailed specifications for upcoming development tasks.

---

## Task 1: Improve Configuration Page - Direct Open Links

### Overview
Add direct links from the configuration page to open the configured feed in various calendar applications and debug mode.

### Current State
- Configuration page at `/` allows users to build filter queries
- Users can copy URL or download iCal file
- No direct integration with calendar applications
- No quick link to debug mode

### Goals
1. **Bidirectional Debug Navigation**
   - Add link on config page to open current configuration in debug mode
   - Add link on debug page to return to config page with same parameters

2. **Calendar Application Integration**
   - Add "Open in..." buttons for popular calendar applications
   - Support platform-specific apps (macOS/iOS/Windows/Android)
   - Provide fallback for unsupported platforms

### Requirements

#### Functional Requirements

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

#### Non-Functional Requirements

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

### Technical Design

#### UI Components

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

#### URL Formats

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

#### Platform Detection

Use JavaScript to detect platform:
```javascript
const platform = {
  isMac: /Mac/.test(navigator.platform),
  isIOS: /iPhone|iPad|iPod/.test(navigator.platform),
  isWindows: /Win/.test(navigator.platform),
  isAndroid: /Android/.test(navigator.userAgent)
};
```

### Implementation Files

**Files to Modify:**
- `internal/server/server.go` - Update ConfigPage handler
- `internal/server/ui.go` - Add new UI components (if exists)
- `internal/server/templates/` - Update HTML templates

**New Files to Create:**
- None (all changes in existing files)

### Testing Requirements

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

### Dependencies
- None - uses existing infrastructure

### Estimated Effort
- **Design/Planning:** 1 hour
- **Implementation:** 3-4 hours
- **Testing:** 1-2 hours
- **Total:** ~6 hours

---

## Task 2: Named Feeds (Persistent Filter Configurations)

### Overview
Allow users to save filter configurations as named feeds with persistent slugs, enabling feed updates without requiring calendar reconfiguration.

### Current State
- Filters are defined via URL query parameters
- URL must be updated in calendar apps to change filters
- No way to save/name configurations
- No persistence layer

### Goals
1. **Create Named Feeds**
   - User provides description for filter configuration
   - System generates unique slug (UUID)
   - Feed becomes accessible via slug URL

2. **Manage Named Feeds**
   - Update feed configuration without changing slug
   - Update description
   - Delete feeds
   - View all created feeds

3. **Maintain Compatibility**
   - Direct query parameter URLs continue to work
   - Named feeds are optional enhancement
   - No breaking changes

### Requirements

#### Functional Requirements

**FR1: Feed Creation**
- [ ] User can save current filter configuration as named feed
- [ ] System generates unique slug (UUID v4)
- [ ] User provides human-readable description
- [ ] Description is stored with configuration
- [ ] Confirmation page shows slug URL

**FR2: Feed Access**
- [ ] Named feed URL: `/slug/{uuid}`
- [ ] Serves filtered iCal based on saved configuration
- [ ] Returns 404 if slug not found
- [ ] Config page: `/slug/{uuid}/config`
- [ ] Debug page: `/slug/{uuid}/debug`

**FR3: Feed Management**
- [ ] View all created feeds (list page)
- [ ] Update feed description
- [ ] Update feed filters
- [ ] Delete feed
- [ ] View feed statistics (access count, last access)

**FR4: Backwards Compatibility**
- [ ] Direct URLs (`/query?param=value`) continue working
- [ ] Named feeds are additive, not replacement
- [ ] Old integrations unaffected

#### Non-Functional Requirements

**NFR1: Performance**
- Feed lookup by slug must be fast (< 10ms)
- Use in-memory cache for active feeds
- Persist to disk/database
- Handle 100+ named feeds efficiently

**NFR2: Security**
- See dedicated Security Requirements section below

**NFR3: Reliability**
- Feed data must persist across restarts
- Backup/restore capability
- Atomic updates (no partial state)

**NFR4: Scalability**
- Support multiple users creating feeds
- Consider multi-tenancy (future)
- Clean up unused/old feeds (retention policy)

### Security Requirements

#### Security Model

**Principle**: UUID-based access control with upstream-protected admin endpoints

The security model uses a two-tier approach:
1. **Feed Access (Public)**: Protected by UUID secrecy - knowing the UUID grants access
2. **Admin Operations (Protected)**: Confined to `/admin/*` path prefix for upstream authentication

#### Security Requirements

**SR1: Feed Access Protection**
- [ ] Feed UUIDs must be generated using cryptographically secure random (UUID v4)
- [ ] UUID length: 36 characters (standard UUID format: `xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx`)
- [ ] Feed access via `/slug/{uuid}` requires knowing the full UUID
- [ ] Invalid/unknown UUIDs return HTTP 404 (not 403, to avoid enumeration hints)
- [ ] No feed listing available without authentication
- [ ] UUIDs are URL-safe and do not require encoding

**SR2: Admin Endpoint Protection**
- [ ] All administrative operations confined to `/admin/*` path prefix
- [ ] Admin endpoints include:
  - `GET /admin/feeds` - List all feeds
  - `POST /admin/feeds` - Create new feed
  - `PUT /admin/feeds/{uuid}` - Update feed
  - `DELETE /admin/feeds/{uuid}` - Delete feed
  - `GET /admin/feeds/{uuid}/stats` - View feed statistics
- [ ] Admin endpoints return HTTP 401 if no upstream authentication
- [ ] No authorization logic in ReCal service itself
- [ ] Upstream reverse proxy (nginx, Caddy, etc.) handles authentication

**SR3: Rate Limiting**
- [ ] Feed creation limited to prevent abuse
- [ ] Recommended: 10 feed creates per hour per IP
- [ ] Rate limiting implemented at reverse proxy level (not in ReCal)
- [ ] ReCal logs all admin operations for audit trail

**SR4: Input Validation**
- [ ] Feed descriptions limited to 200 characters
- [ ] Filter parameters validated (same validation as query params)
- [ ] UUID format strictly validated (reject malformed UUIDs)
- [ ] Reject requests with oversized JSON payloads (max 10KB)

**SR5: Enumeration Prevention**
- [ ] No feed listing without authentication
- [ ] Feed access errors return generic 404 (never reveal if UUID exists)
- [ ] No timing attacks: constant-time UUID lookup where feasible
- [ ] Admin endpoints don't leak UUIDs in error messages to unauthenticated users

**SR6: Data Integrity**
- [ ] Feed updates are atomic (lock during write)
- [ ] Concurrent access handled safely with mutexes
- [ ] Feed deletion marks as deleted first, then removes (no orphaned refs)
- [ ] File-based storage uses atomic writes (write to temp, then rename)

#### Recommended Reverse Proxy Configuration

**Nginx Example:**
```nginx
# Public feed access - no auth required
location /slug/ {
    proxy_pass http://localhost:8080;
}

# Admin operations - require HTTP Basic Auth
location /admin/ {
    auth_basic "ReCal Admin";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://localhost:8080;
}

# Rate limiting for admin endpoints
limit_req_zone $binary_remote_addr zone=admin:10m rate=10r/h;
location /admin/feeds {
    limit_req zone=admin burst=5;
    auth_basic "ReCal Admin";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://localhost:8080;
}
```

**Caddy Example:**
```caddyfile
# Public feed access
handle /slug/* {
    reverse_proxy localhost:8080
}

# Admin operations with basic auth
handle /admin/* {
    basicauth {
        admin $2a$14$...  # bcrypt hash
    }
    reverse_proxy localhost:8080
}
```

#### Security Documentation

The service will include documentation explaining:
1. UUID-based access is "security by obscurity" - treat UUIDs as secrets
2. Share feed URLs only with intended recipients
3. Admin endpoints MUST be protected by upstream authentication
4. Recommended: Use HTTPS for all traffic (webcals:// for calendar subscriptions)
5. Feed rotation: Delete and recreate if UUID is compromised

### Technical Design

#### Data Model

**Feed Structure:**
```go
type NamedFeed struct {
    Slug        string                 // UUID v4
    Description string                 // User-provided description
    CreatedAt   time.Time             // Creation timestamp
    UpdatedAt   time.Time             // Last update timestamp
    Filters     map[string][]string   // Filter parameters
    AccessCount int64                 // Number of times accessed
    LastAccess  time.Time             // Last access timestamp
    Owner       string                // Optional: user identifier
}
```

**Storage Options:**

1. **File-based (Simple, Initial Implementation)**
   ```
   data/feeds/
     ├── {uuid1}.json
     ├── {uuid2}.json
     └── index.json  # List of all feeds
   ```

2. **SQLite (Future Enhancement)**
   ```sql
   CREATE TABLE named_feeds (
       slug TEXT PRIMARY KEY,
       description TEXT NOT NULL,
       created_at TIMESTAMP NOT NULL,
       updated_at TIMESTAMP NOT NULL,
       filters JSON NOT NULL,
       access_count INTEGER DEFAULT 0,
       last_access TIMESTAMP,
       owner TEXT
   );
   CREATE INDEX idx_updated_at ON named_feeds(updated_at);
   ```

3. **In-Memory Cache**
   ```go
   type FeedCache struct {
       mu    sync.RWMutex
       feeds map[string]*NamedFeed
       store FeedStore  // Interface for persistence
   }
   ```

#### API Endpoints

**Public Endpoints (No Authentication Required):**

1. **Get Feed (iCal)**
   ```
   GET /slug/{uuid}

   Response 200 OK:
   Content-Type: text/calendar
   Cache-Control: public, max-age=900
   [iCal data based on saved filters]

   Response 404 Not Found:
   Feed not found
   ```

2. **Get Feed Config Page**
   ```
   GET /slug/{uuid}/config

   Response 200 OK:
   Content-Type: text/html
   [Configuration page with saved filters pre-loaded]
   Shows: Description, current filters, ability to save changes (creates new feed)
   ```

3. **Get Feed Debug Page**
   ```
   GET /slug/{uuid}/debug

   Response 200 OK:
   Content-Type: text/html
   [Debug page showing filter statistics for this feed]
   ```

**Admin Endpoints (Require Upstream Authentication):**

4. **Create Feed**
   ```
   POST /admin/feeds
   Content-Type: application/json

   {
     "description": "Göta Grad 1-3",
     "filters": {
       "Grad": ["3"],
       "Loge": ["Göta"]
     }
   }

   Response 201 Created:
   {
     "slug": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
     "description": "Göta Grad 1-3",
     "url": "https://pb.thorsell.info/slug/a1b2c3d4-e5f6-7890-abcd-ef1234567890",
     "config_url": "https://pb.thorsell.info/slug/a1b2c3d4-e5f6-7890-abcd-ef1234567890/config",
     "created_at": "2025-11-07T12:34:56Z"
   }

   Response 400 Bad Request:
   Invalid filter parameters or description too long

   Response 401 Unauthorized:
   Authentication required (returned by upstream proxy)
   ```

5. **List All Feeds**
   ```
   GET /admin/feeds

   Response 200 OK:
   {
     "feeds": [
       {
         "slug": "a1b2c3d4-...",
         "description": "Göta Grad 1-3",
         "created_at": "2025-11-07T12:34:56Z",
         "updated_at": "2025-11-07T14:22:10Z",
         "access_count": 42,
         "last_access": "2025-11-08T08:15:30Z"
       }
     ],
     "total": 1
   }

   Response 401 Unauthorized:
   Authentication required
   ```

6. **Get Feed Details**
   ```
   GET /admin/feeds/{uuid}

   Response 200 OK:
   {
     "slug": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
     "description": "Göta Grad 1-3",
     "created_at": "2025-11-07T12:34:56Z",
     "updated_at": "2025-11-07T14:22:10Z",
     "filters": {
       "Grad": ["3"],
       "Loge": ["Göta"]
     },
     "access_count": 42,
     "last_access": "2025-11-08T08:15:30Z"
   }

   Response 404 Not Found:
   Feed not found
   ```

7. **Update Feed**
   ```
   PUT /admin/feeds/{uuid}
   Content-Type: application/json

   {
     "description": "Updated description",
     "filters": {
       "Grad": ["4"],
       "Loge": ["Göta"]
     }
   }

   Response 200 OK:
   {
     "slug": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
     "description": "Updated description",
     "updated_at": "2025-11-08T10:45:20Z"
   }

   Response 404 Not Found:
   Feed not found

   Response 400 Bad Request:
   Invalid parameters
   ```

8. **Delete Feed**
   ```
   DELETE /admin/feeds/{uuid}

   Response 204 No Content

   Response 404 Not Found:
   Feed not found
   ```

9. **Get Feed Statistics**
   ```
   GET /admin/feeds/{uuid}/stats

   Response 200 OK:
   {
     "slug": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
     "access_count": 42,
     "last_access": "2025-11-08T08:15:30Z",
     "created_at": "2025-11-07T12:34:56Z",
     "age_days": 1
   }
   ```

#### UI Flow

**Creating a Named Feed:**

1. User configures filters on main page
2. Clicks "Save as Named Feed" button
3. Modal/dialog appears asking for description
4. User enters description, clicks "Create"
5. System generates slug and saves
6. Confirmation page shows:
   - Slug URL
   - Config URL
   - Buttons to open in calendar apps
   - QR code (optional)

**Managing Named Feeds:**

1. New page: `/feeds` - Lists all created feeds
2. Each feed shows:
   - Description
   - Slug (copyable)
   - Access count
   - Last accessed
   - Edit/Delete buttons
3. Edit button → Config page with saved filters
4. Delete button → Confirmation dialog → Delete

#### Configuration

**Add to config.yaml:**
```yaml
feeds:
  storage_path: "./data/feeds"  # Where to store feed data
  max_feeds: 100                # Maximum feeds per instance
  slug_length: 36               # UUID v4 length
  retention_days: 365           # Auto-delete after N days of inactivity
```

### Implementation Files

**Files to Create:**
- `internal/feeds/feed.go` - Feed data structure
- `internal/feeds/store.go` - Storage interface and file-based implementation
- `internal/feeds/cache.go` - In-memory cache
- `internal/feeds/manager.go` - High-level feed management
- `internal/server/feeds_handlers.go` - HTTP handlers for feed APIs
- `data/feeds/` - Directory for feed storage

**Files to Modify:**
- `internal/config/config.go` - Add feeds configuration
- `internal/server/server.go` - Register new routes
- `config.yaml` - Add feeds configuration
- Root page (`/`) - Add "Save as Named Feed" button

### Testing Requirements

**Unit Tests:**
- [ ] Feed creation and validation
- [ ] UUID generation uniqueness
- [ ] File-based storage operations
- [ ] Cache operations (get, set, delete)
- [ ] Feed manager operations

**Integration Tests:**
- [ ] Create feed via API
- [ ] Access feed via slug URL
- [ ] Update feed configuration
- [ ] Delete feed
- [ ] List all feeds
- [ ] 404 for non-existent slug
- [ ] Persistence across restarts

**Performance Tests:**
- [ ] Benchmark feed lookup (target: < 10ms)
- [ ] Test with 100+ feeds
- [ ] Concurrent access to same feed

### Dependencies

**New Dependencies:**
- `github.com/google/uuid` - For UUID generation

**Configuration:**
- Storage directory must be writable
- File permissions for feed data

### Migration Path

**Phase 1: Basic Implementation (MVP)**
1. File-based storage
2. Create/Read/Delete operations
3. Basic UI for feed creation
4. No authentication

**Phase 2: Enhanced Features**
1. Update operations
2. Feed statistics
3. Management UI
4. Search/filter feeds

**Phase 3: Advanced Features**
1. SQLite storage option
2. Authentication/ownership
3. Feed sharing
4. API rate limiting
5. QR codes for mobile

### Estimated Effort

- **Design/Planning:** 2-3 hours
- **Data Model & Storage:** 4-6 hours
- **API Endpoints:** 4-6 hours
- **UI Implementation:** 6-8 hours
- **Testing:** 4-6 hours
- **Documentation:** 2 hours
- **Total:** ~25-30 hours

### Open Questions

1. **Authentication:** Do we need user accounts or is the slug security enough?
2. **Multi-tenancy:** Should different users have separate feed namespaces?
3. **Limits:** What limits on number of feeds, description length, etc.?
4. **Analytics:** What statistics should we track beyond access count?
5. **Export/Import:** Should users be able to export/import feed configurations?
6. **Versioning:** Should feeds have version history?

---

## Priority Order

**Recommended Implementation Order:**

1. **Task 3 (Clean Up Endpoints)** - API redesign, foundation for Tasks 1 & 2 (~5 hours)
2. **Task 4 (Normalize Lodge Names)** - Fix possessive variants confusion (~5 hours)
3. **Task 1 (Direct Open Links)** - Calendar app integration (~6 hours)
4. **Task 2 (Named Feeds)** - Persistent feed configurations (~25-30 hours)

**Rationale:**
- **Task 3 first**: Establishes endpoint structure that all other tasks will use
- **Task 4 second**: Fixes UX issue, small and independent, high user value
- **Task 1 third**: Immediate UX improvement, no persistence needed
- **Task 2 last**: Most complex, benefits from all previous tasks

**Task Dependencies:**
- Task 3: None (foundation for others)
- Task 4: None (independent improvement)
- Task 1: Benefits from Task 3's clean endpoints
- Task 2: Requires Task 3's endpoint structure, benefits from Task 1's UI patterns

**Alternative Order:** Tasks 3 and 4 can be done in parallel (no dependencies between them)

---

---

## Task 3: Clean Up Endpoints (API Redesign)

### Overview

Redesign endpoint structure to improve clarity, consistency, and firewall-based access control while maintaining RESTful principles where appropriate.

### Current State

Endpoints are functional but could be better organized:
- Mix of paths without clear pattern (`/`, `/query`, `/debug` (redirects to `/query/preview`))
- No clear namespace for future named feeds feature
- Difficult to apply path-based firewall rules for future admin features

### Goals

1. **Clear Path Structure**: Organize endpoints with intent-focused prefixes
2. **Firewall-Friendly**: Enable simple path-based access control
3. **RESTful Conventions**: Follow REST principles for API endpoints
4. **Future-Proof**: Support upcoming named feeds feature cleanly

### Design Principles

**Modified REST Approach:**
- Resource-oriented URLs (nouns, not verbs)
- HTTP methods convey action (GET/POST/PUT/DELETE)
- Path prefixes for access control (`/admin/*` for protected operations)
- Sub-resources for related functionality (`/feed/{uuid}/config`)
- Pragmatic: Accept both GET and POST for HTML forms where needed

**Path-based Access Control:**
```nginx
# One rule protects all admin operations
location /admin/ {
    auth_basic "Admin";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://localhost:8080;
}
```

### Current Endpoints (Before Task 3)

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/` | Configuration page UI | None |
| GET | `/query` | Filtered iCal feed (query params) | None |
| GET | `/debug` (redirects to `/query/preview`) | Debug mode (query params) | None |
| GET | `/status` | Server status page | None |
| GET | `/health` | Health check (JSON) | None |
| GET | `/api/lodges` | List available lodges (JSON) | None |

**Total:** 6 endpoints

### Proposed Endpoints (After Task 3)

#### Public Endpoints (No Authentication)

**Configuration & Static:**

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/` | Main configuration page UI | None |
| GET | `/status` | Server status page with metrics | None |
| GET | `/health` | Health check endpoint (JSON) | None |
| GET | `/api/lodges` | List available lodges (JSON) | None |

**Dynamic Query-Based Feeds:**

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/query` | Filtered iCal feed via query parameters | None |
| GET | `/query/preview` | Preview/debug mode for query-based filters | None |

**Named Feeds (Task 2):**

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/feed/{uuid}` | iCal feed for named feed | None* |
| GET | `/feed/{uuid}/config` | View/edit feed configuration (HTML form) | None* |
| POST | `/feed/{uuid}/config` | Update feed configuration (form submit) | None* |
| GET | `/feed/{uuid}/preview` | Preview/debug named feed | None* |

*UUID acts as access token - knowing UUID grants access

#### Admin Endpoints (Require Authentication)

**Feed Management:**

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | `/admin/feeds` | Create new named feed (JSON API) | Required |
| GET | `/admin/feeds` | List all named feeds (JSON API) | Required |
| GET | `/admin/feeds/{uuid}` | Get feed details (JSON API) | Required |
| DELETE | `/admin/feeds/{uuid}` | Delete named feed | Required |

**Total After Task 3:** 14 endpoints (6 current + 8 new)

### Endpoint Changes Summary

**Renamed/Reorganized:**
- `/debug` (redirects to `/query/preview`) → `/query/preview` (clearer intent, groups with `/query`)

**New Endpoints:**
- `/query/preview` - Debug mode for query-based filters
- `/feed/{uuid}` - Named feed iCal
- `/feed/{uuid}/config` (GET/POST) - Named feed configuration UI
- `/feed/{uuid}/preview` - Named feed debug mode
- `/admin/feeds` (POST/GET) - Admin feed management
- `/admin/feeds/{uuid}` (GET/DELETE) - Admin single feed operations

**Unchanged:**
- `/` - Config page
- `/query` - Query-based iCal
- `/status` - Status page
- `/health` - Health check
- `/api/lodges` - Lodge listing

### Migration Path

**Phase 1: Add new endpoints, keep old (backward compatible)**
1. Implement new `/query/preview` alongside `/debug` (redirects to `/query/preview`)
2. Both endpoints work identically
3. Update UI to link to `/query/preview`
4. Add deprecation notice to `/debug` (redirects to `/query/preview`)

**Phase 2: Implement Task 2 (Named Feeds)**
1. Add `/feed/{uuid}` endpoints
2. Add `/admin/feeds` endpoints
3. No breaking changes to existing functionality

**Phase 3: Deprecation (future)**
1. Mark `/debug` (redirects to `/query/preview`) as deprecated (respond with 301 redirect to `/query/preview`)
2. Update documentation
3. Monitor usage
4. Eventually remove (or keep redirecting permanently)

### Implementation Requirements

**IR1: Endpoint Routing**
- [ ] Add `/query/preview` handler (same as current `/debug` (redirects to `/query/preview`))
- [ ] Update internal links to use new paths
- [ ] Add 301 redirect from `/debug` (redirects to `/query/preview`) to `/query/preview` (for backward compatibility)

**IR2: Documentation Updates**
- [ ] Update README with new endpoint structure
- [ ] Update API documentation
- [ ] Add migration guide for existing users

**IR3: Testing**
- [ ] Test all existing endpoints still work
- [ ] Test new `/query/preview` endpoint
- [ ] Test redirect from `/debug` (redirects to `/query/preview`) to `/query/preview`
- [ ] Update integration tests

**IR4: UI Updates**
- [ ] Update "View in Debug Mode" button to link to `/query/preview`
- [ ] Update any hardcoded `/debug` (redirects to `/query/preview`) references

### Firewall Configuration Examples

**Nginx:**
```nginx
# Public endpoints - no authentication
location / {
    proxy_pass http://localhost:8080;
}

# Admin endpoints - require authentication
location /admin/ {
    auth_basic "ReCal Admin";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://localhost:8080;
}

# Optional: Rate limiting for admin
limit_req_zone $binary_remote_addr zone=admin:10m rate=10r/h;
location /admin/feeds {
    limit_req zone=admin burst=5;
    auth_basic "ReCal Admin";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://localhost:8080;
}
```

**Caddy:**
```caddyfile
# Public endpoints
handle /filter* {
    reverse_proxy localhost:8080
}

handle /feed/* {
    reverse_proxy localhost:8080
}

# Admin endpoints with authentication
handle /admin/* {
    basicauth {
        admin JDJhJDE0JC4uLg==  # bcrypt hash
    }
    reverse_proxy localhost:8080
}

# Everything else (/, /status, /health, /api/*)
handle {
    reverse_proxy localhost:8080
}
```

### Benefits

1. **Clearer Intent**: `/query/preview` makes purpose obvious
2. **Better Organization**: Related endpoints grouped by prefix
3. **Simple Firewall Rules**: Single rule protects all admin operations
4. **REST Compliance**: Standard HTTP methods for CRUD operations
5. **Scalable**: Easy to add more resources under `/admin/` or `/feed/`
6. **Backward Compatible**: Old `/debug` (redirects to `/query/preview`) endpoint redirects to new location

### Estimated Effort

- **Planning/Design:** 1 hour (completed)
- **Implementation:** 2-3 hours
  - Add `/query/preview` routing
  - Add redirect from `/debug` (redirects to `/query/preview`)
  - Update UI links
- **Testing:** 1 hour
- **Documentation:** 1 hour
- **Total:** ~5 hours

---

## Complete Endpoint Reference

This section lists ALL endpoints in the ReCal application - current and planned.

### Current Endpoints (Implemented)

**Public Endpoints:**

| Method | Path | Description | Authentication |
|--------|------|-------------|----------------|
| GET | `/` | Configuration page UI for building filter queries | None |
| GET | `/query` | Filtered iCal feed based on query parameters | None |
| GET | `/debug` (redirects to `/query/preview`) | Debug mode showing filter statistics (HTML) | None |
| GET | `/status` | Server status page with metrics and cache stats | None |
| GET | `/health` | Health check endpoint (JSON) | None |
| GET | `/api/lodges` | List of available lodges (JSON) | None |

### Task 1 Endpoints (Direct Open Links)

**No new endpoints** - Task 1 only adds UI enhancements to existing pages:
- Adds "View in Debug Mode" button on `/` (config page)
- Adds "Edit Configuration" button on `/debug` (redirects to `/query/preview`) page
- Adds "Open in Calendar App" buttons on `/` page
- All use existing `/query` and `/debug` (redirects to `/query/preview`) endpoints with query parameters

### Task 2 Endpoints (Named Feeds)

**Public Endpoints (No Authentication):**

| Method | Path | Description | Authentication |
|--------|------|-------------|----------------|
| GET | `/slug/{uuid}` | Get filtered iCal feed for named feed | None (UUID required) |
| GET | `/slug/{uuid}/config` | Configuration page pre-loaded with feed filters | None (UUID required) |
| GET | `/slug/{uuid}/debug` | Debug page for named feed | None (UUID required) |

**Admin Endpoints (Require Upstream Auth):**

| Method | Path | Description | Authentication |
|--------|------|-------------|----------------|
| POST | `/admin/feeds` | Create new named feed | Required |
| GET | `/admin/feeds` | List all named feeds | Required |
| GET | `/admin/feeds/{uuid}` | Get feed details | Required |
| PUT | `/admin/feeds/{uuid}` | Update feed description/filters | Required |
| DELETE | `/admin/feeds/{uuid}` | Delete named feed | Required |
| GET | `/admin/feeds/{uuid}/stats` | Get feed access statistics | Required |

### Complete Endpoint Summary (All Tasks)

**Total Endpoints After All Tasks:** 15

**By Authentication Requirement:**
- **Public (no auth):** 9 endpoints
  - Current: 6 (`/`, `/query`, `/debug` (redirects to `/query/preview`), `/status`, `/health`, `/api/lodges`)
  - Task 2: 3 (`/slug/{uuid}`, `/slug/{uuid}/config`, `/slug/{uuid}/debug`)
- **Admin (requires auth):** 6 endpoints
  - All from Task 2 under `/admin/*` prefix

**By HTTP Method:**
- GET: 13 endpoints
- POST: 1 endpoint (`/admin/feeds`)
- PUT: 1 endpoint (`/admin/feeds/{uuid}`)
- DELETE: 1 endpoint (`/admin/feeds/{uuid}`)

**By Content Type:**
- HTML: 6 endpoints (`/`, `/debug` (redirects to `/query/preview`), `/status`, `/slug/{uuid}/config`, `/slug/{uuid}/debug`, plus admin UI future)
- iCal: 2 endpoints (`/query`, `/slug/{uuid}`)
- JSON: 7 endpoints (`/health`, `/api/lodges`, all `/admin/*` endpoints)

**Path Prefixes for Upstream Protection:**

To protect admin endpoints with reverse proxy authentication, configure:

```nginx
# Protect everything under /admin/
location /admin/ {
    auth_basic "ReCal Admin";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://localhost:8080;
}

# Allow public access to everything else
location / {
    proxy_pass http://localhost:8080;
}
```

This clean separation ensures:
1. All administrative operations are under a single path prefix
2. Easy to protect with upstream authentication
3. No mixing of public and protected endpoints
4. Clear security boundary

---

## Task 4: Normalize Lodge Names (Handle Possessive Variants)

### Overview

Improve lodge filtering to handle possessive variations in lodge names. Currently, some lodges appear inconsistently with or without possessive 's' (e.g., "Sundsvall PB:" vs "Sundsvalls PB:"), requiring users to understand which variant exists in the upstream feed.

### Current State

**Problem:**
- Lodge names in upstream feed are inconsistent
- Example: "Sundsvall PB:" and "Sundsvalls PB:" both appear
- Users must know exact variant to filter correctly
- UI shows both "Sundsvall" and "Sundsvalls" as separate options (confusing)

**Current Behavior:**
```
Loge filter: "Sundsvall" → Only matches "Sundsvall PB:"
Loge filter: "Sundsvalls" → Only matches "Sundsvalls PB:"
```

**User Impact:**
- Confusion: Why are there two Sundsvall options?
- Missed events: Selecting "Sundsvall" misses "Sundsvalls PB:" events
- Poor UX: Duplicate/similar lodge names in checkbox list

### Goals

1. **Normalize Lodge Display**: Show single canonical name in UI (without possessive 's')
2. **Smart Matching**: Single selection matches all variants (with/without 's')
3. **Maintain Accuracy**: Don't over-normalize (e.g., don't match unrelated lodges)

### Requirements

**FR1: Lodge Name Normalization**
- [ ] Extract lodge names from upstream feed
- [ ] Normalize possessive variants to base form
- [ ] Rule: If lodge name ends with 's', create base form without 's'
- [ ] Store both canonical form and variants for matching

**FR2: UI Display**
- [ ] Show only canonical (base) form in lodge checkbox list
- [ ] Example: Show "Sundsvall" not both "Sundsvall" and "Sundsvalls"
- [ ] Maintain Swedish alphabetical sorting

**FR3: Filter Matching**
- [ ] When user selects "Sundsvall", match both variants:
  - "Sundsvall PB:"
  - "Sundsvalls PB:"
- [ ] Apply to all lodges, not just specific examples
- [ ] Case-insensitive matching for robustness

**FR4: Backward Compatibility**
- [ ] Existing filter URLs continue to work
- [ ] If URL contains `Loge=Sundsvalls`, normalize to base form
- [ ] No breaking changes to API

### Technical Design

#### Normalization Algorithm

**Step 1: Extract lodge variants from upstream**
```go
// Parse event: "Grad 4, Sundsvalls PB: Meeting"
// Extract: "Sundsvalls"
```

**Step 2: Normalize to canonical form**
```go
func normalizeLodgeName(name string) string {
    // Trim whitespace
    name = strings.TrimSpace(name)

    // If ends with 's', remove it for canonical form
    // But only if it's likely a possessive (length > 3 to avoid "As", "Os", etc.)
    if len(name) > 3 && strings.HasSuffix(strings.ToLower(name), "s") {
        return name[:len(name)-1]
    }

    return name
}

// Examples:
// "Sundsvalls" → "Sundsvall"
// "Göta" → "Göta" (no change)
// "Zions" → "Zion"
// "As" → "As" (too short, no change)
```

**Step 3: Build variant map**
```go
type LodgeVariants struct {
    Canonical string   // "Sundsvall"
    Variants  []string // ["Sundsvall", "Sundsvalls"]
}

// Build from upstream
variants := map[string]*LodgeVariants{
    "Sundsvall": {
        Canonical: "Sundsvall",
        Variants: ["Sundsvall", "Sundsvalls"],
    },
    "Göta": {
        Canonical: "Göta",
        Variants: ["Göta"],
    },
}
```

**Step 4: Return canonical list to UI**
```json
{
  "lodges": ["Göta", "Sundsvall", "Zion"]
}
```

**Step 5: Match against all variants**
```go
func matchesLodge(eventSummary string, selectedLodge string) bool {
    // Normalize user selection
    canonical := normalizeLodgeName(selectedLodge)

    // Get all variants for this canonical form
    variants := getLodgeVariants(canonical)

    // Check if event matches any variant
    for _, variant := range variants {
        pattern := variant + " PB:"
        if strings.Contains(eventSummary, pattern) {
            return true
        }
    }

    return false
}
```

#### Implementation Points

**Files to Modify:**

1. **`internal/server/server.go`** - `GetLodges()` function
   - Extract lodge names and variants
   - Normalize to canonical forms
   - Return deduplicated list

2. **`internal/filter/loge.go`** (or wherever loge filter lives)
   - Update matching logic to check all variants
   - Normalize filter parameter before matching

3. **Tests**
   - Unit tests for normalization logic
   - Test matching with possessive variants
   - Test edge cases (short names, special characters)

#### Edge Cases to Handle

**EC1: Short names**
```
"As" → "As" (don't remove 's', too short)
"Os" → "Os" (don't remove 's', too short)
```

**EC2: Non-possessive 's'**
```
"Norrlands" → "Norrland" (okay, possessive)
"Anders" → "Ander" (might be wrong, but acceptable)
```

**EC3: Multiple consecutive 's'**
```
"Sundsvalls" → "Sundsvall" (remove one 's')
```

**EC4: Case variations**
```
"SUNDSVALLS PB:" → matches "Sundsvall"
"sundsvalls pb:" → matches "Sundsvall"
```

#### Alternative Approaches Considered

**Option A: Fuzzy matching (rejected)**
- Too complex, may match unintended lodges
- Hard to explain to users

**Option B: Allow both in UI with grouping (rejected)**
- Still confusing
- Doesn't solve the selection problem

**Option C: Manual mapping (rejected)**
- Requires maintenance
- Brittle when new lodges added

**Option D: Suffix removal (selected)**
- Simple, predictable rule
- Automatic, no maintenance
- Works for Swedish possessive pattern

### Testing Requirements

**TC1: Normalization**
- [ ] "Sundsvalls" normalizes to "Sundsvall"
- [ ] "Göta" remains "Göta"
- [ ] "Zions" normalizes to "Zion"
- [ ] "As" remains "As" (too short)

**TC2: Variant Detection**
- [ ] Upstream with "Sundsvall PB:" and "Sundsvalls PB:" creates single entry
- [ ] Both variants stored in variant map

**TC3: Filter Matching**
- [ ] Filter `Loge=Sundsvall` matches "Sundsvall PB:" events
- [ ] Filter `Loge=Sundsvall` matches "Sundsvalls PB:" events
- [ ] Filter `Loge=Göta` only matches "Göta PB:" (no variants)

**TC4: UI Display**
- [ ] `/api/lodges` returns deduplicated canonical names
- [ ] No duplicate/variant entries in response
- [ ] Proper Swedish sorting maintained

**TC5: Backward Compatibility**
- [ ] Old URL `?Loge=Sundsvalls` still works (normalizes internally)
- [ ] Filter behavior unchanged for lodges without variants

### Benefits

1. **Better UX**: Users see clean, deduplicated lodge list
2. **No Missed Events**: Single selection catches all variants
3. **Automatic**: Works for all lodges, not just known cases
4. **Simple Rule**: Easy to understand and maintain
5. **No Breaking Changes**: Existing URLs continue to work

### Potential Issues

**Issue 1: Over-normalization**
- Might normalize names that shouldn't be normalized
- Mitigation: Minimum length requirement (> 3 chars)

**Issue 2: Legitimate different lodges**
- What if "Sundsvall" and "Sundsvalls" are different lodges?
- Mitigation: Unlikely in Swedish Freemasonry context; can be overridden with manual mapping if needed

**Issue 3: Non-Swedish lodges**
- Rule is Swedish-specific
- Mitigation: Still works, just might not normalize correctly for other languages (acceptable)

### Estimated Effort

- **Design/Planning:** 1 hour (completed)
- **Implementation:** 2-3 hours
  - Update normalization logic
  - Modify GetLodges endpoint
  - Update filter matching
- **Testing:** 1-2 hours
  - Unit tests for normalization
  - Integration tests for matching
  - UI testing
- **Documentation:** 30 minutes
- **Total:** ~5 hours

### Priority

**Recommended:** After Task 3, before or alongside Task 1

**Rationale:**
- Improves core filtering functionality
- Small, focused change
- High user value (reduces confusion)
- No dependencies on other tasks

---

## Next Steps

Before implementation:

1. **Review & Refine** - Review these specs, identify gaps, clarify requirements
2. **UI Mockups** - Create visual mockups for both tasks
3. **Technical Decisions** - Decide on storage approach for Task 2
4. **Answer Open Questions** - Resolve open questions for Task 2
5. **Create Implementation Plan** - Break down into smaller, testable increments
