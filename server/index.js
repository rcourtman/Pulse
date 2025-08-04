// Pulse v4 Migration Shim
// This file prevents v3 from running after downloading v4 files

console.error('\n========================================');
console.error('PULSE V4 MIGRATION REQUIRED');
console.error('========================================\n');
console.error('Your Pulse installation has been partially updated to v4.');
console.error('Pulse v4 is a complete rewrite in Go and requires manual migration.\n');
console.error('To complete the migration:');
console.error('1. Stop the service: systemctl stop pulse.service');
console.error('2. Run the installer: /opt/pulse/install.sh');
console.error('3. OR create a fresh installation in a new container\n');
console.error('Your existing configuration is preserved in /opt/pulse/.env\n');
console.error('For more information: https://github.com/rcourtman/Pulse/releases/v4.0.0');
console.error('========================================\n');

// Exit with error to prevent service from running
process.exit(1);