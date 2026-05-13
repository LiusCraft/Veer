#!/bin/bash
set -e

echo "=== Building Veer manager ==="

cd backend
go mod tidy
go build -ldflags="-s -w" -o veer ./cmd/manager

echo ""
echo "=== Build complete: backend/veer ==="
