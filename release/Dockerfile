# Release Dockerfile for GoReleaser.
# Binary is built by GoReleaser and copied in via COPY.
# otelcol comes from the official multi-arch image — no qemu needed.

FROM otel/opentelemetry-collector:0.146.1 AS otelcol

FROM alpine:3.21

ARG TARGETPLATFORM
COPY --from=otelcol /otelcol /usr/local/bin/otelcol
COPY ${TARGETPLATFORM}/otlp-mcp /usr/local/bin/otlp-mcp
COPY otel-config.yaml /etc/otel/config.yaml
COPY entrypoint.sh /entrypoint.sh

RUN addgroup -g 4317 -S otlp && adduser -u 4317 -S otlp -G otlp

USER otlp

ENV MCP_PORT=9912
ENV OTLP_PORT=4317

EXPOSE 4317 4318 9912

ENTRYPOINT ["/entrypoint.sh"]
