#!/bin/bash
# Demo script for otlp-mcp
# Usage: ./demo.sh <endpoint>
# Example: ./demo.sh 127.0.0.1:38279

ENDPOINT=${1:-127.0.0.1:4317}

# Find otel-cli using Go's GOPATH
if command -v otel-cli >/dev/null 2>&1; then
  CLI="otel-cli"
else
  # Get GOPATH from Go itself
  GOPATH=$(go env GOPATH 2>/dev/null)
  if [ -z "$GOPATH" ]; then
    echo "❌ Error: Go not found or GOPATH not set"
    echo "Please install Go from https://go.dev/dl/"
    exit 1
  fi

  CLI="$GOPATH/bin/otel-cli"

  if [ ! -f "$CLI" ]; then
    echo "❌ otel-cli not found at $CLI"
    echo ""
    echo "Install it by running:"
    echo "  go install github.com/tobert/otel-cli@latest"
    echo ""
    echo "Then make sure $GOPATH/bin is in your PATH:"
    echo "  export PATH=\"\$PATH:$GOPATH/bin\""
    exit 1
  fi
fi

echo "📡 Sending traces to $ENDPOINT"
echo "   Using: $CLI"

# Simulate a web request with database call
"$CLI" span --endpoint $ENDPOINT --protocol grpc --insecure \
  --service "web-api" --name "GET /api/users" --kind server \
  --attrs "http.method=GET,http.status_code=200"

"$CLI" span --endpoint $ENDPOINT --protocol grpc --insecure \
  --service "database" --name "SELECT users" --kind client \
  --attrs "db.system=postgres"

"$CLI" span --endpoint $ENDPOINT --protocol grpc --insecure \
  --service "cache-service" --name "cache.get" --kind client \
  --attrs "cache.hit=true"

echo "✅ Sent 3 test traces!"
echo "💡 In Claude Code, ask: 'Show me recent traces'"
