# Development Workflow Guide

This document describes the improved development workflow for ReCal to avoid the inefficient "push → wait → download logs → debug" cycle.

## Quick Start

```bash
# 1. Test locally before pushing (catches issues early!)
make test-ci-local

# 2. Push and watch CI automatically
git push && make watch-ci
```

## Available Tools

### 1. Local CI Testing (`test-local.sh`)

**What it does:** Runs the exact same integration tests that GitHub Actions runs, but locally on your machine.

**Why use it:** Catch CI failures before pushing to GitHub, saving time and avoiding failed builds.

**Usage:**
```bash
# Run directly
./test-local.sh

# Or via Makefile
make test-ci-local
```

**What it tests:**
- ✅ Builds Docker image locally
- ✅ Starts Python HTTP server for test data
- ✅ Tests `/health` endpoint
- ✅ Tests `/status` endpoint
- ✅ Tests `/filter` endpoint (no filters)
- ✅ Tests `/filter` with Grad filter
- ✅ Tests `/debug` endpoint
- ✅ Validates iCal output format

**Requirements:**
- Docker
- Python 3 (for test HTTP server)

### 2. CI Build Monitoring (`watch-ci.sh`)

**What it does:** Watches your GitHub Actions CI build in real-time and automatically downloads logs if it fails.

**Why use it:** No more manually refreshing GitHub, checking Actions tab, clicking through to logs, and downloading them.

**Usage:**
```bash
# Run directly
./watch-ci.sh

# Or via Makefile
make watch-ci

# Combined with push
git push && make watch-ci
```

**What it does:**
1. Detects latest commit SHA
2. Waits for GitHub Actions workflow to start
3. Watches build status in real-time
4. If build fails:
   - Auto-downloads logs to `./temp/logs_<run_id>/`
   - Displays last 50 lines of test output
   - Shows error context immediately
5. If build succeeds:
   - Prints success message and exits

**Requirements:**
- GitHub CLI (`gh`)
  ```bash
  # Install on macOS
  brew install gh

  # Authenticate
  gh auth login
  ```

## Recommended Workflow

### Option 1: Test Locally First (Recommended)

```bash
# 1. Make your changes
vim internal/server/server.go

# 2. Run unit tests
make test-local

# 3. Run CI integration tests locally
make test-ci-local

# 4. If all pass, push
git add .
git commit -m "Your changes"
git push

# 5. Optionally watch CI
make watch-ci
```

### Option 2: Watch CI After Push

```bash
# 1. Make changes and push
git add .
git commit -m "Your changes"
git push && make watch-ci

# If CI fails:
# - Logs are automatically downloaded to ./temp/
# - Check the error output shown in terminal
# - Fix the issue
# - Run 'make test-ci-local' to verify fix
# - Push again
```

### Option 3: Quick Dev Cycle

```bash
# Run tests and build in one command
make dev

# If tests pass, commit and push
git add .
git commit -m "Your changes"
git push && make watch-ci
```

## Makefile Commands

Run `make help` or just `make` to see all available commands:

```
Available targets:
  build           - Build the binary using Docker (reproducible)
  build-local     - Build the binary using local Go (faster)
  test            - Run all tests using Docker (reproducible)
  test-local      - Run all tests using local Go (faster)
  test-coverage   - Run tests with coverage report
  test-integration - Run integration tests against live server
  test-ci-local   - Run CI integration tests locally (before pushing) ⭐
  run             - Run the application using Docker
  clean           - Remove build artifacts
  docker-build    - Build Docker image
  docker-run      - Run Docker container
  docker-clean    - Remove Docker images
  fmt             - Format code using Docker
  vet             - Run go vet using Docker
  lint            - Run golangci-lint using Docker
  ci-test         - Run CI test suite (used by GitHub Actions)
  watch-ci        - Watch CI build and auto-download logs ⭐
  dev             - Quick dev cycle: test-local + build-local ⭐
```

## Common Issues

### Issue: `test-local.sh` fails with Docker errors

**Solution:** Make sure Docker is running:
```bash
docker ps
```

### Issue: `watch-ci.sh` says "gh not found"

**Solution:** Install GitHub CLI:
```bash
# macOS
brew install gh

# Linux (Debian/Ubuntu)
sudo apt install gh

# Authenticate
gh auth login
```

### Issue: `test-local.sh` fails with "port 8888 already in use"

**Solution:** Kill any existing Python HTTP server:
```bash
pkill -f "python3 -m http.server 8888"
```

Or use a different port (edit `test-local.sh`).

### Issue: CI passes locally but fails on GitHub

**Possible causes:**
1. **Environment differences**: Check if you're using `DISABLE_SSRF_PROTECTION=true` locally but not in CI
2. **Timing issues**: CI might be slower, add sleep delays if needed
3. **Test data differences**: Ensure `testdata/` is committed

## Advanced: Pre-push Hook

Automatically run tests before every push:

```bash
# Create pre-push hook
cat > .git/hooks/pre-push <<'EOF'
#!/bin/bash
echo "Running CI tests before push..."
make test-ci-local
if [ $? -ne 0 ]; then
    echo ""
    echo "❌ CI tests failed! Push cancelled."
    echo "Fix the errors and try again."
    exit 1
fi
echo "✅ CI tests passed, proceeding with push..."
EOF

chmod +x .git/hooks/pre-push
```

Now every `git push` will automatically run `make test-ci-local` first!

## Debugging Failed Tests

### 1. Check Docker logs

If a test fails, check what the container is logging:

```bash
# During test-local.sh run
docker logs recal-test

# If container exists
docker ps -a | grep recal
docker logs <container-id>
```

### 2. Interactive debugging

Start container manually and poke around:

```bash
# Build image
docker build -t recal:debug .

# Run interactively
docker run -it --rm \
  --network host \
  -e DISABLE_SSRF_PROTECTION=true \
  -v $(pwd)/test-config.yaml:/app/config.yaml:ro \
  recal:debug /bin/sh

# Or run with logs visible
docker run --rm \
  --network host \
  -e DISABLE_SSRF_PROTECTION=true \
  -v $(pwd)/test-config.yaml:/app/config.yaml:ro \
  recal:debug
```

### 3. Test individual endpoints

```bash
# Start container
./test-local.sh

# In another terminal, test endpoints manually
curl -v http://localhost:8080/health
curl -v http://localhost:8080/filter
curl -v "http://localhost:8080/filter?Grad=3"
curl -v "http://localhost:8080/debug?Grad=3" | less
```

## Comparison: Old vs New Workflow

### Old Workflow (Inefficient)
1. Make changes
2. Push to GitHub
3. Wait 5-10 minutes for CI to run
4. CI fails
5. Go to GitHub Actions tab
6. Click through to failed job
7. Scroll through long logs
8. Find the error
9. Download logs manually
10. Copy logs to temp folder
11. Tell Claude to check temp folder
12. Wait for analysis
13. Fix issue
14. Repeat from step 2

**Time per iteration:** 10-15 minutes

### New Workflow (Efficient)
1. Make changes
2. Run `make test-ci-local` (takes 30-60 seconds)
3. If it passes, push
4. Optionally run `make watch-ci` to monitor
5. If CI fails, logs are auto-downloaded
6. Error is immediately visible in terminal

**Time per iteration:** 1-2 minutes

**Time saved:** 80-90%

## Future Improvements

Potential future enhancements to consider:

1. **VS Code integration**: Add tasks.json to run tests from IDE
2. **Watch mode**: Auto-run tests when files change
3. **Slack/Discord notifications**: Get notified when CI fails
4. **Performance benchmarks**: Track request latency in CI
5. **Docker layer caching**: Speed up local Docker builds
6. **Parallel test execution**: Run integration tests in parallel

## Summary

- ✅ **Run `make test-ci-local` before every push** to catch failures early
- ✅ **Use `make watch-ci` after pushing** to automatically monitor CI
- ✅ **No more manual log downloading** - it's all automated
- ✅ **Immediate feedback** - errors shown in terminal within seconds
- ✅ **85%+ time savings** compared to old workflow

Happy developing! 🚀
