#!/bin/bash
# Integration test script for validating the ReCal server
# Tests actual HTTP endpoints when the server is running

set -e

BASE_URL="${1:-http://localhost:8080}"

echo "Testing ReCal server at $BASE_URL"
echo "========================================="
echo ""

# Test 1: Root endpoint shows configuration page
echo "Test 1: GET / (Configuration Page)"
response=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/")
if [ "$response" = "200" ]; then
    echo "✓ PASS: Root endpoint returns 200 OK"

    # Check if page contains expected elements
    page_content=$(curl -s "$BASE_URL/")
    if echo "$page_content" | grep -q "ReCal"; then
        echo "✓ PASS: Page contains 'ReCal' title"
    else
        echo "✗ FAIL: Page missing 'ReCal' title"
    fi

    if echo "$page_content" | grep -q "grad-select"; then
        echo "✓ PASS: Page contains grade selector"
    else
        echo "✗ FAIL: Page missing grade selector"
    fi

    if echo "$page_content" | grep -q "loge-checkboxes"; then
        echo "✓ PASS: Page contains lodge checkboxes"
    else
        echo "✗ FAIL: Page missing lodge checkboxes"
    fi
else
    echo "✗ FAIL: Root endpoint returned $response, expected 200"
fi
echo ""

# Test 2: Health endpoint
echo "Test 2: GET /health (Health Check)"
response=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health")
if [ "$response" = "200" ]; then
    echo "✓ PASS: Health endpoint returns 200 OK"

    health_json=$(curl -s "$BASE_URL/health")
    if echo "$health_json" | grep -q '"status":"ok"'; then
        echo "✓ PASS: Health check contains status:ok"
    else
        echo "✗ FAIL: Health check missing status:ok"
    fi
else
    echo "✗ FAIL: Health endpoint returned $response, expected 200"
fi
echo ""

# Test 3: Filter endpoint with no parameters (should redirect)
echo "Test 3: GET /filter (No parameters - should redirect)"
response=$(curl -s -o /dev/null -w "%{http_code}" -L "$BASE_URL/filter")
if [ "$response" = "200" ]; then
    echo "✓ PASS: /filter redirects to configuration page (followed redirect to 200)"
else
    echo "✗ FAIL: /filter returned $response after following redirect"
fi

# Check redirect without following
redirect_response=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/filter")
if [ "$redirect_response" = "303" ]; then
    echo "✓ PASS: /filter returns 303 redirect"
else
    echo "✗ FAIL: /filter returned $redirect_response, expected 303"
fi
echo ""

# Test 4: Filter endpoint with pattern (requires actual upstream)
echo "Test 4: GET /filter?pattern=test (Basic filter)"
response=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/filter?pattern=test")
if [ "$response" = "200" ]; then
    echo "✓ PASS: Filter endpoint returns 200 OK"

    # Check content type
    content_type=$(curl -s -I "$BASE_URL/filter?pattern=test" | grep -i "content-type" | cut -d' ' -f2-)
    if echo "$content_type" | grep -q "text/calendar"; then
        echo "✓ PASS: Content-Type is text/calendar"
    else
        echo "✗ FAIL: Content-Type is $content_type, expected text/calendar"
    fi

    # Check for iCal content
    ical_content=$(curl -s "$BASE_URL/filter?pattern=test")
    if echo "$ical_content" | grep -q "BEGIN:VCALENDAR"; then
        echo "✓ PASS: Response contains valid iCal data"
    else
        echo "✗ FAIL: Response missing VCALENDAR"
    fi

    # Check cache headers
    cache_control=$(curl -s -I "$BASE_URL/filter?pattern=test" | grep -i "cache-control" | cut -d' ' -f2-)
    if echo "$cache_control" | grep -q "max-age"; then
        echo "✓ PASS: Cache-Control header present"
    else
        echo "✗ FAIL: Cache-Control header missing or invalid"
    fi
elif [ "$response" = "502" ]; then
    echo "⚠ SKIP: Filter returned 502 (upstream fetch failed - expected in test environment)"
else
    echo "✗ FAIL: Filter endpoint returned $response"
fi
echo ""

# Test 5: Debug mode
echo "Test 5: GET /filter?pattern=test&debug=true (Debug mode)"
response=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/filter?pattern=test&debug=true")
if [ "$response" = "200" ]; then
    echo "✓ PASS: Debug mode returns 200 OK"

    # Check content type
    content_type=$(curl -s -I "$BASE_URL/filter?pattern=test&debug=true" | grep -i "content-type" | cut -d' ' -f2-)
    if echo "$content_type" | grep -q "text/html"; then
        echo "✓ PASS: Content-Type is text/html"
    else
        echo "✗ FAIL: Content-Type is $content_type, expected text/html"
    fi

    # Check for debug page elements
    debug_content=$(curl -s "$BASE_URL/filter?pattern=test&debug=true")
    if echo "$debug_content" | grep -q "ReCal Debug Report"; then
        echo "✓ PASS: Debug page contains report title"
    else
        echo "✗ FAIL: Debug page missing report title"
    fi

    if echo "$debug_content" | grep -q "Summary Statistics"; then
        echo "✓ PASS: Debug page contains statistics"
    else
        echo "✗ FAIL: Debug page missing statistics"
    fi
elif [ "$response" = "502" ]; then
    echo "⚠ SKIP: Debug mode returned 502 (upstream fetch failed - expected in test environment)"
else
    echo "✗ FAIL: Debug mode returned $response"
fi
echo ""

# Test 6: Special filters
echo "Test 6: GET /filter?Grad=4 (Grad filter)"
response=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/filter?Grad=4")
if [ "$response" = "200" ]; then
    echo "✓ PASS: Grad filter returns 200 OK"
elif [ "$response" = "502" ]; then
    echo "⚠ SKIP: Grad filter returned 502 (upstream fetch failed - expected in test environment)"
else
    echo "✗ FAIL: Grad filter returned $response"
fi
echo ""

echo "Test 7: GET /filter?Loge=Göta,Borås (Loge filter)"
response=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/filter?Loge=G%C3%B6ta,Bor%C3%A5s")
if [ "$response" = "200" ]; then
    echo "✓ PASS: Loge filter returns 200 OK"
elif [ "$response" = "502" ]; then
    echo "⚠ SKIP: Loge filter returned 502 (upstream fetch failed - expected in test environment)"
else
    echo "✗ FAIL: Loge filter returned $response"
fi
echo ""

echo "Test 8: GET /filter?RemoveUnconfirmed (RemoveUnconfirmed filter)"
response=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/filter?RemoveUnconfirmed")
if [ "$response" = "200" ]; then
    echo "✓ PASS: RemoveUnconfirmed filter returns 200 OK"
elif [ "$response" = "502" ]; then
    echo "⚠ SKIP: RemoveUnconfirmed filter returned 502 (upstream fetch failed - expected in test environment)"
else
    echo "✗ FAIL: RemoveUnconfirmed filter returned $response"
fi
echo ""

echo "Test 9: GET /filter?RemoveInstallt (RemoveInstallt filter)"
response=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/filter?RemoveInstallt")
if [ "$response" = "200" ]; then
    echo "✓ PASS: RemoveInstallt filter returns 200 OK"
elif [ "$response" = "502" ]; then
    echo "⚠ SKIP: RemoveInstallt filter returned 502 (upstream fetch failed - expected in test environment)"
else
    echo "✗ FAIL: RemoveInstallt filter returned $response"
fi
echo ""

# Test 10: API endpoints
echo "Test 10: GET /api/lodges (Lodge list API)"
response=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/lodges")
if [ "$response" = "200" ]; then
    echo "✓ PASS: Lodges API returns 200 OK"

    lodges_json=$(curl -s "$BASE_URL/api/lodges")
    if echo "$lodges_json" | grep -q '"lodges"'; then
        echo "✓ PASS: Response contains lodges array"
    else
        echo "✗ FAIL: Response missing lodges array"
    fi
elif [ "$response" = "502" ]; then
    echo "⚠ SKIP: Lodges API returned 502 (upstream fetch failed - expected in test environment)"
else
    echo "✗ FAIL: Lodges API returned $response"
fi
echo ""

echo "========================================="
echo "Testing complete!"
echo ""
echo "Note: Tests that require upstream calendar access may show SKIP or FAIL"
echo "if the upstream is not accessible. This is expected in test environments."
