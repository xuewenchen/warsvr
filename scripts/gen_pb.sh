#!/bin/bash
set -e
ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

PROTO_DIR="protocol/proto"
OUT_DIR="protocol/pb"

echo "=== Generating protobuf Go code ==="

find "$PROTO_DIR" -name '*.proto' -print0 | while IFS= read -r -d '' proto; do
    rel=$(realpath --relative-to="$PROTO_DIR" "$proto")
    echo "  $rel"
    protoc \
        --proto_path="$PROTO_DIR" \
        --go_out="$OUT_DIR" \
        --go_opt=paths=source_relative \
        --go-grpc_out="$OUT_DIR" \
        --go-grpc_opt=paths=source_relative \
        "$proto"
done

echo "=== Done ==="
