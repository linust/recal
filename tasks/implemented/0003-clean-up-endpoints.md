# Task 3: Clean Up Endpoints (API Redesign)

## Status
✅ **COMPLETE** - Implemented on 2025-11-08

## Overview

Redesign endpoint structure to improve clarity, consistency, and firewall-based access control while maintaining RESTful principles where appropriate.

## Current State

Endpoints are functional but could be better organized:
- Mix of paths without clear pattern (`/`, `/query`, `/debug` (redirects to `/query/preview`))
- No clear namespace for future named feeds feature
- Difficult to apply path-based firewall rules for future admin features

## Goals

1. **Clear Path Structure**: Organize endpoints with intent-focused prefixes
2. **Firewall-Friendly**: Enable simple path-based access control
3. **RESTful Conventions**: Follow REST principles for API endpoints
4. **Future-Proof**: Support upcoming named feeds feature cleanly

## Design Principles

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

## What Was Implemented

### Changes Made
1. Added `/query/preview` endpoint (new canonical debug/preview endpoint)
2. Added `/debug` → `/query/preview` redirect (301 Moved Permanently for backward compatibility)
3. Updated integration tests with new `TestIntegrationDebugRedirect` test
4. Updated CI/CD workflow to test both endpoints

### Files Modified
- `internal/server/server.go` - Added DebugRedirect handler, updated routing
- `internal/server/integration_test.go` - Added redirect tests
- `.github/workflows/docker-publish.yml` - Updated CI tests

### Test Results
- ✅ All Go tests pass (18 tests)
- ✅ Docker build successful (recal:test image created)
- ✅ Manual testing verified redirect works correctly
- ✅ Backward compatible (old /debug URLs work via redirect)

## Endpoint Changes Summary

**Renamed/Reorganized:**
- `/debug` (redirects to `/query/preview`) → `/query/preview` (clearer intent, groups with `/query`)

**New Endpoints:**
- `/query/preview` - Debug mode for query-based filters
- `/feed/{uuid}` - Named feed iCal (future - Task 2)
- `/feed/{uuid}/config` (GET/POST) - Named feed configuration UI (future - Task 2)
- `/feed/{uuid}/preview` - Named feed debug mode (future - Task 2)
- `/admin/feeds` (POST/GET) - Admin feed management (future - Task 2)
- `/admin/feeds/{uuid}` (GET/DELETE) - Admin single feed operations (future - Task 2)

**Unchanged:**
- `/` - Config page
- `/query` - Query-based iCal
- `/status` - Status page
- `/health` - Health check
- `/api/lodges` - Lodge listing

## Benefits

1. **Clearer Intent**: `/query/preview` makes purpose obvious
2. **Better Organization**: Related endpoints grouped by prefix
3. **Simple Firewall Rules**: Single rule protects all admin operations
4. **REST Compliance**: Standard HTTP methods for CRUD operations
5. **Scalable**: Easy to add more resources under `/admin/` or `/feed/`
6. **Backward Compatible**: Old `/debug` endpoint redirects to new location

## Estimated Effort

- **Planning/Design:** 1 hour
- **Implementation:** 2-3 hours
- **Testing:** 1 hour
- **Documentation:** 1 hour
- **Total:** ~5 hours
