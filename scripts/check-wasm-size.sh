#!/bin/bash

WASM_FILE="sdk/examples/wasm/birb-nest-sdk.wasm"

if [ ! -f "$WASM_FILE" ]; then
    echo "❌ WASM module not found. Run 'make sdk-wasm' first."
    exit 1
fi

# Get file size in bytes
SIZE=$(ls -l "$WASM_FILE" | awk '{print $5}')

# Calculate size in MB
SIZE_MB=$(echo "scale=2; $SIZE / 1048576" | bc)

# Display size
echo "   WASM module size: ${SIZE_MB}MB ($SIZE bytes)"

# Check if size exceeds 10MB (10485760 bytes)
if [ $SIZE -gt 10485760 ]; then
    echo "⚠️  WARNING: WASM module exceeds 10MB target!"
else
    echo "✅ WASM module is within 10MB target"
fi
