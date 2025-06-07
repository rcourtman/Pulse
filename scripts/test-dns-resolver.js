#!/usr/bin/env node

/**
 * Test script for the resilient DNS resolver
 * Usage: node test-dns-resolver.js <hostname>
 */

const dnsResolver = require('../server/dnsResolver');

async function testDnsResolution(hostname) {
    console.log(`\n=== Testing DNS Resolution for: ${hostname} ===\n`);
    
    try {
        // Test basic resolution
        console.log('1. Testing basic DNS resolution...');
        const addresses = await dnsResolver.resolveHostname(hostname);
        console.log(`   ✓ Resolved to ${addresses.length} addresses:`);
        addresses.forEach((addr, idx) => {
            console.log(`     ${idx + 1}. ${addr}`);
        });
        
        // Test cache
        console.log('\n2. Testing cached resolution...');
        const cachedAddresses = await dnsResolver.resolveHostname(hostname);
        console.log(`   ✓ Got ${cachedAddresses.length} addresses from cache`);
        
        // Test marking IPs as failed
        if (addresses.length > 1) {
            console.log('\n3. Testing failed IP handling...');
            const firstIp = addresses[0];
            dnsResolver.markHostFailed(firstIp);
            console.log(`   - Marked ${firstIp} as failed`);
            
            const filteredAddresses = await dnsResolver.resolveHostname(hostname);
            console.log(`   ✓ After filtering: ${filteredAddresses.length} working addresses`);
            
            // Wait for retry delay
            console.log('\n4. Testing retry delay...');
            console.log(`   - Waiting for failed IP to be retryable...`);
            
            const isStillFailed = dnsResolver.isHostFailed(firstIp);
            console.log(`   - IP ${firstIp} is ${isStillFailed ? 'still marked as failed' : 'available again'}`);
        }
        
        // Test hostname extraction
        console.log('\n5. Testing hostname extraction...');
        const testUrls = [
            `https://${hostname}:8006`,
            `${hostname}:8006`,
            `https://${hostname}/api2/json`,
            hostname
        ];
        
        testUrls.forEach(url => {
            const extracted = dnsResolver.extractHostname(url);
            console.log(`   - "${url}" -> "${extracted}"`);
        });
        
        // Test canResolve
        console.log('\n6. Testing canResolve...');
        const canResolve = await dnsResolver.canResolve(hostname);
        console.log(`   ✓ Can resolve ${hostname}: ${canResolve}`);
        
        console.log('\n=== Test completed successfully ===\n');
        
    } catch (error) {
        console.error(`\n✗ DNS resolution failed: ${error.message}\n`);
        process.exit(1);
    }
}

// Main execution
const hostname = process.argv[2];

if (!hostname) {
    console.error('Usage: node test-dns-resolver.js <hostname>');
    console.error('Example: node test-dns-resolver.js proxmox.lan');
    process.exit(1);
}

testDnsResolution(hostname).catch(error => {
    console.error('Unexpected error:', error);
    process.exit(1);
});