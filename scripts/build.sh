#!/usr/bin/env bash
set -euo pipefail
mkdir -p dist
for arch in amd64 arm64; do
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o "dist/greflow-linux-$arch" ./cmd/greflow
done
(cd dist && sha256sum greflow-linux-* > SHA256SUMS)
