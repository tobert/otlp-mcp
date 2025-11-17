#!/bin/bash

# Integration test for otlp-mcp configuration file feature
# Tests various configuration scenarios and precedence rules

set -e  # Exit on error

echo "🧪 Testing otlp-mcp configuration file feature"
echo "=============================================="

# Build the binary first
echo "📦 Building otlp-mcp..."
go build -o otlp-mcp ./cmd/otlp-mcp || exit 1

# Helper function to extract port from verbose output
extract_port() {
    echo "$1" | grep "OTLP bind:" | sed 's/.*://' | tr -d ' '
}

# Helper function to extract buffer size from verbose output
extract_trace_buffer() {
    echo "$1" | grep "Trace buffer:" | head -1 | sed 's/.*: //' | sed 's/ spans//' | tr -d ' '
}

# Test 1: Default configuration (no config file)
echo ""
echo "Test 1: Default configuration (no config file)"
echo "-----------------------------------------------"
# Move to tmp to avoid picking up project config
cd /tmp
OUTPUT=$($OLDPWD/otlp-mcp serve --verbose 2>&1 | head -20)
PORT=$(extract_port "$OUTPUT")
TRACE_BUF=$(extract_trace_buffer "$OUTPUT")
cd - > /dev/null

if [ "$PORT" = "0" ]; then
    echo "✅ Default: Using ephemeral port (0)"
else
    echo "❌ Expected ephemeral port (0), got: $PORT"
    exit 1
fi

if [ "$TRACE_BUF" = "10000" ]; then
    echo "✅ Default: Trace buffer is 10000"
else
    echo "❌ Expected trace buffer 10000, got: $TRACE_BUF"
    exit 1
fi

# Test 2: Project config file (.otlp-mcp.json)
echo ""
echo "Test 2: Project config file (.otlp-mcp.json)"
echo "---------------------------------------------"
# Create test project config
cat > .otlp-mcp.json <<EOF
{
  "comment": "Test project config",
  "otlp_port": 4317,
  "trace_buffer_size": 15000
}
EOF

OUTPUT=$(./otlp-mcp serve --verbose 2>&1 | head -20)
PORT=$(extract_port "$OUTPUT")
TRACE_BUF=$(extract_trace_buffer "$OUTPUT")

if [ "$PORT" = "4317" ]; then
    echo "✅ Project config: Using port 4317"
else
    echo "❌ Expected port 4317, got: $PORT"
    exit 1
fi

if [ "$TRACE_BUF" = "15000" ]; then
    echo "✅ Project config: Trace buffer is 15000"
else
    echo "❌ Expected trace buffer 15000, got: $TRACE_BUF"
    exit 1
fi

# Test 3: CLI flag overrides config file
echo ""
echo "Test 3: CLI flag overrides config file"
echo "---------------------------------------"
OUTPUT=$(./otlp-mcp serve --otlp-port 5555 --trace-buffer-size 20000 --verbose 2>&1 | head -20)
PORT=$(extract_port "$OUTPUT")
TRACE_BUF=$(extract_trace_buffer "$OUTPUT")

if [ "$PORT" = "5555" ]; then
    echo "✅ CLI override: Using port 5555"
else
    echo "❌ Expected port 5555, got: $PORT"
    exit 1
fi

if [ "$TRACE_BUF" = "20000" ]; then
    echo "✅ CLI override: Trace buffer is 20000"
else
    echo "❌ Expected trace buffer 20000, got: $TRACE_BUF"
    exit 1
fi

# Test 4: Explicit config file via --config flag
echo ""
echo "Test 4: Explicit config file via --config flag"
echo "-----------------------------------------------"
cat > /tmp/custom-config.json <<EOF
{
  "comment": "Custom test config",
  "otlp_port": 9999,
  "trace_buffer_size": 30000,
  "log_buffer_size": 150000
}
EOF

OUTPUT=$(./otlp-mcp serve --config /tmp/custom-config.json --verbose 2>&1 | head -20)
PORT=$(extract_port "$OUTPUT")
TRACE_BUF=$(extract_trace_buffer "$OUTPUT")

if [ "$PORT" = "9999" ]; then
    echo "✅ Custom config: Using port 9999"
else
    echo "❌ Expected port 9999, got: $PORT"
    exit 1
fi

if [ "$TRACE_BUF" = "30000" ]; then
    echo "✅ Custom config: Trace buffer is 30000"
else
    echo "❌ Expected trace buffer 30000, got: $TRACE_BUF"
    exit 1
fi

# Test 5: Config file with comment field
echo ""
echo "Test 5: Config file with comment field"
echo "---------------------------------------"
cat > /tmp/comment-config.json <<EOF
{
  "comment": "This is a test configuration with documentation",
  "otlp_port": 7777
}
EOF

OUTPUT=$(./otlp-mcp serve --config /tmp/comment-config.json --verbose 2>&1 | head -20)
PORT=$(extract_port "$OUTPUT")

if [ "$PORT" = "7777" ]; then
    echo "✅ Comment field: Config loads correctly with comment"
else
    echo "❌ Failed to load config with comment field"
    exit 1
fi

# Test 6: Invalid JSON should fail gracefully
echo ""
echo "Test 6: Invalid JSON handling"
echo "------------------------------"
cat > /tmp/invalid-config.json <<EOF
{
  "otlp_port": 4317
  "missing_comma": true
}
EOF

OUTPUT=$(./otlp-mcp serve --config /tmp/invalid-config.json 2>&1 || true)
if echo "$OUTPUT" | grep -q "failed to parse config file"; then
    echo "✅ Invalid JSON: Proper error message"
else
    echo "❌ Expected parse error for invalid JSON"
    exit 1
fi

# Test 7: Nonexistent config file with --config should fail
echo ""
echo "Test 7: Nonexistent config file handling"
echo "-----------------------------------------"
OUTPUT=$(./otlp-mcp serve --config /tmp/nonexistent-config.json 2>&1 || true)
if echo "$OUTPUT" | grep -q "failed to read config file"; then
    echo "✅ Nonexistent file: Proper error message"
else
    echo "❌ Expected read error for nonexistent file"
    exit 1
fi

# Clean up test files
rm -f .otlp-mcp.json /tmp/custom-config.json /tmp/comment-config.json /tmp/invalid-config.json

echo ""
echo "=============================================="
echo "🎉 All configuration tests passed!"
echo ""
echo "Summary:"
echo "- Default configuration works"
echo "- Project config file is loaded correctly"
echo "- CLI flags override config files"
echo "- Custom config paths work"
echo "- Comment fields are handled"
echo "- Invalid configs fail gracefully"
echo "- Missing configs fail gracefully"