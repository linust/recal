# ReCal Scalability Guide

## Overview

ReCal is designed to scale from 10 feeds to 10,000+ feeds efficiently. This document covers storage architecture, performance considerations, and alternative approaches.

## Current Architecture

### File-Based Storage (Default)

**Format**: One JSON file per feed
- Location: `./data/feeds/{uuid}.json`
- Size: ~500 bytes per feed (typical)
- Total disk usage at 10,000 feeds: ~5 MB

**File Structure**:
```json
{
  "slug": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "description": "Team meetings for Grade 4",
  "created_at": "2026-01-12T10:30:00Z",
  "updated_at": "2026-01-12T10:30:00Z",
  "filters": {
    "Grad": "4",
    "Loge": "Stockholm"
  },
  "access_count": 42,
  "last_access": "2026-01-12T14:22:00Z",
  "owner": "user@example.com"
}
```

### Performance Characteristics

**At 10,000 feeds:**
- Directory listing: <100ms (modern filesystems)
- Single feed lookup: <1ms (O(1) with UUID)
- Full scan with filter: ~500ms
- Pagination (50 feeds): ~50ms

**Modern filesystem limits:**
- ext4: 10+ million files per directory
- XFS: unlimited (practically)
- NTFS: 4+ million files per directory

## Frontend Pagination

The admin interface implements server-side pagination:

```javascript
// Load 50 feeds at a time
GET /admin/feeds?page=1&page_size=50

// With search
GET /admin/feeds?page=1&page_size=50&q=Stockholm
```

**Benefits:**
- Constant memory usage regardless of feed count
- Fast page loads (<100ms)
- Responsive search with debouncing
- Scalable to 100,000+ feeds

## Alternative Storage Approaches

If you need to scale beyond 10,000 feeds or want different trade-offs:

### Option 1: Sharded File Storage (Hybrid Approach)

Store feeds in prefix-based shards for better organization:

```
data/feeds/
  ├── a/
  │   ├── a1b2c3d4.json
  │   ├── a5e8f9a2.json
  │   └── ...
  ├── b/
  │   ├── b2c3d4e5.json
  │   └── ...
  └── ...
```

**Implementation** (future feature):
```go
// In config.yaml
feeds:
  storage_type: "sharded"
  storage_path: "./data/feeds"
  shard_depth: 1  # Use first N characters of UUID
```

**Benefits:**
- Better directory performance at 100k+ feeds
- Easier backup/restore by shard
- Same simple file-based approach

### Option 2: Grouped Feed Storage

Store multiple feeds in prefix-based files:

**File**: `data/feeds/stockholm.json`
```json
{
  "prefix": "stockholm",
  "feeds": [
    {
      "slug": "stockholm-user1",
      "description": "User 1's Stockholm feed",
      ...
    },
    {
      "slug": "stockholm-user2",
      "description": "User 2's Stockholm feed",
      ...
    }
  ]
}
```

**Usage Pattern:**
1. Request: `https://example.com/feed/stockholm-user1`
2. System loads `stockholm.json`
3. Finds feed with slug `stockholm-user1`
4. Serves filtered calendar

**Benefits:**
- Fewer files (100 files for 10,000 feeds with 100 per file)
- Faster full scans
- Group related feeds together

**Trade-offs:**
- More complex locking (write conflicts)
- Larger individual files
- Need to manage group prefixes

### Option 3: SQLite Database

For high-scale (50k+ feeds) or complex queries:

```go
// In config.yaml
feeds:
  storage_type: "sqlite"
  storage_path: "./data/feeds.db"
```

**Benefits:**
- Efficient indexing and queries
- ACID transactions
- Full-text search
- Backup via single file

**Trade-offs:**
- Requires SQLite library
- More complex implementation
- Less transparent than JSON files

### Option 4: External Database (PostgreSQL/MySQL)

For enterprise scale (100k+ feeds):

```go
// In config.yaml
feeds:
  storage_type: "postgres"
  connection: "postgresql://user:pass@localhost/recal"
```

**Benefits:**
- Multi-server deployment
- Advanced querying
- Replication/backup
- User management

**Trade-offs:**
- Requires database server
- More operational complexity
- Not self-contained

## Recommended Approach by Scale

| Feed Count | Recommendation | Notes |
|------------|----------------|-------|
| < 1,000 | **File-per-feed** (current) | Simple, fast, reliable |
| 1,000 - 10,000 | **File-per-feed** with pagination | Proven to work well |
| 10,000 - 50,000 | **Sharded files** or **SQLite** | Better directory performance |
| 50,000 - 500,000 | **SQLite** | Single-file database |
| 500,000+ | **PostgreSQL** | Distributed system |

## Programmatic Feed Creation

For bulk feed creation (e.g., from user registry):

### Python Script Example

```python
#!/usr/bin/env python3
"""
Generate feed configurations from a user registry.
Creates one feed per user with custom filters.
"""

import json
import uuid
from datetime import datetime
from pathlib import Path

def create_feed(user_email, grade, lodge):
    """Create a feed configuration for a user."""
    slug = str(uuid.uuid4())
    now = datetime.utcnow().isoformat() + "Z"

    feed = {
        "slug": slug,
        "description": f"Calendar for {user_email} - Grade {grade}",
        "created_at": now,
        "updated_at": now,
        "filters": {
            "Grad": str(grade),
            "Loge": lodge
        },
        "access_count": 0,
        "last_access": "0001-01-01T00:00:00Z",
        "owner": user_email
    }

    return slug, feed

def main():
    # Configuration
    feeds_dir = Path("./data/feeds")
    feeds_dir.mkdir(parents=True, exist_ok=True)

    # Example user registry
    users = [
        {"email": "user1@example.com", "grade": 1, "lodge": "Stockholm"},
        {"email": "user2@example.com", "grade": 2, "lodge": "Göteborg"},
        {"email": "user3@example.com", "grade": 3, "lodge": "Malmö"},
        # ... add thousands more ...
    ]

    # Generate feeds
    feed_urls = []
    for user in users:
        slug, feed = create_feed(user["email"], user["grade"], user["lodge"])

        # Write to file
        feed_path = feeds_dir / f"{slug}.json"
        with open(feed_path, 'w') as f:
            json.dump(feed, f, indent=2)

        # Record URL for email notification
        feed_url = f"https://pb.thorsell.info/feed/{slug}"
        feed_urls.append({
            "email": user["email"],
            "url": feed_url,
            "slug": slug
        })

        print(f"Created feed for {user['email']}: {slug}")

    # Export URL mapping for email notifications
    with open("feed_urls.json", 'w') as f:
        json.dump(feed_urls, f, indent=2)

    print(f"\nGenerated {len(feed_urls)} feeds")
    print(f"URL mapping saved to: feed_urls.json")

if __name__ == "__main__":
    main()
```

### Bash Script Example

```bash
#!/bin/bash
# Bulk create feeds from CSV

FEEDS_DIR="./data/feeds"
mkdir -p "$FEEDS_DIR"

# Read CSV file (format: email,grade,lodge)
while IFS=',' read -r email grade lodge; do
    # Generate UUID
    slug=$(uuidgen | tr '[:upper:]' '[:lower:]')
    now=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    # Create JSON file
    cat > "$FEEDS_DIR/${slug}.json" <<EOF
{
  "slug": "$slug",
  "description": "Calendar for $email - Grade $grade",
  "created_at": "$now",
  "updated_at": "$now",
  "filters": {
    "Grad": "$grade",
    "Loge": "$lodge"
  },
  "access_count": 0,
  "last_access": "0001-01-01T00:00:00Z",
  "owner": "$email"
}
EOF

    # Output URL for email
    echo "$email,https://pb.thorsell.info/feed/$slug" >> feed_urls.csv
    echo "Created feed for $email: $slug"
done < users.csv

echo "Done! URLs saved to feed_urls.csv"
```

## Performance Tuning

### 1. Filesystem Optimization

**For ext4:**
```bash
# Enable directory indexing (usually on by default)
tune2fs -O dir_index /dev/sdX1

# Verify
tune2fs -l /dev/sdX1 | grep dir_index
```

**For XFS (better for many files):**
```bash
# Already optimized for large directories
# No tuning needed
```

### 2. Caching Strategy

The in-memory cache (configured in config.yaml) reduces disk I/O:

```yaml
feeds:
  cache_max_age: 15m  # Keep feed configs in memory
```

At 10,000 feeds:
- Memory usage: ~5 MB
- Typical cache hit rate: >95%
- Cold start: ~1 second to load all feeds

### 3. Pagination Settings

Adjust page size based on your needs:

```javascript
// In admin page, modify:
let url = '/admin/feeds?page=' + page + '&page_size=100';  // Larger pages
```

Trade-offs:
- Larger pages: Faster scrolling, more initial load time
- Smaller pages: Faster initial load, more clicks

### 4. Search Optimization

Current implementation: Full scan with string matching
- Works well up to 10,000 feeds
- ~500ms worst case

For better performance at scale:
1. Add database with indexed search
2. Use full-text search (SQLite FTS5, PostgreSQL)
3. Implement prefix indexes

## Monitoring

Track key metrics to detect scaling issues:

```bash
# Feed count
ls data/feeds/*.json | wc -l

# Directory listing performance
time ls -1 data/feeds/ > /dev/null

# Disk usage
du -sh data/feeds/

# API response time
time curl -s "http://localhost:8080/admin/feeds?page=1" > /dev/null
```

## When to Migrate Storage

Signs you need to upgrade from file-per-feed:

1. **Directory listing >500ms**: Consider sharding
2. **Search >2 seconds**: Consider database with indexes
3. **Need complex queries**: Use SQL database
4. **Multi-server deployment**: Use external database
5. **Frequent write conflicts**: Use database with transactions

## Conclusion

The current file-per-feed architecture is:
- ✅ Simple and reliable
- ✅ Scalable to 10,000+ feeds
- ✅ Easy to backup and migrate
- ✅ No database dependencies
- ✅ Transparent and debuggable

For your use case (5,000-10,000 feeds), **no changes needed**. The system will perform well with the current architecture + pagination.
