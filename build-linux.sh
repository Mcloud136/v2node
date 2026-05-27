#!/bin/bash

# v2node Linux Build Script
# 使用方法: chmod +x build.sh && ./build.sh

set -e

VERSION="1.0.2"
BUILD_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "Building v2node ${VERSION} for Linux..."

# Linux AMD64 (x86_64)
echo "Building for linux/amd64..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build \
  -ldflags " \
    -s -w \
    -X github.com/wyx2685/v2node/cmd.version=${VERSION} \
    -linkmode=external \
  " \
  -gcflags="all=-B -l=4" \
  -o "${BUILD_DIR}/v2node-linux-amd64" \
  "${BUILD_DIR}"

# Linux ARM64 (aarch64)
echo "Building for linux/arm64..."
GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build \
  -ldflags " \
    -s -w \
    -X github.com/wyx2685/v2node/cmd.version=${VERSION} \
    -linkmode=external \
  " \
  -gcflags="all=-B -l=4" \
  -o "${BUILD_DIR}/v2node-linux-arm64" \
  "${BUILD_DIR}"

echo ""
echo "Build completed!"
echo "Output files:"
ls -lh "${BUILD_DIR}/v2node-linux-"*

echo ""
echo "Recommended runtime settings for multi-threading:"
echo "  export GOMAXPROCS=0  # Auto-detect CPU cores"
echo "  ./v2node-linux-amd64"
