const fs = require('fs');

// Read the file
const content = fs.readFileSync('/opt/pulse/src/public/js/ui/backups.js', 'utf8');
const lines = content.split('\n');

// Find start and end of _getInitialBackupData
let startLine = -1;
let endLine = -1;
let braceCount = 0;
let inFunction = false;

for (let i = 0; i < lines.length; i++) {
    if (lines[i].includes('// REMOVED: _getInitialBackupData function')) {
        startLine = i + 1; // Start removing from next line
        break;
    }
}

// From startLine, find the matching closing brace at line 1074
// We know it ends at original line 1074, which is now at a different position
for (let i = startLine; i < lines.length; i++) {
    const line = lines[i];
    
    if (line.trim() === '}' && lines[i-1].includes('return result;')) {
        endLine = i;
        break;
    }
}

if (startLine !== -1 && endLine !== -1) {
    // Remove the lines
    lines.splice(startLine, endLine - startLine + 1);
    
    // Write back
    fs.writeFileSync('/opt/pulse/src/public/js/ui/backups.js', lines.join('\n'));
    
    console.log(`Removed ${endLine - startLine + 1} lines of obsolete code`);
} else {
    console.log('Could not find function boundaries');
    console.log('startLine:', startLine, 'endLine:', endLine);
}