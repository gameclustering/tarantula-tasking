#!/usr/bin/env bash
# Run from repo root: ./generate-proto.sh
# Requires: protoc, protoc-gen-go, protoc-gen-go-grpc
set -euo pipefail

PROTO_DIR="../tarantula-protocol"
OUT_DIR="."
MODULE="gameclustering.com"

for tool in protoc protoc-gen-go protoc-gen-go-grpc; do
    if ! command -v "$tool" &>/dev/null; then
        echo "Missing tool: $tool"
        echo "Install with:"
        echo "  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
        echo "  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
        exit 1
    fi
done

echo "Generating from $PROTO_DIR -> internal/protocol/ ..."

protoc \
    --proto_path="$PROTO_DIR" \
    --go_out="$OUT_DIR" \
    --go_opt="module=$MODULE" \
    --go-grpc_out="$OUT_DIR" \
    --go-grpc_opt="module=$MODULE" \
    "$PROTO_DIR"/*.proto

echo "Done. Files written to internal/protocol/"
