#!/bin/sh
# Build and push a release of otlp-mcp.
#
# Prerequisites:
#   - goreleaser (go install github.com/goreleaser/goreleaser/v2@latest)
#   - podman (or docker)
#   - logged into ghcr.io (echo $TOKEN | podman login ghcr.io -u USERNAME --password-stdin)
#   - GITHUB_TOKEN set or ~/.gh-packages-key exists
#
# Usage:
#   git tag v0.5.0
#   git push origin v0.5.0
#   ./release/do-release.sh

set -eu

REGISTRY="ghcr.io/tobert/otlp-mcp"
PLATFORMS="linux/amd64 linux/arm64"

# Resolve container runtime.
if command -v podman >/dev/null 2>&1; then
  CTR=podman
elif command -v docker >/dev/null 2>&1; then
  CTR=docker
else
  echo "Error: podman or docker required" >&2
  exit 1
fi

# Resolve GitHub token.
if [ -z "${GITHUB_TOKEN:-}" ]; then
  if [ -f "$HOME/.gh-packages-key" ]; then
    GITHUB_TOKEN=$(cat "$HOME/.gh-packages-key")
    export GITHUB_TOKEN
  else
    echo "Error: GITHUB_TOKEN not set and ~/.gh-packages-key not found" >&2
    exit 1
  fi
fi

# Get version from latest git tag.
TAG=$(git describe --tags --exact-match 2>/dev/null || true)
if [ -z "$TAG" ]; then
  echo "Error: HEAD is not tagged. Tag first: git tag v0.x.y" >&2
  exit 1
fi
echo "==> Releasing ${TAG}"

# Step 1: goreleaser builds binaries, packages, and creates GitHub release.
echo "==> Running goreleaser..."
goreleaser release --clean --skip=docker

# Step 2: Set up Docker build context from goreleaser output.
CONTEXT=$(mktemp -d)
trap 'rm -rf "$CONTEXT"' EXIT

cp dist/otlp-mcp_linux_amd64_v1/otlp-mcp "$CONTEXT/linux-amd64"
cp dist/otlp-mcp_linux_arm64_v8.0/otlp-mcp "$CONTEXT/linux-arm64"
cp otel-config.yaml entrypoint.sh "$CONTEXT/"
cp release/Dockerfile "$CONTEXT/Dockerfile"

# Create platform directories matching TARGETPLATFORM layout.
mkdir -p "$CONTEXT/linux/amd64" "$CONTEXT/linux/arm64"
cp "$CONTEXT/linux-amd64" "$CONTEXT/linux/amd64/otlp-mcp"
cp "$CONTEXT/linux-arm64" "$CONTEXT/linux/arm64/otlp-mcp"

# Step 3: Build per-arch images.
echo "==> Building container images..."
for PLAT in $PLATFORMS; do
  ARCH_TAG="${TAG}-$(echo "$PLAT" | tr '/' '-')"
  echo "    ${REGISTRY}:${ARCH_TAG}"
  $CTR build \
    --platform "$PLAT" \
    --build-arg "TARGETPLATFORM=$PLAT" \
    -f "$CONTEXT/Dockerfile" \
    -t "${REGISTRY}:${ARCH_TAG}" \
    "$CONTEXT"
done

# Step 4: Push per-arch images.
echo "==> Pushing images..."
for PLAT in $PLATFORMS; do
  ARCH_TAG="${TAG}-$(echo "$PLAT" | tr '/' '-')"
  $CTR push "${REGISTRY}:${ARCH_TAG}"
done

# Step 5: Create and push multi-arch manifest.
echo "==> Creating manifest ${REGISTRY}:${TAG}..."
$CTR manifest create "${REGISTRY}:${TAG}" \
  "${REGISTRY}:${TAG}-linux-amd64" \
  "${REGISTRY}:${TAG}-linux-arm64"
$CTR manifest push "${REGISTRY}:${TAG}" "docker://${REGISTRY}:${TAG}"

echo "==> Creating manifest ${REGISTRY}:latest..."
$CTR manifest create "${REGISTRY}:latest" \
  "${REGISTRY}:${TAG}-linux-amd64" \
  "${REGISTRY}:${TAG}-linux-arm64"
$CTR manifest push "${REGISTRY}:latest" "docker://${REGISTRY}:latest"

echo "==> Done! Release ${TAG} published."
echo "    GitHub: https://github.com/tobert/otlp-mcp/releases/tag/${TAG}"
echo "    Docker: ${REGISTRY}:${TAG}"
