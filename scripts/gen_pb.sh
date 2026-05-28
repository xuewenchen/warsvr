#!/bin/bash
set -e
ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

PROTO_DIR="protocol/proto"
OUT_DIR="protocol/pb"

echo "=== Generating protobuf Go code ==="

for proto in "$PROTO_DIR"/*.proto; do
  name=$(basename "$proto")
  echo "  $name"
  protoc \
    --proto_path="$PROTO_DIR" \
    --go_out="$OUT_DIR" \
    --go_opt=paths=source_relative \
    "$proto"
done

echo "=== Done ==="
