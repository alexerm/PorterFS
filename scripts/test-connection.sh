#!/bin/bash

# PorterFS Connection Test Script
# This script starts PorterFS server and runs connection tests

set -e

PORTER_BIN="./porter"
TEST_CLIENT_BIN="./test-client"
CONFIG_FILE="config.yaml"
PID_FILE="/tmp/porter.pid"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

cleanup() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if kill -0 "$PID" 2>/dev/null; then
            log "Stopping PorterFS server (PID: $PID)"
            kill "$PID"
            rm -f "$PID_FILE"
        fi
    fi
    
    # Clean up test data
    if [ -d "./data" ]; then
        log "Cleaning up test data directory"
        rm -rf ./data
    fi
}

# Set up cleanup on exit
trap cleanup EXIT

# Check if binaries exist
if [ ! -f "$PORTER_BIN" ]; then
    error "Porter binary not found. Run: go build -o porter ./cmd/porter"
    exit 1
fi

if [ ! -f "$TEST_CLIENT_BIN" ]; then
    log "Building test client..."
    go build -o test-client ./cmd/test-client
fi

# Create config if it doesn't exist
if [ ! -f "$CONFIG_FILE" ]; then
    log "Creating default config file..."
    cp config.yaml.example config.yaml
fi

# Start PorterFS server
log "Starting PorterFS server..."
$PORTER_BIN -config "$CONFIG_FILE" &
SERVER_PID=$!
echo $SERVER_PID > "$PID_FILE"

# Wait for server to start
log "Waiting for server to start..."
sleep 3

# Check if server is running
if ! kill -0 $SERVER_PID 2>/dev/null; then
    error "Failed to start PorterFS server"
    exit 1
fi

log "Server started successfully (PID: $SERVER_PID)"

# Run connection tests
log "Running connection tests..."
echo "=================================================="

if $TEST_CLIENT_BIN -endpoint="http://localhost:9000" -bucket="test-bucket"; then
    echo "=================================================="
    log "All tests passed! ✅"
else
    echo "=================================================="
    error "Some tests failed! ❌"
    exit 1
fi

log "Test completed successfully!"