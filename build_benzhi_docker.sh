#!/bin/bash
# Build the collation workbench image for a given platform and print the
# verification command. Usage: bash build_benzhi_docker.sh <镜像名> <平台>
set -e

IMAGE_NAME=${1:-task165-collation}
DOCKER_PLATFORM=${2:-linux/amd64}

docker build --platform "$DOCKER_PLATFORM" -f benzhi.Dockerfile -t "$IMAGE_NAME" .

echo ""
echo "✅ Docker image '$IMAGE_NAME' built successfully for $DOCKER_PLATFORM!"
echo ""
echo "📋 Verify (must print 'smoke test passed'):"
echo "  • docker run --rm $IMAGE_NAME /app/collation --smoke-test"
echo ""
