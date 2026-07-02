#!/usr/bin/env bash
# demo-trace.sh - Generate a complex multi-service trace for the web UI timeline.
#
# Simulates: browser -> api-gateway -> auth-service
#                                   -> product-service -> postgres
#                                                     -> redis (cache miss then fill)
#                                   -> recommendation-service (error)
#                                   -> render-response
#
# Requires: otel-cli (https://github.com/equinixmetal-oss/otel-cli)
#
# Usage: ./test/demo-trace.sh [endpoint]
#   endpoint defaults to OTEL_EXPORTER_OTLP_ENDPOINT or 127.0.0.1:4317

set -euo pipefail

OTELCLI="${OTELCLI:-$HOME/bin/otel-cli}"
ENDPOINT="${1:-${OTEL_EXPORTER_OTLP_ENDPOINT:-127.0.0.1:4317}}"

# Generate random IDs from /dev/urandom
hexid() { head -c "$1" /dev/urandom | od -An -tx1 | tr -d ' \n'; }

TRACE_ID=$(hexid 16)    # 32 hex chars
ID_GATEWAY=$(hexid 8)
ID_AUTH=$(hexid 8)
ID_PRODUCT=$(hexid 8)
ID_PG_QUERY=$(hexid 8)
ID_REDIS_GET=$(hexid 8)
ID_REDIS_SET=$(hexid 8)
ID_RECOMMEND=$(hexid 8)
ID_RENDER=$(hexid 8)

echo "trace_id: $TRACE_ID"
echo "endpoint: $ENDPOINT"

# Timestamps: base = now in nanoseconds, offsets in milliseconds
BASE_NS=$(date +%s%N)
ts() {  # ts 50 = base + 50ms, ts 820 = base + 820ms
  echo $(( BASE_NS + $1 * 1000000 ))
}

# Common flags for every span
C="--endpoint $ENDPOINT --verbose --fail --force-trace-id $TRACE_ID"

# -- api-gateway: root span (820ms total request) --
$OTELCLI span $C \
  --service api-gateway --name "GET /products/featured" --kind server \
  --force-span-id "$ID_GATEWAY" \
  --start "$(ts 0)" --end "$(ts 820)" \
  --status-code ok \
  -a "http.method=GET" \
  -a "http.url=/products/featured" \
  -a "http.status_code=200" \
  -a "net.peer.ip=203.0.113.42" &

# -- auth-service: JWT validation (35ms) --
$OTELCLI span $C \
  --service auth-service --name "validate-jwt" --kind client \
  --force-span-id "$ID_AUTH" --force-parent-span-id "$ID_GATEWAY" \
  --start "$(ts 10)" --end "$(ts 45)" \
  --status-code ok \
  -a "auth.method=jwt" \
  -a "auth.user_id=user-7291" \
  -a "auth.scopes=read:products" &

# -- product-service: data orchestration (600ms) --
$OTELCLI span $C \
  --service product-service --name "get-featured-products" --kind server \
  --force-span-id "$ID_PRODUCT" --force-parent-span-id "$ID_GATEWAY" \
  --start "$(ts 50)" --end "$(ts 650)" \
  --status-code ok \
  -a "product.category=featured" \
  -a "product.limit=12" \
  -a "product.result_count=12" &

# -- postgres: SQL query (250ms) --
$OTELCLI span $C \
  --service postgres --name "SELECT products" --kind client \
  --force-span-id "$ID_PG_QUERY" --force-parent-span-id "$ID_PRODUCT" \
  --start "$(ts 60)" --end "$(ts 310)" \
  --status-code ok \
  -a "db.system=postgresql" \
  -a "db.name=catalog" \
  -a "db.operation=SELECT" \
  -a "db.sql.table=products" \
  -a "db.rows_affected=12" &

# -- redis: cache GET (3ms, miss) --
$OTELCLI span $C \
  --service redis --name "GET featured:prices" --kind client \
  --force-span-id "$ID_REDIS_GET" --force-parent-span-id "$ID_PRODUCT" \
  --start "$(ts 55)" --end "$(ts 58)" \
  --status-code ok \
  -a "db.system=redis" \
  -a "db.operation=GET" \
  -a "db.redis.key=featured:prices" \
  -a "cache.hit=false" &

# -- redis: cache SET (5ms, backfill after PG query completes) --
$OTELCLI span $C \
  --service redis --name "SET featured:prices" --kind client \
  --force-span-id "$ID_REDIS_SET" --force-parent-span-id "$ID_PRODUCT" \
  --start "$(ts 320)" --end "$(ts 325)" \
  --status-code ok \
  -a "db.system=redis" \
  -a "db.operation=SET" \
  -a "db.redis.key=featured:prices" \
  -a "db.redis.ttl_seconds=300" &

# -- recommendation-svc: ERROR (500ms of retries then failure) --
$OTELCLI span $C \
  --service recommendation-svc --name "get-recommendations" --kind client \
  --force-span-id "$ID_RECOMMEND" --force-parent-span-id "$ID_GATEWAY" \
  --start "$(ts 50)" --end "$(ts 550)" \
  --status-code error --status-description "connection refused: ml-model-server:8080" \
  -a "rpc.system=grpc" \
  -a "rpc.service=RecommendationService" \
  -a "rpc.method=GetPersonalized" \
  -a "rpc.grpc.status_code=14" \
  -a "error.type=ConnectionRefused" \
  -a "retry.count=3" \
  -a "retry.max_backoff_ms=500" &

# -- api-gateway: render response (150ms, after data ready) --
$OTELCLI span $C \
  --service api-gateway --name "render-response" --kind internal \
  --force-span-id "$ID_RENDER" --force-parent-span-id "$ID_GATEWAY" \
  --start "$(ts 660)" --end "$(ts 810)" \
  --status-code ok \
  -a "template.name=product-grid" \
  -a "template.items=12" \
  -a "response.size_bytes=24832" &

wait
echo "Done. Open the web UI and expand the trace to see the timeline."
