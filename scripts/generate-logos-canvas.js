#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

// Logo structure for reference
const logoInfo = {
    sizes: {
        favicon: [16, 32, 48],
        app: [64, 128, 192, 256, 512],
        highRes: [1024]
    },
    uses: {
        '16x16': 'Browser tab favicon',
        '32x32': 'High-DPI favicon', 
        '48x48': 'Windows taskbar',
        '64x64': 'macOS dock (small)',
        '128x128': 'macOS dock (medium)',
        '192x192': 'Android home screen',
        '256x256': 'Windows tiles, macOS apps',
        '512x512': 'PWA splash screens',
        '1024x1024': 'App store submissions'
    }
};

console.log('🎨 Pulse Logo Structure\n');
console.log('Since image conversion tools are not available, here\'s what you need:\n');

console.log('📁 Logo files to generate from pulse-logo.svg:\n');

Object.entries(logoInfo.sizes).forEach(([category, sizes]) => {
    console.log(`${category.charAt(0).toUpperCase() + category.slice(1)}:`);
    sizes.forEach(size => {
        console.log(`  - pulse-logo-${size}x${size}.png (${logoInfo.uses[`${size}x${size}`] || 'General use'})`);
    });
    console.log('');
});

console.log('🛠️  To generate these, you can:\n');
console.log('1. Install rsvg-convert (recommended):');
console.log('   sudo apt-get install librsvg2-bin\n');
console.log('2. Use an online converter like:');
console.log('   - https://cloudconvert.com/svg-to-png');
console.log('   - https://convertio.co/svg-png/\n');
console.log('3. Use a design tool (Figma, Inkscape, etc.)\n');

// Create a shell script for when tools are available
const shellScript = `#!/bin/bash

# Pulse Logo Generator
# Run this after installing rsvg-convert

SIZES=(16 32 48 64 128 192 256 512 1024)
INPUT_SVG="src/public/logos/pulse-logo.svg"
OUTPUT_DIR="src/public/logos"

echo "🎨 Generating Pulse logos..."

for size in "\${SIZES[@]}"; do
    rsvg-convert -w $size -h $size "$INPUT_SVG" -o "$OUTPUT_DIR/pulse-logo-\${size}x\${size}.png"
    echo "✓ Generated \${size}x\${size}"
done

# Copy favicons
cp "$OUTPUT_DIR/pulse-logo-16x16.png" "src/public/favicon-16x16.png"
cp "$OUTPUT_DIR/pulse-logo-32x32.png" "src/public/favicon-32x32.png"
cp "$OUTPUT_DIR/pulse-logo-32x32.png" "src/public/favicon.png"

echo "✅ Done!"
`;

const scriptPath = path.join(__dirname, 'generate-logos.sh');
fs.writeFileSync(scriptPath, shellScript);
fs.chmodSync(scriptPath, '755');

console.log(`✅ Created ${scriptPath}`);
console.log('   Run it with: ./scripts/generate-logos.sh\n');

// Update the old logo.svg to match the new one
const srcLogo = path.join(__dirname, '../src/public/logos/pulse-logo.svg');
const destLogo = path.join(__dirname, '../src/public/logo.svg');

// The logo.svg is already updated, but let's ensure consistency
console.log('📋 Current logo locations:');
console.log(`   - ${destLogo} (main)`);
console.log(`   - ${srcLogo} (source)`);
console.log('   - /favicon.svg (should be linked)\n');

console.log('💡 For the GitHub discussion response:');
console.log('   You now have a scalable SVG that can be rendered at any size.');
console.log('   High-res PNGs can be generated from it as needed.\n');