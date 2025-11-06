#!/bin/bash
set -e

echo "=== Local Integration Test ==="

# Build the Docker image locally
echo "Building Docker image..."
docker build -t recal:test .

# Start HTTP server for test data
echo "Starting test data server..."
cd testdata
python3 -m http.server 8888 &
HTTP_SERVER_PID=$!
cd ..

# Wait for server
sleep 2

# Verify test data is accessible
curl -f http://localhost:8888/sample-feed.ics > /dev/null || {
  echo "Failed to access test data"
  kill $HTTP_SERVER_PID
  exit 1
}

# Create test config
cat > test-config.yaml <<EOF
server:
  port: 8080
  base_url: "http://localhost:8080"
upstream:
  default_url: "http://localhost:8888/sample-feed.ics"
  timeout: 30s
cache:
  max_size: 10
  max_memory: 20971520
  default_ttl: 5m
  min_output_cache: 15m
  max_ttl: 72h
regex:
  max_execution_time: 1s
filters:
  grad:
    field: "SUMMARY"
    pattern_template: "Grad %s"
  loge:
    field: "SUMMARY"
    patterns:
      default:
        template: "%s PB:"
  confirmed_only:
    field: "STATUS"
    pattern: "CONFIRMED"
    description: "Test filter"
  installt:
    field: "SUMMARY"
    pattern: "INSTÄLLT"
    description: "Test filter"
EOF

# Run container
echo "Starting test container..."
docker run -d \
  --network host \
  -e DISABLE_SSRF_PROTECTION=true \
  -v $(pwd)/test-config.yaml:/app/config.yaml:ro \
  --name recal-test \
  recal:test

# Wait for startup
sleep 5

# Function to cleanup
cleanup() {
  echo "Cleaning up..."
  docker stop recal-test 2>/dev/null || true
  docker rm recal-test 2>/dev/null || true
  kill $HTTP_SERVER_PID 2>/dev/null || true
  rm -f test-config.yaml filtered.ics grad-filtered.ics debug.html
}

# Set trap for cleanup on exit
trap cleanup EXIT

# Run tests
echo "Testing /health endpoint..."
curl -f http://localhost:8080/health || {
  echo "❌ Health check failed"
  docker logs recal-test
  exit 1
}
echo "✅ Health check passed"

echo "Testing /status endpoint..."
curl -f http://localhost:8080/status > /dev/null || {
  echo "❌ Status check failed"
  exit 1
}
echo "✅ Status check passed"

echo "Testing /filter endpoint (no filters)..."
curl -f "http://localhost:8080/filter" -o filtered.ics || {
  echo "❌ Filter endpoint failed"
  docker logs recal-test
  exit 1
}

grep -q "BEGIN:VCALENDAR" filtered.ics || {
  echo "❌ Output is not a valid iCal file"
  cat filtered.ics
  exit 1
}
echo "✅ Filter endpoint passed"

echo "Testing Grad filter..."
curl -f "http://localhost:8080/filter?Grad=3" -o grad-filtered.ics || {
  echo "❌ Grad filter failed"
  exit 1
}
echo "✅ Grad filter passed"

echo "Testing debug mode..."
curl -f "http://localhost:8080/debug?Grad=3" > debug.html || {
  echo "❌ Debug mode failed"
  exit 1
}

grep -q "Summary Statistics" debug.html || {
  echo "❌ Debug output missing expected content"
  cat debug.html
  exit 1
}
echo "✅ Debug mode passed"

echo ""
echo "🎉 All tests passed!"
