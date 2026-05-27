#!/bin/bash

# v2node Windows Build Script (run in Linux with mingw-w64)
# 使用方法: chmod +x build-windows.sh && ./build.sh

set -e

VERSION="1.0.2"
BUILD_DIR="$(cd "$(dirname "$0")" && pwd)"
CC=x86_64-w64-mingw32-gcc
CXX=x86_64-w64-mingw32-g++

echo "Building v2node ${VERSION} for Windows (using mingw-w64)..."

# Windows AMD64
echo "Building for windows/amd64..."
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=$CC CXX=$CXX go build \
  -ldflags " \
    -s -w \
    -X github.com/wyx2685/v2node/cmd.version=${VERSION} \
    -linkmode=external \
  " \
  -gcflags="all=-B -l=4" \
  -o "${BUILD_DIR}/v2node.exe" \
  "${BUILD_DIR}"

echo ""
echo "Build completed!"
echo "Output file: ${BUILD_DIR}/v2node.exe"
ls -lh "${BUILD_DIR}/v2node.exe"
