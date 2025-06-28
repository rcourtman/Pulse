#!/usr/bin/env node

const axios = require('axios');
const { detectVmidCollisions, analyzePbsConfiguration } = require('./server/pbsUtils');

const API_BASE = 'http://localhost:7655';

// Color codes for output
const colors = {
    reset: '\x1b[0m',
    bright: '\x1b[1m',
    red: '\x1b[31m',
    green: '\x1b[32m',
    yellow: '\x1b[33m',
    blue: '\x1b[34m',
    cyan: '\x1b[36m'
};

function log(message, color = 'reset') {
    console.log(`${colors[color]}${message}${colors.reset}`);
}

async function fetchData(endpoint) {
    try {
        const response = await axios.get(`${API_BASE}${endpoint}`);
        return response.data;
    } catch (error) {
        log(`Failed to fetch ${endpoint}: ${error.message}`, 'red');
        return null;
    }
}

async function runValidationTests() {
    log('\n=== PBS UI/API Validation Tests ===\n', 'bright');
    
    // Test 1: Compare state endpoint with PBS API
    log('Test 1: Comparing /api/state PBS data with /api/pbs/backups', 'cyan');
    
    const stateData = await fetchData('/api/state');
    const pbsApiData = await fetchData('/api/pbs/backups');
    
    if (stateData && pbsApiData) {
        const statePbs = stateData.pbs || [];
        const apiPbs = pbsApiData.data?.instances || [];
        
        log(`State endpoint PBS instances: ${statePbs.length}`, 'blue');
        log(`PBS API instances: ${apiPbs.length}`, 'blue');
        
        if (statePbs.length !== apiPbs.length) {
            log('❌ Instance count mismatch!', 'red');
        } else {
            log('✅ Instance count matches', 'green');
        }
        
        // Compare collision detection
        statePbs.forEach((instance, idx) => {
            log(`\nInstance: ${instance.pbsInstanceName}`, 'yellow');
            
            // Check collision data
            if (instance.vmidCollisions) {
                log(`  State: ${instance.vmidCollisions.totalCollisions} collisions detected`, 'blue');
            }
            
            const apiInstance = apiPbs[idx];
            if (apiInstance && apiInstance.collisions) {
                const apiCollisionCount = apiInstance.collisions.reduce((sum, c) => 
                    sum + (c.collisions?.length || 0), 0);
                log(`  API: ${apiCollisionCount} collisions in response`, 'blue');
            }
        });
    }
    
    // Test 2: Validate collision endpoint
    log('\n\nTest 2: Validating collision detection endpoint', 'cyan');
    
    const collisionData = await fetchData('/api/pbs/collisions');
    if (collisionData && collisionData.data) {
        const { hasCollisions, totalCollisions, criticalCollisions, warningCollisions } = collisionData.data;
        log(`Collisions detected: ${hasCollisions}`, hasCollisions ? 'yellow' : 'green');
        log(`Total: ${totalCollisions} (Critical: ${criticalCollisions}, Warning: ${warningCollisions})`, 'blue');
        
        if (collisionData.data.byInstance) {
            collisionData.data.byInstance.forEach(inst => {
                log(`\n${inst.instanceName}:`, 'yellow');
                Object.entries(inst.collisions.byDatastore || {}).forEach(([ds, info]) => {
                    log(`  Datastore ${ds}: ${info.collisions.length} collisions`, 'blue');
                    // Show first 3 collisions
                    info.collisions.slice(0, 3).forEach(c => {
                        log(`    - ${c.vmid}: ${c.sources.length} sources`, 'blue');
                    });
                });
            });
        }
    }
    
    // Test 3: Date-based queries
    log('\n\nTest 3: Testing date-based queries', 'cyan');
    
    const testDates = [
        '2025-06-27',
        '2025-06-28',
        new Date().toISOString().split('T')[0] // Today
    ];
    
    for (const date of testDates) {
        const dateData = await fetchData(`/api/pbs/backups/date/${date}`);
        if (dateData && dateData.data) {
            const { totalBackups, totalSize, guests } = dateData.data.summary;
            log(`\n${date}: ${totalBackups} backups, ${guests} guests, ${(totalSize / 1e9).toFixed(2)} GB`, 'blue');
            
            // Show hourly distribution
            const hourlyData = dateData.data.summary.byHour;
            const activeHours = hourlyData.map((count, hour) => 
                count > 0 ? `${hour}h(${count})` : null
            ).filter(Boolean).join(', ');
            log(`  Active hours: ${activeHours || 'None'}`, 'blue');
        }
    }
    
    // Test 4: Namespace summary comparison
    log('\n\nTest 4: Namespace summary comparison', 'cyan');
    
    if (pbsApiData && pbsApiData.data) {
        const globalStats = pbsApiData.data.globalStats;
        log(`\nGlobal namespace statistics:`, 'yellow');
        log(`Namespaces: ${globalStats.namespaces.join(', ')}`, 'blue');
        
        pbsApiData.data.instances.forEach(instance => {
            log(`\n${instance.name} namespace breakdown:`, 'yellow');
            Object.entries(instance.namespaces || {}).forEach(([ns, data]) => {
                const vmCount = Object.keys(data.vms || {}).length;
                const ctCount = Object.keys(data.cts || {}).length;
                log(`  ${ns}: ${data.totalBackups} backups, ${vmCount} VMs, ${ctCount} CTs`, 'blue');
            });
        });
    }
    
    // Test 5: Guest-specific queries
    log('\n\nTest 5: Testing guest-specific queries', 'cyan');
    
    // Find a guest with collisions
    if (collisionData && collisionData.data.byInstance.length > 0) {
        const firstCollision = collisionData.data.byInstance[0].collisions.byDatastore.main?.collisions[0];
        if (firstCollision) {
            const [type, id] = firstCollision.vmid.split('/');
            const guestData = await fetchData(`/api/pbs/backups/guest/${type}/${id}`);
            
            if (guestData && guestData.data) {
                log(`\nGuest ${type}/${id}:`, 'yellow');
                log(`Total backups: ${guestData.data.totalBackups}`, 'blue');
                log(`Total size: ${(guestData.data.totalSize / 1e9).toFixed(2)} GB`, 'blue');
                log(`Instances with backups: ${guestData.data.instances.length}`, 'blue');
            }
        }
    }
    
    // Test 6: Summary card data validation
    log('\n\nTest 6: Validating summary card data', 'cyan');
    
    // The summary card should show namespace-grouped data
    if (statePbs.length > 0 && statePbs[0].datastores) {
        const instance = statePbs[0];
        log(`\nAnalyzing ${instance.pbsInstanceName}:`, 'yellow');
        
        // Count snapshots by namespace manually
        const manualNamespaceCounts = {};
        let totalSnapshots = 0;
        
        instance.datastores.forEach(ds => {
            if (ds.snapshots && Array.isArray(ds.snapshots)) {
                ds.snapshots.forEach(snap => {
                    const ns = snap.namespace || 'root';
                    if (!manualNamespaceCounts[ns]) {
                        manualNamespaceCounts[ns] = { count: 0, vms: new Set(), cts: new Set() };
                    }
                    manualNamespaceCounts[ns].count++;
                    if (snap['backup-type'] === 'vm') {
                        manualNamespaceCounts[ns].vms.add(snap['backup-id']);
                    } else {
                        manualNamespaceCounts[ns].cts.add(snap['backup-id']);
                    }
                    totalSnapshots++;
                });
            }
        });
        
        log(`\nManual count from state data:`, 'green');
        Object.entries(manualNamespaceCounts).forEach(([ns, data]) => {
            log(`  ${ns}: ${data.count} backups, ${data.vms.size} VMs, ${data.cts.size} CTs`, 'blue');
        });
        log(`  Total: ${totalSnapshots} snapshots`, 'blue');
        
        // Compare with API data
        const apiInstance = pbsApiData.data.instances[0];
        if (apiInstance) {
            log(`\nAPI processed data:`, 'green');
            Object.entries(apiInstance.namespaces || {}).forEach(([ns, data]) => {
                const vmCount = Object.keys(data.vms || {}).length;
                const ctCount = Object.keys(data.cts || {}).length;
                log(`  ${ns}: ${data.totalBackups} backups, ${vmCount} VMs, ${ctCount} CTs`, 'blue');
            });
            log(`  Total: ${apiInstance.stats.totalBackups} backups`, 'blue');
        }
    }
    
    log('\n\n=== Test Summary ===', 'bright');
    log('Check the above output for any discrepancies between:', 'yellow');
    log('1. State endpoint raw data vs PBS API processed data', 'blue');
    log('2. Manual counts vs API counts', 'blue');
    log('3. Collision detection consistency', 'blue');
    log('4. Namespace grouping accuracy', 'blue');
}

// Run the tests
runValidationTests().catch(error => {
    log(`Test failed: ${error.message}`, 'red');
    process.exit(1);
});