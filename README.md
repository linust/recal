# ReCal

A lightweight Go proxy that filters iCal feeds using regular expressions. Download any iCal feed, apply regex-based filters, and serve the filtered results.

**ReCal** = **Re**gex **Cal**endar Filter or Re-Calendar to signify that what this does is to adjust calendar feeds. 

## Features

- **Flexible Filtering**: Filter iCal events using regex patterns on any field (SUMMARY, DESCRIPTION, LOCATION, etc.)
- **Named Feeds**: Create persistent feeds with UUID-based slugs for stable, shareable URLs
- **Custom Filter Expansions**: Define domain-specific filter shortcuts for your use case
- **Two-Level Caching**: Efficient caching of both upstream feeds and filtered results (15min minimum)
- **Debug Mode**: HTML output showing filtered events and match details
- **Security**: Runs as non-root in distroless container with SSRF protection
- **Reproducible Builds**: Versioned build environment ensures identical binaries across platforms
- **Feed Management API**: RESTful API for creating, updating, and monitoring named feeds


## Future Development

- **Merging of Feeds**: Multiple upstream iCal feeds merged into one continuous stream

## Quick Start

### Using Docker Compose

```bash
docker-compose up -d
```

The service will be available at `http://localhost:8080`

### Using Docker

Build the image:
```bash
docker build -t recal:latest .
```

Run the container:
```bash
docker run -p 8080:8080 recal:latest
```

### Building Locally

```bash
go build -o recal ./cmd/recal
./recal
```

## Service Interfaces

ReCal provides both user-facing and administrative interfaces:

### User Interfaces (Public)

**Web Configuration UI** - Interactive feed builder
```
http://localhost:8080/
```
Browse to the root URL to access a web form for building filter queries visually. The UI pre-fills the upstream URL from your config and provides dropdowns for available filters.

**Query Endpoint** - Get filtered iCal feed
```
http://localhost:8080/query?pattern=Meeting
```
Returns a filtered iCal feed that can be subscribed to in any calendar application (Google Calendar, Apple Calendar, Outlook, etc.).

**Preview Endpoint** - HTML preview of filtered events
```
http://localhost:8080/query/preview?pattern=Meeting
```
Shows an HTML page with filtering statistics, active filters, and sample events. Useful for testing filters before subscribing.

**Named Feeds** - Persistent, shareable feed URLs
```
http://localhost:8080/feed/{slug}
```
Subscribe to a named feed using its UUID slug. The URL stays stable even if filter parameters change.

**User Feed Management** - Self-service feed editing
```
http://localhost:8080/feed/{slug}/edit
```
End users can manage their own feed filters through a dedicated interface. Features include:

- Update feed name and description
- Visual filter selection (grade, lodge, special filters)
- Select All / Deselect All for lodge filters
- Preview changes before saving
- Copy feed URL for calendar subscription
- No admin access required - users can only edit their own feed

### Administrative Interfaces

**Admin Dashboard** - Web UI for managing feeds
```
http://localhost:8080/admin
```
Full-featured web interface for creating, editing, and deleting named feeds. Includes:
- Visual feed management with search
- Create/edit forms with filter builder
- One-click URL copying
- Direct links to preview and configure feeds
- Access statistics for each feed
- Quick access to status and health endpoints

**Status Dashboard** - Server metrics and statistics
```
http://localhost:8080/status
```
HTML dashboard showing request metrics, cache statistics, hit ratios, and system uptime.

**Health Check** - JSON health endpoint
```
http://localhost:8080/health
```
Returns `{"status":"ok","upstream_cache":N,"filtered_cache":M}` for monitoring.

**Feed Management API** - RESTful API for managing named feeds
```
POST   /admin/feeds           # Create new feed
GET    /admin/feeds           # List all feeds
GET    /admin/feeds/{slug}    # Get feed details
PUT    /admin/feeds/{slug}    # Update feed
DELETE /admin/feeds/{slug}    # Delete feed
GET    /admin/feeds/{slug}/stats  # Get access statistics
```

**Lodge API** - Get available lodges (domain-specific)
```
GET /api/lodges
```
Returns JSON list of lodge names for filter dropdowns.

## Usage Examples

### Basic Filtering

Filter events with "Meeting" in summary:
```
http://localhost:8080/query?pattern=Meeting
```

Filter by specific field:
```
http://localhost:8080/query?field=SUMMARY&pattern=Meeting
```

### Multiple Filters

Use indexed parameters for multiple filters (AND logic):
```
http://localhost:8080/query?field1=SUMMARY&pattern1=Meeting&field2=DESCRIPTION&pattern2=urgent
```

### Regex Patterns

Use standard regex syntax:
```
# Match multiple alternatives
http://localhost:8080/query?pattern=Meeting|Conference|Workshop

# Match with wildcards
http://localhost:8080/query?pattern=Project%20[ABC]

# Case-insensitive matching (use regex flags)
http://localhost:8080/query?pattern=(?i)meeting
```

### Custom Filter Expansions

You can define custom filter shortcuts in your config file for domain-specific filtering. For example:

```yaml
# In config.yaml
filters:
  priority:
    field: "SUMMARY"
    pattern_template: "\\[%s\\]"
```

Then use: `/query?priority=HIGH,URGENT`

**See [CUSTOMIZATION.md](CUSTOMIZATION.md) for detailed examples** including:
- Corporate calendars (project codes, priorities)
- School calendars (grade levels, event types)
- Multi-location businesses (office filtering)
- Par Bricole calendar (original use case)

### Custom Upstream

Specify a different upstream feed:
```
http://localhost:8080/query?upstream=https://example.com/calendar.ics&pattern=Meeting
```

### Named Feeds

Create persistent, shareable feeds with custom filter configurations. Named feeds use UUID-based slugs and provide stable URLs that won't break when filter parameters change.

#### Creating a Named Feed (Web UI)

Browse to `http://localhost:8080/admin` and use the visual interface to create and manage feeds.

#### Creating a Named Feed (API)

```bash
curl -X POST http://localhost:8080/admin/feeds \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Team meetings and standup events",
    "filters": {
      "pattern": "Meeting|Standup",
      "field": "SUMMARY"
    }
  }'
```

#### Creating Feeds Programmatically (Direct File Creation)

Named feeds are stored as JSON files in the `feeds.storage_path` directory (default: `./data/feeds`). You can create feeds programmatically by writing JSON files directly:

**File Location**: `./data/feeds/{slug}.json` where `{slug}` is a UUID v4 string

**File Format**:
```json
{
  "slug": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "description": "Team meetings and standups",
  "created_at": "2026-01-12T10:30:00Z",
  "updated_at": "2026-01-12T10:30:00Z",
  "filters": {
    "pattern": "Meeting|Standup",
    "field": "SUMMARY",
    "Grad": "1,2,3"
  },
  "access_count": 0,
  "last_access": "0001-01-01T00:00:00Z",
  "owner": ""
}
```

**Example Script** (Python):
```python
import json
import uuid
from datetime import datetime
from pathlib import Path

# Configuration
feeds_dir = Path("./data/feeds")
feeds_dir.mkdir(parents=True, exist_ok=True)

# Generate a new feed
slug = str(uuid.uuid4())
feed = {
    "slug": slug,
    "description": "Generated feed from user registry",
    "created_at": datetime.utcnow().isoformat() + "Z",
    "updated_at": datetime.utcnow().isoformat() + "Z",
    "filters": {
        "Grad": "1,2,3",
        "Loge": "Stockholm"
    },
    "access_count": 0,
    "last_access": "0001-01-01T00:00:00Z",
    "owner": "user@example.com"
}

# Write to file
feed_path = feeds_dir / f"{slug}.json"
with open(feed_path, 'w') as f:
    json.dump(feed, f, indent=2)

print(f"Created feed: http://localhost:8080/feed/{slug}")
```

Response:
```json
{
  "slug": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "description": "Team meetings and standup events",
  "filters": {
    "pattern": "Meeting|Standup",
    "field": "SUMMARY"
  },
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z",
  "access_count": 0,
  "urls": {
    "feed": "http://localhost:8080/feed/a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "edit": "http://localhost:8080/feed/a1b2c3d4-e5f6-7890-abcd-ef1234567890/edit",
    "preview": "http://localhost:8080/feed/a1b2c3d4-e5f6-7890-abcd-ef1234567890/preview"
  }
}
```

#### Using Named Feeds

**Subscribe to feed:**
```
http://localhost:8080/feed/{slug}
```

**Edit feed filters:**
```
http://localhost:8080/feed/{slug}/edit
```

**Preview filtered events (HTML debug view):**
```
http://localhost:8080/feed/{slug}/preview
```

#### Managing Named Feeds

**List all feeds:**
```bash
curl http://localhost:8080/admin/feeds
```

**Get feed details:**
```bash
curl http://localhost:8080/admin/feeds/{slug}
```

**Update feed:**
```bash
curl -X PUT http://localhost:8080/admin/feeds/{slug} \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Updated description",
    "filters": {
      "pattern": "NewPattern",
      "Grad": "1,2,3"
    }
  }'
```

**Delete feed:**
```bash
curl -X DELETE http://localhost:8080/admin/feeds/{slug}
```

**Get feed statistics:**
```bash
curl http://localhost:8080/admin/feeds/{slug}/stats
```

Response:
```json
{
  "slug": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "access_count": 42,
  "created_at": "2024-01-15T10:30:00Z",
  "last_accessed_at": "2024-01-15T14:22:00Z"
}
```

## Configuration

Copy `config.yaml.example` to `config.yaml` and customize:

- **Server settings**: Port, timeouts, base URL
- **Upstream settings**: Default iCal URL, timeout
- **Cache settings**: Max size, memory limits, TTL (15min minimum for output)
- **Regex settings**: Max execution time (DoS protection)
- **Custom filters**: Define domain-specific filter expansions (optional)

**See [CUSTOMIZATION.md](CUSTOMIZATION.md) for detailed configuration examples.**

For Par Bricole specific setup, see `config-parbricole.yaml.example`.

### Environment Variables

Override config with environment variables:

- `PORT`: HTTP server port (e.g., `8080`)
- `BASE_URL`: Base URL for the server (e.g., `http://localhost:8080`)
- `DEFAULT_UPSTREAM`: Default upstream iCal URL
- `CACHE_MAX_SIZE`: Maximum cache entries (e.g., `100`)
- `CACHE_DEFAULT_TTL`: Default cache TTL (e.g., `5m`)
- `CACHE_MIN_OUTPUT`: Minimum output cache time (e.g., `15m`)
- `UPSTREAM_TIMEOUT`: Timeout for upstream requests (e.g., `30s`)
- `MAX_REGEX_TIME`: Maximum regex execution time (e.g., `1s`)
- `CONFIG_FILE`: Path to config file (default: `./config.yaml`)

## Health Check

```
http://localhost:8080/health
```

Returns JSON with status and cache statistics.

## Development

All build and test operations use Docker for reproducibility. This ensures the same binary is produced regardless of the development environment.

### Build Commands

**Reproducible build (Docker):**
```bash
make build        # Build binary using Docker (recommended for releases)
make test         # Run tests using Docker
make fmt          # Format code using Docker
make vet          # Run go vet using Docker
make lint         # Run golangci-lint using Docker
```

**Fast local development:**
```bash
make build-local  # Build using local Go (faster iteration)
make test-local   # Test using local Go
make dev          # Quick cycle: test + build locally
```

### Running Tests

The project includes 65+ automated tests across all components.

```bash
# Run all tests in Docker (reproducible, CI-ready)
make test

# Run tests locally (faster development)
make test-local

# Run tests with coverage report
make test-coverage

# Run integration tests against live server
make test-integration
```

**Test Types:**
- **Unit Tests** - Test individual components (Go)
- **Integration Tests** - Test component interactions (Go, httptest)
- **System Tests** - Test live HTTP server (Bash)

**Test Coverage:**
- 65+ tests across 6 packages
- Validates configuration page at `http://localhost:8080/`
- Validates filter functionality with test data
- Real test data: `testdata/sample-feed.ics`

See **[TESTING.md](TESTING.md)** for detailed testing documentation.

### CI/CD

The project uses GitHub Actions for CI/CD:
- **`.github/workflows/ci.yml`**: Runs tests, linting, and builds on every push/PR
- **`renovate.json`**: Automatic dependency updates via Renovate

All CI operations use Docker for consistency with local development.

### Project Structure

```
recal/
├── cmd/recal/                     # Main application entry point
├── internal/
│   ├── cache/                     # Two-level cache (upstream + filtered)
│   ├── config/                    # Configuration loader with env overrides
│   ├── fetcher/                   # Upstream fetcher with HTTP caching & SSRF protection
│   ├── filter/                    # Generic filter engine with custom expansions
│   ├── parser/                    # iCal parser (RFC 5545)
│   └── server/                    # HTTP server with debug mode
├── testdata/                      # Test fixtures
├── config.yaml.example            # Generic configuration template
├── config-parbricole.yaml.example # Par Bricole specific example
├── CUSTOMIZATION.md               # Guide to custom filters
├── Dockerfile                     # Multi-stage distroless build
└── docker-compose.yml             # Docker Compose configuration
```

## Security

- **Distroless Runtime**: Minimal attack surface, no shell
- **Non-Root User**: Runs as UID 65532
- **Read-Only Filesystem**: Container has no write access
- **SSRF Protection**: Blocks access to private networks
- **Regex DoS Protection**: Timeout limits for regex execution
- **Input Validation**: Sanitizes all URL parameters
- **XSS Protection**: HTML escaping in debug mode

## Architecture

See [CLAUDE.md](CLAUDE.md) for detailed design documentation.

## License

[Your License Here]
