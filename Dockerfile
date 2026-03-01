# Stage 1: Build otlp-mcp from local source
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /otlp-mcp ./cmd/otlp-mcp

# Stage 2: Get otelcol binary from official image
FROM otel/opentelemetry-collector:0.146.1 AS otelcol

# Stage 3: Final image
FROM alpine:3.21

COPY --from=builder /otlp-mcp /usr/local/bin/otlp-mcp
COPY --from=otelcol /otelcol /usr/local/bin/otelcol
COPY otel-config.yaml /etc/otel/config.yaml
COPY entrypoint.sh /entrypoint.sh

RUN addgroup -g 4317 -S otlp && adduser -u 4317 -S otlp -G otlp
USER otlp

ENV MCP_PORT=9912
ENV OTLP_PORT=4317

EXPOSE 4317 4318 9912

ENTRYPOINT ["/entrypoint.sh"]
