#!/bin/bash
set -e
echo "=== Building Veer ==="

echo "[1/3] Building frontend..."
cd frontend
npm ci
npm run build
cd ..

echo "[2/3] Copying frontend dist to backend..."
rm -rf backend/dist
cp -r frontend/dist backend/dist

echo "[3/3] Building backend..."
cd backend
go mod tidy
go build -ldflags="-s -w" -o veer .

echo ""
echo "=== Build complete: backend/veer ==="
