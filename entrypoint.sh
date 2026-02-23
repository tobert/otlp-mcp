#!/bin/sh
set -e

MCP_PORT="${MCP_PORT:-9912}"
OTLP_PORT="${OTLP_PORT:-4317}"
STATELESS_FLAG=""
if [ "${STATELESS}" = "1" ]; then
  STATELESS_FLAG="--stateless"
fi

CONFIG_FLAG=""
CONFIG_PATH="/etc/otlp-mcp/config.json"
if [ -f "${CONFIG_PATH}" ]; then
  CONFIG_FLAG="--config ${CONFIG_PATH}"
fi

cleanup() {
  echo "Shutting down..."
  kill "$OTLP_MCP_PID" 2>/dev/null || true
  kill "$OTELCOL_PID" 2>/dev/null || true
  wait
}
trap cleanup TERM INT

# Start otlp-mcp in background
echo "Starting otlp-mcp (MCP HTTP: :${MCP_PORT}, OTLP gRPC: :${OTLP_PORT})"
otlp-mcp serve \
  --transport http \
  --http-host 0.0.0.0 \
  --http-port "${MCP_PORT}" \
  --otlp-host 0.0.0.0 \
  --otlp-port "${OTLP_PORT}" \
  --verbose \
  ${CONFIG_FLAG} \
  ${STATELESS_FLAG} &
OTLP_MCP_PID=$!

# Wait for otlp-mcp to be ready
sleep 1

# Start otelcol in background
echo "Starting OTel Collector (HTTP :4318 -> gRPC :${OTLP_PORT})"
export OTLP_MCP_ENDPOINT="127.0.0.1:${OTLP_PORT}"
otelcol --config /etc/otel/config.yaml &
OTELCOL_PID=$!

# Wait for either process to exit
wait -n "$OTLP_MCP_PID" "$OTELCOL_PID" 2>/dev/null || true
cleanup
