// Simple test that actually checks if the currentNodeType is being set
// by looking at the compiled JavaScript

const fs = require('fs');

console.log('Checking if PBS form fix is actually in the code...\n');

// Read the built frontend JS
const jsFile = '/opt/pulse/frontend-modern/dist/assets/index-D2Xfq6EC.js';
const content = fs.readFileSync(jsFile, 'utf8');

// Check if our fix is in the compiled code
// We're looking for setCurrentNodeType being called with node.type
const hasOldBuggyCode = content.includes('setEditingNode(node),setShowNodeModal(!0)');
const hasFixedCode = content.includes('setCurrentNodeType(node.type') || 
                     content.includes('setCurrentNodeType(e.type') ||
                     content.includes('CurrentNodeType(n.type') ||
                     content.includes('CurrentNodeType(t.type');

console.log('Checking compiled JavaScript for the fix...');
console.log('Old buggy pattern found:', hasOldBuggyCode);
console.log('Fixed pattern found:', hasFixedCode);

if (!hasFixedCode) {
    console.log('\n❌ FIX NOT FOUND IN COMPILED CODE!');
    console.log('The fix is not in the production build.');
    console.log('Need to rebuild: cd /opt/pulse/frontend-modern && npm run build');
    process.exit(1);
} else {
    console.log('\n✅ Fix appears to be in the code');
    console.log('The production build contains the currentNodeType fix');
}

// Also check the source to make sure it's there
const sourceFile = '/opt/pulse/frontend-modern/src/components/Settings/Settings.tsx';
const source = fs.readFileSync(sourceFile, 'utf8');

if (source.includes('setCurrentNodeType(node.type')) {
    console.log('✅ Fix confirmed in source code');
} else {
    console.log('❌ Fix NOT in source code!');
    process.exit(1);
}

console.log('\nThe fix is in place, but still needs manual testing to verify it works.');