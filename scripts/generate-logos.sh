#!/bin/bash

# Pulse Logo Generator
# Run this after installing rsvg-convert

SIZES=(16 32 48 64 128 192 256 512 1024)
INPUT_SVG="src/public/logos/pulse-logo.svg"
OUTPUT_DIR="src/public/logos"

echo "🎨 Generating Pulse logos..."

for size in "${SIZES[@]}"; do
    rsvg-convert -w $size -h $size "$INPUT_SVG" -o "$OUTPUT_DIR/pulse-logo-${size}x${size}.png"
    echo "✓ Generated ${size}x${size}"
done

# Copy favicons
cp "$OUTPUT_DIR/pulse-logo-16x16.png" "src/public/favicon-16x16.png"
cp "$OUTPUT_DIR/pulse-logo-32x32.png" "src/public/favicon-32x32.png"
cp "$OUTPUT_DIR/pulse-logo-32x32.png" "src/public/favicon.png"

echo "✅ Done!"
