#!/usr/bin/env node

const { detectVmidCollisions, analyzePbsConfiguration } = require('./server/pbsUtils');

// Test data simulating VMID collisions
const testSnapshots = [
    // VM 100 from node1
    {
        'backup-type': 'vm',
        'backup-id': '100',
        'backup-time': 1751000000,
        'comment': 'test-vm, node1, 100',
        'namespace': ''
    },
    // VM 100 from node2 (newer)
    {
        'backup-type': 'vm',
        'backup-id': '100',
        'backup-time': 1751001000,
        'comment': 'web-server, node2, 100',
        'namespace': ''
    },
    // CT 101 from node1
    {
        'backup-type': 'ct',
        'backup-id': '101',
        'backup-time': 1751000000,
        'comment': '',
        'namespace': ''
    },
    // CT 101 from node2
    {
        'backup-type': 'ct',
        'backup-id': '101',
        'backup-time': 1751001000,
        'comment': 'different source',
        'namespace': ''
    }
];

console.log('Testing VMID collision detection...\n');

// Test collision detection
const collisionResult = detectVmidCollisions(testSnapshots, '');
console.log('Collision Detection Result:');
console.log('Has Collisions:', collisionResult.hasCollisions);
console.log('\nCollisions Found:');
collisionResult.collisions.forEach(collision => {
    console.log(`- ${collision.vmid}: ${collision.totalSnapshots} snapshots from ${collision.sources.length} sources`);
    collision.sources.forEach(source => {
        console.log(`  * Source: "${source.comment}" (${source.count} backups)`);
    });
});

console.log('\nWarnings:');
collisionResult.warnings.forEach(warning => {
    console.log(`- [${warning.severity.toUpperCase()}] ${warning.message}`);
});

// Test configuration analysis
const testDatastores = [{
    name: 'main',
    snapshots: testSnapshots
}];

console.log('\n\nTesting PBS Configuration Analysis...\n');
const configAnalysis = analyzePbsConfiguration(testDatastores);
console.log('Configuration Analysis:');
console.log('Has Namespaces:', configAnalysis.hasNamespaces);
console.log('Total Collisions:', configAnalysis.totalCollisions);
console.log('\nRecommendations:');
configAnalysis.recommendations.forEach(rec => {
    console.log(`- [${rec.severity.toUpperCase()}] ${rec.message}`);
    console.log(`  Action: ${rec.action}`);
});