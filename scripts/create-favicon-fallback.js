#!/usr/bin/env node

// This creates a simple message about favicon.ico
const fs = require('fs');
const path = require('path');

const message = `
The modern approach for Pulse uses SVG favicons, which are supported by all modern browsers.

If you need favicon.ico for legacy support:
1. Use an online converter to convert logo.svg to favicon.ico
2. Or install ImageMagick and run:
   convert -background none -resize 32x32 logo.svg favicon.ico

For now, the SVG favicon will work for 95%+ of users.
`;

console.log(message);

// Create a placeholder to avoid 404s
const placeholderPath = path.join(__dirname, '../src/public/favicon.ico');
// We'll just copy the logo.svg as browsers will ignore it if they don't support SVG
if (!fs.existsSync(placeholderPath)) {
    // Create empty file to prevent 404s
    fs.writeFileSync(placeholderPath, '');
}