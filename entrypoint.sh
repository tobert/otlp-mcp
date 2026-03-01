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

# shellcheck disable=SC2329  # invoked via trap
cleanup() {
  echo "Shutting down..."
  kill "$OTLP_MCP_PID" 2>/dev/null || true
  kill "$OTELCOL_PID" 2>/dev/null || true
  wait
}
trap cleanup TERM INT

# Start otlp-mcp in background
echo "Starting otlp-mcp (MCP HTTP: :${MCP_PORT}, OTLP gRPC: :${OTLP_PORT})"
# shellcheck disable=SC2086  # intentional word splitting on CONFIG_FLAG/STATELESS_FLAG
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

# Wait for otlp-mcp to be ready. 1s is sufficient — it binds quickly and
# otelcol takes longer to initialize than otlp-mcp does to start listening.
sleep 1

# Start otelcol in background
echo "Starting OTel Collector (HTTP :4318 -> gRPC :${OTLP_PORT})"
export OTLP_MCP_ENDPOINT="127.0.0.1:${OTLP_PORT}"
otelcol --config /etc/otel/config.yaml &
OTELCOL_PID=$!

# Wait for either process to exit (POSIX-compatible, no bash wait -n)
while kill -0 "$OTLP_MCP_PID" 2>/dev/null && kill -0 "$OTELCOL_PID" 2>/dev/null; do
  sleep 1
done

# One of the processes has exited; capture its status and then shut down the other.
STATUS=0
if ! kill -0 "$OTLP_MCP_PID" 2>/dev/null; then
  # otlp-mcp exited first
  wait "$OTLP_MCP_PID"
  STATUS=$?
  if kill -0 "$OTELCOL_PID" 2>/dev/null; then
    kill "$OTELCOL_PID" 2>/dev/null || true
    wait "$OTELCOL_PID" 2>/dev/null || true
  fi
elif ! kill -0 "$OTELCOL_PID" 2>/dev/null; then
  # otelcol exited first
  wait "$OTELCOL_PID"
  STATUS=$?
  if kill -0 "$OTLP_MCP_PID" 2>/dev/null; then
    kill "$OTLP_MCP_PID" 2>/dev/null || true
    wait "$OTLP_MCP_PID" 2>/dev/null || true
  fi
fi

exit "$STATUS"
