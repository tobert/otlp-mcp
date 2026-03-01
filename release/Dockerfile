# Release Dockerfile for GoReleaser / do-release.sh.
# Go binary is cross-compiled externally. otelcol comes from the official
# multi-arch image. No qemu needed — the only RUN is in a native-arch stage.

FROM otel/opentelemetry-collector:0.146.1 AS otelcol

# Create user files in a native-arch stage (avoids qemu for cross builds).
# BUILDPLATFORM is always the host arch, so this RUN executes natively.
ARG BUILDPLATFORM
FROM --platform=${BUILDPLATFORM} alpine:3.21 AS usersetup
RUN addgroup -g 4317 -S otlp && adduser -u 4317 -S otlp -G otlp

# Final image — no RUN, so --platform works without qemu.
FROM alpine:3.21

ARG TARGETPLATFORM

COPY --from=usersetup /etc/passwd /etc/passwd
COPY --from=usersetup /etc/group /etc/group
COPY --from=otelcol /otelcol /usr/local/bin/otelcol
COPY ${TARGETPLATFORM}/otlp-mcp /usr/local/bin/otlp-mcp
COPY otel-config.yaml /etc/otel/config.yaml
COPY entrypoint.sh /entrypoint.sh

USER otlp

ENV MCP_PORT=9912
ENV OTLP_PORT=4317

VOLUME /logs
EXPOSE 4317 4318 9912

ENTRYPOINT ["/entrypoint.sh"]
