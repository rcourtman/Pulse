#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

// Logo sizes commonly needed
const sizes = [16, 32, 48, 64, 128, 192, 256, 512, 1024];
const inputSvg = path.join(__dirname, '../src/public/logos/pulse-logo.svg');
const outputDir = path.join(__dirname, '../src/public/logos');

// Check if we have necessary tools
function checkTools() {
    try {
        // Try to find an SVG converter
        const converters = ['rsvg-convert', 'convert', 'inkscape'];
        for (const tool of converters) {
            try {
                execSync(`which ${tool}`, { stdio: 'ignore' });
                return tool;
            } catch (e) {
                // Tool not found, try next
            }
        }
        return null;
    } catch (e) {
        return null;
    }
}

// Generate PNG using available tool
function generatePng(tool, size) {
    const outputFile = path.join(outputDir, `pulse-logo-${size}x${size}.png`);
    
    try {
        switch(tool) {
            case 'rsvg-convert':
                execSync(`rsvg-convert -w ${size} -h ${size} "${inputSvg}" -o "${outputFile}"`);
                break;
            case 'convert':
                execSync(`convert -background none -density 300 -resize ${size}x${size} "${inputSvg}" "${outputFile}"`);
                break;
            case 'inkscape':
                execSync(`inkscape -w ${size} -h ${size} "${inputSvg}" -o "${outputFile}"`);
                break;
        }
        console.log(`✓ Generated ${size}x${size} PNG`);
    } catch (e) {
        console.error(`✗ Failed to generate ${size}x${size} PNG:`, e.message);
    }
}

// Create favicon formats
function createFavicons() {
    // Copy 16x16 and 32x32 as favicon candidates
    try {
        if (fs.existsSync(path.join(outputDir, 'pulse-logo-16x16.png'))) {
            fs.copyFileSync(
                path.join(outputDir, 'pulse-logo-16x16.png'),
                path.join(outputDir, '..', 'favicon-16x16.png')
            );
        }
        if (fs.existsSync(path.join(outputDir, 'pulse-logo-32x32.png'))) {
            fs.copyFileSync(
                path.join(outputDir, 'pulse-logo-32x32.png'),
                path.join(outputDir, '..', 'favicon-32x32.png')
            );
        }
        console.log('✓ Created favicon files');
    } catch (e) {
        console.error('✗ Failed to create favicon files:', e.message);
    }
}

// Main function
async function main() {
    console.log('🎨 Pulse Logo Generator\n');
    
    // Check if SVG exists
    if (!fs.existsSync(inputSvg)) {
        console.error('✗ Source SVG not found:', inputSvg);
        process.exit(1);
    }
    
    // Check for tools
    const tool = checkTools();
    if (!tool) {
        console.log('⚠️  No SVG conversion tool found.');
        console.log('Please install one of: rsvg-convert (recommended), imagemagick, or inkscape\n');
        console.log('For Ubuntu/Debian: sudo apt-get install librsvg2-bin');
        console.log('For macOS: brew install librsvg');
        
        // Still create necessary copies
        console.log('\n📁 Creating logo structure without PNGs...\n');
    } else {
        console.log(`✓ Found converter: ${tool}\n`);
        console.log('📁 Generating PNG files...\n');
        
        // Generate all sizes
        for (const size of sizes) {
            generatePng(tool, size);
        }
    }
    
    // Copy SVG to root for easy access
    fs.copyFileSync(inputSvg, path.join(outputDir, '..', 'logo.svg'));
    console.log('\n✓ Copied logo.svg to public root');
    
    // Create favicon files
    createFavicons();
    
    // Create manifest icons list
    const manifestIcons = [192, 512].map(size => ({
        src: `/logos/pulse-logo-${size}x${size}.png`,
        sizes: `${size}x${size}`,
        type: 'image/png'
    }));
    
    console.log('\n📋 Manifest icons configuration:');
    console.log(JSON.stringify(manifestIcons, null, 2));
    
    console.log('\n✅ Logo generation complete!\n');
}

main().catch(console.error);