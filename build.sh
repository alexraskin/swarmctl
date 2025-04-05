#!/bin/bash
set -euo pipefail

# Set these variables as needed
IMAGE_NAME="ghcr.io/alexraskin/swarmctl"
version=$(git describe --tags --always)
commit=$(git rev-parse HEAD)
buildTime=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Optional: Login to GHCR (ensure you have a GitHub token with `write:packages`)
# echo $GHCR_TOKEN | docker login ghcr.io -u alexraskin--password-stdin

echo "🛠️  Building Docker image: $IMAGE_NAME:$version"

docker buildx build --build-arg VERSION="$version" --build-arg COMMIT="$commit" --build-arg BUILD_TIME="$buildTime" -t ghcr.io/alexraskin/swarmctl:latest --push .

echo "✅ Successfully built and pushed: $IMAGE_NAME:$version"