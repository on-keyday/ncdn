#!/bin/bash
SCRIPT_DIR=$(dirname "$(realpath "$0")")
echo "$SCRIPT_DIR"
OUTPUT_DIR="$SCRIPT_DIR/../popcache/wasm"
cd "$SCRIPT_DIR" || exit 1
cargo build --release --target wasm32-wasip1 
if [ $? -ne 0 ]; then
    echo "Cargo build failed"
    exit 1
fi
mkdir -p "$OUTPUT_DIR"
cp target/wasm32-wasip1/release/edge_app.wasm "$OUTPUT_DIR/"
if [ $? -ne 0 ]; then
    echo "Failed to copy wasm file"
    exit 1
fi
echo "WASM file built and copied to $OUTPUT_DIR"