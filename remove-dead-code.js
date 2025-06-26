const fs = require('fs');

// Read the file
const content = fs.readFileSync('/opt/pulse/src/public/js/ui/backups.js', 'utf8');
const lines = content.split('\n');

// Find the line with "return;" after "Failed to load backup data"
let startLine = -1;
for (let i = 0; i < lines.length; i++) {
    if (lines[i].includes('Failed to load backup data')) {
        // Find the return statement after this
        for (let j = i; j < i + 10 && j < lines.length; j++) {
            if (lines[j].trim() === 'return;') {
                startLine = j + 1;
                break;
            }
        }
        break;
    }
}

// Find the end of the updateBackupsTab function
let endLine = -1;
let braceLevel = 0;
let foundFunctionStart = false;

// First find the function declaration
for (let i = 0; i < startLine; i++) {
    if (lines[i].includes('async function updateBackupsTab')) {
        foundFunctionStart = true;
        // Count opening brace
        if (lines[i].includes('{')) braceLevel = 1;
        break;
    }
}

// Now find the closing brace
if (foundFunctionStart && startLine !== -1) {
    for (let i = startLine; i < lines.length; i++) {
        const line = lines[i];
        // Count braces
        for (let char of line) {
            if (char === '{') braceLevel++;
            if (char === '}') braceLevel--;
        }
        
        if (braceLevel === 0) {
            endLine = i - 1; // Don't remove the closing brace
            break;
        }
    }
}

if (startLine !== -1 && endLine !== -1 && startLine < endLine) {
    console.log(`Removing dead code from line ${startLine} to ${endLine}`);
    console.log(`Removing ${endLine - startLine + 1} lines`);
    
    // Remove the lines
    lines.splice(startLine, endLine - startLine + 1);
    
    // Write back
    fs.writeFileSync('/opt/pulse/src/public/js/ui/backups.js', lines.join('\n'));
    
    console.log('Dead code removed successfully');
} else {
    console.log('Could not find dead code boundaries');
    console.log('startLine:', startLine, 'endLine:', endLine);
}