const fs = require('fs');
const crypto = require('crypto');
const path = require('path');

// This is a test to see if password is in the encrypted file
// We can't actually decrypt without the key, but we can check file size

const encFile = '/etc/pulse/email.enc';
const stats = fs.statSync(encFile);

console.log('Encrypted email file info:');
console.log('  Size:', stats.size, 'bytes');
console.log('  Modified:', stats.mtime);

// A config with password should be larger than one without
// Typical empty config ~200 bytes, with password ~250+ bytes
if (stats.size < 200) {
  console.log('  ⚠️  File seems too small to contain full config');
} else if (stats.size > 250) {
  console.log('  ✅ File size suggests it may contain password');
} else {
  console.log('  🤔 File size is in between - uncertain');
}
