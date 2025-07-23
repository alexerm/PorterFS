#!/bin/bash

# PorterFS SSL Connection Test Script
# This script starts PorterFS with SSL and runs connection tests

set -e

PORTER_BIN="./porter"
TEST_CLIENT_BIN="./test-client"
CONFIG_FILE="config-ssl.yaml"
PID_FILE="/tmp/porter-ssl.pid"

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
            log "Stopping PorterFS SSL server (PID: $PID)"
            kill "$PID"
            rm -f "$PID_FILE"
        fi
    fi
    
    # Clean up test data
    if [ -d "./data-ssl" ]; then
        log "Cleaning up SSL test data directory"
        rm -rf ./data-ssl
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

# Check if SSL config exists
if [ ! -f "$CONFIG_FILE" ]; then
    error "SSL config file not found: $CONFIG_FILE"
    exit 1
fi

# Check if SSL certificates exist
if [ ! -f "./certs/server.crt" ] || [ ! -f "./certs/server.key" ]; then
    error "SSL certificates not found. Run the certificate generation first."
    exit 1
fi

# Start PorterFS server with SSL
log "Starting PorterFS server with SSL..."
$PORTER_BIN -config "$CONFIG_FILE" &
SERVER_PID=$!
echo $SERVER_PID > "$PID_FILE"

# Wait for server to start
log "Waiting for SSL server to start..."
sleep 5

# Check if server is running
if ! kill -0 $SERVER_PID 2>/dev/null; then
    error "Failed to start PorterFS SSL server"
    exit 1
fi

log "SSL server started successfully (PID: $SERVER_PID)"

# Run connection tests with SSL
log "Running SSL connection tests..."
echo "=================================================="

# Test with custom credentials
if $TEST_CLIENT_BIN \
    -endpoint="https://localhost:9443" \
    -access-key="porter-test-key" \
    -secret-key="porter-test-secret-key-must-be-long-enough" \
    -bucket="ssl-test-bucket"; then
    echo "=================================================="
    log "All SSL tests passed! ✅"
else
    echo "=================================================="
    error "Some SSL tests failed! ❌"
    exit 1
fi

log "SSL test completed successfully!"

# Show how to use with AWS CLI
echo ""
log "To test with AWS CLI, run:"
echo "aws configure set aws_access_key_id porter-test-key"
echo "aws configure set aws_secret_access_key porter-test-secret-key-must-be-long-enough"
echo "aws --endpoint-url https://localhost:9443 --no-verify-ssl s3 ls"