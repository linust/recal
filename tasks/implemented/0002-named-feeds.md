# Task 2: Named Feeds (Persistent Filter Configurations)

## Overview
Allow users to save filter configurations as named feeds with persistent slugs, enabling feed updates without requiring calendar reconfiguration.

## Current State
- Filters are defined via URL query parameters
- URL must be updated in calendar apps to change filters
- No way to save/name configurations
- No persistence layer

## Goals
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

## Requirements

### Functional Requirements

**FR1: Feed Creation**
- [ ] User can save current filter configuration as named feed
- [ ] System generates unique slug (UUID v4)
- [ ] User provides human-readable description
- [ ] Description is stored with configuration
- [ ] Confirmation page shows slug URL

**FR2: Feed Access**
- [ ] Named feed URL: `/feed/{uuid}`
- [ ] Serves filtered iCal based on saved configuration
- [ ] Returns 404 if slug not found
- [ ] Config page: `/feed/{uuid}/config`
- [ ] Debug page: `/feed/{uuid}/debug`

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

### Non-Functional Requirements

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

## Security Requirements

### Security Model

**Principle**: UUID-based access control with upstream-protected admin endpoints

The security model uses a two-tier approach:
1. **Feed Access (Public)**: Protected by UUID secrecy - knowing the UUID grants access
2. **Admin Operations (Protected)**: Confined to `/admin/*` path prefix for upstream authentication

### Security Requirements

**SR1: Feed Access Protection**
- [ ] Feed UUIDs must be generated using cryptographically secure random (UUID v4)
- [ ] UUID length: 36 characters (standard UUID format: `xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx`)
- [ ] Feed access via `/feed/{uuid}` requires knowing the full UUID
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

### Recommended Reverse Proxy Configuration

**Nginx Example:**
```nginx
# Public feed access - no auth required
location /feed/ {
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
handle /feed/* {
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

### Security Documentation

The service will include documentation explaining:
1. UUID-based access is "security by obscurity" - treat UUIDs as secrets
2. Share feed URLs only with intended recipients
3. Admin endpoints MUST be protected by upstream authentication
4. Recommended: Use HTTPS for all traffic (webcals:// for calendar subscriptions)
5. Feed rotation: Delete and recreate if UUID is compromised

## Technical Design

### Data Model

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

### API Endpoints

**Public Endpoints (No Authentication Required):**

1. **Get Feed (iCal)**
   ```
   GET /feed/{uuid}

   Response 200 OK:
   Content-Type: text/calendar
   Cache-Control: public, max-age=900
   [iCal data based on saved filters]

   Response 404 Not Found:
   Feed not found
   ```

2. **Get Feed Config Page**
   ```
   GET /feed/{uuid}/config

   Response 200 OK:
   Content-Type: text/html
   [Configuration page with saved filters pre-loaded]
   Shows: Description, current filters, ability to save changes (creates new feed)
   ```

3. **Get Feed Debug Page**
   ```
   GET /feed/{uuid}/debug

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
     "url": "https://pb.thorsell.info/feed/a1b2c3d4-e5f6-7890-abcd-ef1234567890",
     "config_url": "https://pb.thorsell.info/feed/a1b2c3d4-e5f6-7890-abcd-ef1234567890/config",
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

### UI Flow

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

### Configuration

**Add to config.yaml:**
```yaml
feeds:
  storage_path: "./data/feeds"  # Where to store feed data
  max_feeds: 100                # Maximum feeds per instance
  slug_length: 36               # UUID v4 length
  retention_days: 365           # Auto-delete after N days of inactivity
```

## Implementation Files

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

## Testing Requirements

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

## Dependencies

**New Dependencies:**
- `github.com/google/uuid` - For UUID generation

**Configuration:**
- Storage directory must be writable
- File permissions for feed data

## Migration Path

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

## Estimated Effort

- **Design/Planning:** 2-3 hours
- **Data Model & Storage:** 4-6 hours
- **API Endpoints:** 4-6 hours
- **UI Implementation:** 6-8 hours
- **Testing:** 4-6 hours
- **Documentation:** 2 hours
- **Total:** ~25-30 hours

## Open Questions

### Answers (from SESSION_STATE.md):

1. **Authentication:** Do we need user accounts or is the slug security enough?
   - **Answer:** Yes, UUID security is enough

2. **Multi-tenancy:** Should different users have separate feed namespaces?
   - **Answer:** This might be a future extension

3. **Limits:** What limits on number of feeds, description length, etc.?
   - **Answer:** Description length: 500 characters, no limit on feed number yet (caching should keep it manageable)

4. **Analytics:** What statistics should we track beyond access count?
   - **Answer:** Average feed length, average response time

5. **Export/Import:** Should users be able to export/import feed configurations?
   - **Answer:** Since we will initially keep this as a set of files in a folder we do not need to implement an import/export functionality

6. **Versioning:** Should feeds have version history?
   - **Answer:** Can be tracked with incremental backups
