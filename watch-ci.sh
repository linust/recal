#!/bin/bash
# Watch CI build and download logs automatically if it fails
# Requires: gh (GitHub CLI) - install with: brew install gh

set -e

# Check if gh is installed
if ! command -v gh &> /dev/null; then
    echo "❌ GitHub CLI (gh) is not installed"
    echo "Install with: brew install gh"
    exit 1
fi

# Check if authenticated
if ! gh auth status &> /dev/null; then
    echo "❌ Not authenticated with GitHub CLI"
    echo "Run: gh auth login"
    exit 1
fi

echo "🔍 Watching CI build for latest commit..."
echo ""

# Get the latest commit SHA
COMMIT_SHA=$(git rev-parse HEAD)
SHORT_SHA=$(git rev-parse --short HEAD)

echo "📝 Latest commit: $SHORT_SHA"
echo "⏳ Waiting for workflow to start..."

# Wait for workflow to start (max 30 seconds)
for i in {1..30}; do
    WORKFLOW_RUN=$(gh run list --commit "$COMMIT_SHA" --json databaseId,status,conclusion --limit 1 2>/dev/null || echo "[]")
    if [ "$WORKFLOW_RUN" != "[]" ] && [ "$WORKFLOW_RUN" != "" ]; then
        break
    fi
    sleep 1
done

if [ "$WORKFLOW_RUN" = "[]" ] || [ "$WORKFLOW_RUN" = "" ]; then
    echo "❌ No workflow found for this commit"
    exit 1
fi

RUN_ID=$(echo "$WORKFLOW_RUN" | jq -r '.[0].databaseId')

echo "🏃 Workflow started (Run ID: $RUN_ID)"
echo "🔗 View in browser: https://github.com/$(gh repo view --json nameWithOwner -q .nameWithOwner)/actions/runs/$RUN_ID"
echo ""

# Watch the workflow
while true; do
    RUN_INFO=$(gh run view "$RUN_ID" --json status,conclusion,displayTitle)
    STATUS=$(echo "$RUN_INFO" | jq -r '.status')
    CONCLUSION=$(echo "$RUN_INFO" | jq -r '.conclusion')

    if [ "$STATUS" = "completed" ]; then
        echo ""
        if [ "$CONCLUSION" = "success" ]; then
            echo "✅ CI build passed!"
            exit 0
        else
            echo "❌ CI build failed!"
            echo ""
            echo "📥 Downloading logs..."

            # Create temp directory for logs
            LOG_DIR="./temp/logs_${RUN_ID}"
            mkdir -p "$LOG_DIR"

            # Download logs
            gh run download "$RUN_ID" --dir "$LOG_DIR" 2>/dev/null || {
                echo "⚠️  Could not download logs automatically"
                echo "Download manually from: https://github.com/$(gh repo view --json nameWithOwner -q .nameWithOwner)/actions/runs/$RUN_ID"
                exit 1
            }

            echo "✅ Logs downloaded to: $LOG_DIR"
            echo ""
            echo "🔍 Checking for test failures..."

            # Find and display the test log
            TEST_LOG=$(find "$LOG_DIR" -name "*Test*.txt" | head -1)
            if [ -n "$TEST_LOG" ]; then
                echo "📄 Test log: $TEST_LOG"
                echo ""
                echo "Last 50 lines of test output:"
                echo "════════════════════════════════════════════════════════"
                tail -50 "$TEST_LOG"
                echo "════════════════════════════════════════════════════════"
            fi

            exit 1
        fi
    fi

    # Print status with spinner
    printf "\r⏳ Status: $STATUS ... "
    sleep 5
done
