#!/usr/bin/env bash
set -euo pipefail
version=$(tr -d '\r\n' < VERSION)
[[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Invalid VERSION" >&2; exit 1; }
mkdir -p dist
for arch in amd64 arm64; do
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -ldflags="-s -w -X main.version=$version" -o "dist/greflow-linux-$arch" ./cmd/greflow
done
(cd dist && sha256sum greflow-linux-* > SHA256SUMS)
