# ReCal

A lightweight Go proxy that filters iCal feeds using regular expressions. Download any iCal feed, apply regex-based filters, and serve the filtered results.

**ReCal** = **Re**gex **Cal**endar Filter

## Features

- **Flexible Filtering**: Filter iCal events using regex patterns on any field (SUMMARY, DESCRIPTION, LOCATION, etc.)
- **Custom Filter Expansions**: Define domain-specific filter shortcuts for your use case
- **Two-Level Caching**: Efficient caching of both upstream feeds and filtered results (15min minimum)
- **Debug Mode**: HTML output showing filtered events and match details
- **Security**: Runs as non-root in distroless container with SSRF protection
- **Reproducible Builds**: Versioned build environment ensures identical binaries across platforms

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

## Usage

### Basic Filtering

Filter events with "Meeting" in summary:
```
http://localhost:8080/filter?pattern=Meeting
```

Filter by specific field:
```
http://localhost:8080/filter?field=SUMMARY&pattern=Meeting
```

### Multiple Filters

Use indexed parameters for multiple filters (AND logic):
```
http://localhost:8080/filter?field1=SUMMARY&pattern1=Meeting&field2=DESCRIPTION&pattern2=urgent
```

### Regex Patterns

Use standard regex syntax:
```
# Match multiple alternatives
http://localhost:8080/filter?pattern=Meeting|Conference|Workshop

# Match with wildcards
http://localhost:8080/filter?pattern=Project%20[ABC]

# Case-insensitive matching (use regex flags)
http://localhost:8080/filter?pattern=(?i)meeting
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

Then use: `/filter?priority=HIGH,URGENT`

**See [CUSTOMIZATION.md](CUSTOMIZATION.md) for detailed examples** including:
- Corporate calendars (project codes, priorities)
- School calendars (grade levels, event types)
- Multi-location businesses (office filtering)
- Par Bricole calendar (original use case)

### Debug Mode

Enable debug mode to see filtering details:
```
http://localhost:8080/filter?pattern=Meeting&debug=true
```

### Custom Upstream

Specify a different upstream feed:
```
http://localhost:8080/filter?upstream=https://example.com/calendar.ics&pattern=Meeting
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
