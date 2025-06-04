/**
 * User Workflow Tests - Real Production Scenarios
 * These tests validate actual user workflows and would catch bugs that affect real users
 */

const { fetchDiscoveryData, fetchMetricsData, fetchPbsData, clearCaches } = require('../dataFetcher');
const { processPbsTasks } = require('../pbsUtils');
const customThresholds = require('../customThresholds');
const AlertManager = require('../alertManager');

// Mock only external dependencies, not our business logic
jest.mock('fs', () => ({
    promises: {
        mkdir: jest.fn().mockResolvedValue(),
        readFile: jest.fn().mockResolvedValue('{}'),
        writeFile: jest.fn().mockResolvedValue()
    }
}));

describe('Real User Workflows - Production Scenarios', () => {
    let realApiData;
    let alertManager;

    beforeEach(() => {
        clearCaches();
        alertManager = new AlertManager();
        customThresholds.cache.clear();
        jest.clearAllMocks();

        // Create realistic production data based on your actual setup
        realApiData = {
            // Realistic PVE cluster based on your ground truth data
            pveCluster: {
                nodes: [
                    { node: 'desktop', status: 'online', uptime: 86400 * 5 }, // 5 days
                    { node: 'delly', status: 'online', uptime: 86400 * 12 }, // 12 days  
                    { node: 'minipc', status: 'online', uptime: 86400 * 8 } // 8 days
                ],
                vms: [
                    { vmid: 102, name: 'windows11', status: 'stopped', node: 'desktop', agent: 0 },
                    { vmid: 200, name: 'UnraidServer', status: 'running', node: 'desktop', agent: 1 },
                    { vmid: 400, name: 'ubuntu-gpu-vm', status: 'running', node: 'desktop', agent: 1 }
                ],
                containers: [
                    { vmid: 100, name: 'pbs', status: 'running', node: 'desktop' },
                    { vmid: 101, name: 'homeassistant', status: 'running', node: 'delly' },
                    { vmid: 103, name: 'pihole', status: 'running', node: 'minipc' },
                    { vmid: 106, name: 'pulse', status: 'running', node: 'minipc' }, // This very app!
                    // ... 14 more containers for realistic 18 total guests
                    { vmid: 107, name: 'jellyfin', status: 'running', node: 'minipc' },
                    { vmid: 108, name: 'frigate', status: 'running', node: 'delly' },
                    { vmid: 109, name: 'pbs2', status: 'stopped', node: 'desktop' },
                    { vmid: 110, name: 'tailscale-router', status: 'running', node: 'delly' },
                    { vmid: 111, name: 'debian', status: 'stopped', node: 'desktop' },
                    { vmid: 120, name: 'mqtt', status: 'running', node: 'minipc' },
                    { vmid: 121, name: 'zigbee2mqtt', status: 'running', node: 'minipc' },
                    { vmid: 122, name: 'influxdb-telegraf', status: 'running', node: 'delly' },
                    { vmid: 124, name: 'grafana', status: 'running', node: 'minipc' },
                    { vmid: 105, name: 'homepage', status: 'running', node: 'delly' },
                    { vmid: 104, name: 'cloudflared', status: 'running', node: 'minipc' }
                ]
            },
            // Realistic backup data from your PBS
            pbsBackups: {
                datastores: [{
                    name: 'main-datastore',
                    snapshots: [
                        // Most containers have backups from 2 AM (primary job)
                        { 'backup-id': '100', 'backup-type': 'ct', 'backup-time': getTwoAMToday() },
                        { 'backup-id': '101', 'backup-type': 'ct', 'backup-time': getTwoAMToday() },
                        { 'backup-id': '103', 'backup-type': 'ct', 'backup-time': getTwoAMToday() },
                        { 'backup-id': '106', 'backup-type': 'ct', 'backup-time': getTwoAMToday() },
                        // VM 102 - THE PROBLEM CHILD (no recent backup!)
                        { 'backup-id': '102', 'backup-type': 'vm', 'backup-time': getThreeDaysAgo() },
                        // VMs 200, 400 have backups from 4 AM (secondary job)  
                        { 'backup-id': '200', 'backup-type': 'vm', 'backup-time': getFourAMToday() },
                        { 'backup-id': '400', 'backup-type': 'vm', 'backup-time': getFourAMToday() },
                        // More containers...
                        { 'backup-id': '107', 'backup-type': 'ct', 'backup-time': getTwoAMToday() },
                        { 'backup-id': '108', 'backup-type': 'ct', 'backup-time': getTwoAMToday() },
                        { 'backup-id': '110', 'backup-type': 'ct', 'backup-time': getTwoAMToday() }
                    ]
                }]
            },
            // Realistic metrics - some VMs under stress
            currentMetrics: {
                // Healthy VM
                200: { cpu: 0.15, memory: 2147483648, disk: 10737418240 }, // 15% CPU, 2GB RAM
                // VM under CPU pressure
                400: { cpu: 0.89, memory: 4294967296, disk: 21474836480 }, // 89% CPU, 4GB RAM  
                // Container with memory pressure
                101: { cpu: 0.25, memory: 1073741824, disk: 5368709120 }, // 25% CPU, 1GB RAM
                106: { cpu: 0.12, memory: 536870912, disk: 2684354560 } // Pulse itself
            }
        };
    });

    afterEach(() => {
        if (alertManager) {
            alertManager.destroy();
        }
    });

    describe('Scenario 1: Admin Investigates "Why Does Dashboard Show Wrong VM Count?"', () => {
        test('should detect VM count discrepancy between dashboard and reality', async () => {
            // REAL SCENARIO: Dashboard shows 20 VMs but only 18 guests exist
            
            // Mock realistic discovery that returns actual guest data
            const mockApiClients = createRealisticMockClients(realApiData.pveCluster);
            
            const discoveryData = await fetchDiscoveryData(mockApiClients, {});
            
            // Count actual guests
            const totalGuests = discoveryData.vms.length + discoveryData.containers.length;
            
            // VALIDATE: Should match your known ground truth (18 guests total)
            expect(totalGuests).toBe(18);
            expect(discoveryData.vms).toHaveLength(3); // VMs: 102, 200, 400
            expect(discoveryData.containers).toHaveLength(15); // All the containers
            
            // VALIDATE: All known guests are present
            const allVmids = [...discoveryData.vms, ...discoveryData.containers].map(g => g.vmid);
            expect(allVmids).toContain(102); // windows11
            expect(allVmids).toContain(106); // pulse (this app!)
            expect(allVmids).toContain(200); // UnraidServer
            
            // DETECT: If count was wrong, this would help debug
            if (totalGuests !== 18) {
                console.error(`DISCREPANCY: Expected 18 guests, found ${totalGuests}`);
                console.error('Missing guests:', [100,101,102,103,104,105,106,107,108,109,110,111,120,121,122,124,200,400].filter(id => !allVmids.includes(id)));
                console.error('Extra guests:', allVmids.filter(id => ![100,101,102,103,104,105,106,107,108,109,110,111,120,121,122,124,200,400].includes(id)));
            }
        });
    });

    describe('Scenario 2: Admin Investigates "VM 102 Backup Issue"', () => {
        test('should detect that VM 102 backup is dangerously old', async () => {
            // REAL SCENARIO: VM 102 should be in backup job but backup is 3 days old
            
            const mockPbsClients = createRealisticPbsClients(realApiData.pbsBackups);
            const pbsData = await fetchPbsData(mockPbsClients);
            
            // Find VM 102 backup
            const vm102Backups = pbsData[0].datastores[0].snapshots.filter(
                snap => snap['backup-id'] === '102' && snap['backup-type'] === 'vm'
            );
            
            expect(vm102Backups).toHaveLength(1);
            
            const vm102LastBackup = vm102Backups[0];
            const backupAge = (Date.now() / 1000) - vm102LastBackup['backup-time'];
            const ageInHours = backupAge / 3600;
            
            // VALIDATE: This should detect the problem
            expect(ageInHours).toBeGreaterThan(48); // More than 2 days old!
            
            // ALERT: This should trigger a critical alert
            if (ageInHours > 24) {
                console.warn(`CRITICAL: VM 102 backup is ${Math.round(ageInHours)} hours old!`);
            }
            
            // COMPARE: Other VMs should have recent backups
            const vm200Backups = pbsData[0].datastores[0].snapshots.filter(
                snap => snap['backup-id'] === '200' && snap['backup-type'] === 'vm'
            );
            const vm200Age = (Date.now() / 1000) - vm200Backups[0]['backup-time'];
            expect(vm200Age / 3600).toBeLessThan(24); // Should be recent
        });

        test('should identify backup job configuration issue', async () => {
            // REAL SCENARIO: VM 102 might be excluded from backup jobs or job failed
            
            const mockPbsClients = createRealisticPbsClients(realApiData.pbsBackups);
            const pbsData = await fetchPbsData(mockPbsClients);
            
            // Analyze backup patterns to detect issues
            const backupsByGuest = {};
            pbsData[0].datastores[0].snapshots.forEach(snap => {
                const guestId = snap['backup-id'];
                if (!backupsByGuest[guestId]) {
                    backupsByGuest[guestId] = [];
                }
                backupsByGuest[guestId].push(snap);
            });
            
            // Check backup frequency patterns
            const recentBackups = Object.keys(backupsByGuest).filter(guestId => {
                const latestBackup = backupsByGuest[guestId][0];
                const ageHours = (Date.now() / 1000 - latestBackup['backup-time']) / 3600;
                return ageHours < 24;
            });
            
            // VALIDATE: Most guests should have recent backups
            expect(recentBackups.length).toBeGreaterThan(5);
            
            // DETECT: VM 102 should be flagged as problematic
            expect(recentBackups).not.toContain('102');
            
            // IDENTIFY: Pattern analysis
            const guestsWithoutRecentBackups = Object.keys(backupsByGuest).filter(id => !recentBackups.includes(id));
            if (guestsWithoutRecentBackups.length > 0) {
                console.warn(`Guests with old backups: ${guestsWithoutRecentBackups.join(', ')}`);
            }
        });
    });

    describe('Scenario 3: Admin Responds to "High CPU Alert Storm"', () => {
        test('should detect which VMs are actually problematic vs false alarms', async () => {
            // REAL SCENARIO: Multiple CPU alerts, admin needs to prioritize
            
            const mockApiClients = createRealisticMockClientsWithMetrics(realApiData.currentMetrics);
            const discoveryData = await fetchDiscoveryData(mockApiClients, {});
            
            const runningGuests = [
                ...discoveryData.vms.filter(vm => vm.status === 'running'),
                ...discoveryData.containers.filter(ct => ct.status === 'running')
            ];
            
            const metricsData = await fetchMetricsData(
                discoveryData.vms.filter(vm => vm.status === 'running'),
                discoveryData.containers.filter(ct => ct.status === 'running'),
                mockApiClients
            );
            
            // ANALYZE: Which guests actually have high CPU
            const highCpuGuests = metricsData.filter(metrics => metrics.current.cpu > 0.8);
            const moderateCpuGuests = metricsData.filter(metrics => metrics.current.cpu > 0.5 && metrics.current.cpu <= 0.8);
            
            // VALIDATE: Should detect VM 400 as high CPU (89%)
            expect(highCpuGuests).toHaveLength(1);
            expect(highCpuGuests[0].id).toBe(400);
            expect(highCpuGuests[0].current.cpu).toBeCloseTo(0.89, 2);
            
            // PRIORITIZE: Admin can focus on real issues
            console.log(`HIGH PRIORITY: ${highCpuGuests.length} guests with CPU >80%`);
            console.log(`MEDIUM PRIORITY: ${moderateCpuGuests.length} guests with CPU 50-80%`);
            
            highCpuGuests.forEach(guest => {
                const guestInfo = runningGuests.find(g => g.vmid === guest.id);
                console.log(`  - ${guestInfo.name} (${guestInfo.type} ${guest.id}): ${Math.round(guest.current.cpu * 100)}% CPU`);
            });
        });

        test('should validate alert suppression during maintenance', async () => {
            // REAL SCENARIO: Admin puts VM 400 in maintenance, alerts should stop
            
            // Set custom thresholds to ensure alerts would normally fire
            await customThresholds.setThresholds('primary', 'desktop', '400', {
                cpu: { warning: 70, critical: 85 }
            });
            
            const mockApiClients = createRealisticMockClientsWithMetrics(realApiData.currentMetrics);
            const metricsData = await fetchMetricsData([], [
                { vmid: 400, name: 'ubuntu-gpu-vm', status: 'running', endpointId: 'primary', node: 'desktop', type: 'qemu' }
            ], mockApiClients);
            
            // Process alerts normally - should fire
            const triggeredAlerts = alertManager.processMetrics(metricsData);
            expect(triggeredAlerts.length).toBeGreaterThan(0);
            
            // Suppress alerts for maintenance
            alertManager.suppressAlert('cpu_high', { vmid: 400 }, 3600000, 'Maintenance window');
            
            // Process again - should be suppressed
            const suppressedAlerts = alertManager.processMetrics(metricsData);
            const vm400Alerts = suppressedAlerts.filter(alert => alert.guest.vmid === '400');
            expect(vm400Alerts).toHaveLength(0);
        });
    });

    describe('Scenario 4: Admin Validates "Backup Job Health"', () => {
        test('should validate backup job scheduling is working correctly', async () => {
            // REAL SCENARIO: Admin checks if backup jobs ran on schedule
            
            const mockPbsClients = createRealisticPbsClients(realApiData.pbsBackups);
            const pbsData = await fetchPbsData(mockPbsClients);
            
            // Group backups by time to detect job patterns
            const backupTimes = {};
            pbsData[0].datastores[0].snapshots.forEach(snap => {
                const backupHour = new Date(snap['backup-time'] * 1000).getHours();
                if (!backupTimes[backupHour]) {
                    backupTimes[backupHour] = [];
                }
                backupTimes[backupHour].push(snap);
            });
            
            // VALIDATE: Should see backups at 2 AM and 4 AM (your backup schedule)
            expect(backupTimes[2]).toBeDefined(); // Primary job at 2 AM
            expect(backupTimes[4]).toBeDefined(); // Secondary job at 4 AM
            
            // VALIDATE: 2 AM job should have most containers
            const twoAMBackups = backupTimes[2] || [];
            const fourAMBackups = backupTimes[4] || [];
            
            expect(twoAMBackups.length).toBeGreaterThan(fourAMBackups.length);
            
            // VALIDATE: Specific VMs should be in correct jobs
            const twoAMVmids = twoAMBackups.map(b => b['backup-id']);
            const fourAMVmids = fourAMBackups.map(b => b['backup-id']);
            
            // Based on your ground truth: VMs 200, 400 in secondary job (4 AM)
            expect(fourAMVmids).toContain('200');
            expect(fourAMVmids).toContain('400');
            
            // Most containers in primary job (2 AM) - excluding VMs 102, 200, 400
            expect(twoAMVmids).toContain('100'); // pbs container
            expect(twoAMVmids).toContain('106'); // pulse container
            
            console.log(`Primary job (2 AM): ${twoAMBackups.length} backups`);
            console.log(`Secondary job (4 AM): ${fourAMBackups.length} backups`);
        });
    });

    describe('Scenario 5: Performance Under Load', () => {
        test('should handle realistic cluster size without performance degradation', async () => {
            // REAL SCENARIO: System should stay responsive with full cluster
            
            const startTime = Date.now();
            const startMemory = process.memoryUsage().heapUsed;
            
            // Create full realistic cluster
            const mockApiClients = createLargeRealisticCluster();
            
            const discoveryData = await fetchDiscoveryData(mockApiClients, {});
            const discoveryTime = Date.now() - startTime;
            
            // VALIDATE: Performance should be acceptable
            expect(discoveryTime).toBeLessThan(10000); // 10 seconds max for discovery
            expect(discoveryData.nodes.length).toBeGreaterThan(2);
            expect(discoveryData.vms.length + discoveryData.containers.length).toBeGreaterThan(15);
            
            // VALIDATE: Memory usage should be reasonable
            const endMemory = process.memoryUsage().heapUsed;
            const memoryIncrease = endMemory - startMemory;
            expect(memoryIncrease).toBeLessThan(100 * 1024 * 1024); // Less than 100MB increase
            
            console.log(`Discovery took ${discoveryTime}ms for ${discoveryData.vms.length + discoveryData.containers.length} guests`);
            console.log(`Memory increase: ${Math.round(memoryIncrease / 1024 / 1024)}MB`);
        });
    });
});

// Helper functions for realistic test data
function getTwoAMToday() {
    const now = new Date();
    const twoAM = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 2, 0, 0);
    if (twoAM > now) {
        twoAM.setDate(twoAM.getDate() - 1); // Yesterday's 2 AM
    }
    return Math.floor(twoAM.getTime() / 1000);
}

function getFourAMToday() {
    const now = new Date();
    const fourAM = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 4, 0, 0);
    if (fourAM > now) {
        fourAM.setDate(fourAM.getDate() - 1); // Yesterday's 4 AM
    }
    return Math.floor(fourAM.getTime() / 1000);
}

function getThreeDaysAgo() {
    const threeDaysAgo = new Date();
    threeDaysAgo.setDate(threeDaysAgo.getDate() - 3);
    threeDaysAgo.setHours(2, 0, 0, 0); // 2 AM three days ago
    return Math.floor(threeDaysAgo.getTime() / 1000);
}

function createRealisticMockClients(pveCluster) {
    return {
        'primary': {
            client: {
                get: jest.fn().mockImplementation((path) => {
                    if (path === '/nodes') {
                        return Promise.resolve({ data: { data: pveCluster.nodes } });
                    }
                    if (path.includes('/qemu')) {
                        const node = path.split('/')[2];
                        const nodeVms = pveCluster.vms.filter(vm => vm.node === node);
                        return Promise.resolve({ data: { data: nodeVms } });
                    }
                    if (path.includes('/lxc')) {
                        const node = path.split('/')[2];
                        const nodeContainers = pveCluster.containers.filter(ct => ct.node === node);
                        return Promise.resolve({ data: { data: nodeContainers } });
                    }
                    return Promise.resolve({ data: { data: [] } });
                })
            },
            config: { id: 'primary', name: 'Test Cluster' }
        }
    };
}

function createRealisticPbsClients(pbsBackups) {
    return {
        'pbs-main': {
            client: {
                get: jest.fn().mockImplementation((path) => {
                    if (path === '/nodes') {
                        return Promise.resolve({ data: { data: [{ node: 'pbs-node' }] } });
                    }
                    if (path === '/config/datastore') {
                        return Promise.resolve({ data: { data: [{ name: 'main-datastore' }] } });
                    }
                    if (path.includes('/admin/datastore/main-datastore/snapshots')) {
                        return Promise.resolve({ data: { data: pbsBackups.datastores[0].snapshots } });
                    }
                    if (path.includes('/status/datastore-usage')) {
                        return Promise.resolve({ data: { data: [{ store: 'main-datastore', total: 1000000000, used: 500000000 }] } });
                    }
                    return Promise.resolve({ data: { data: [] } });
                })
            },
            config: { id: 'pbs-main', name: 'Test PBS' }
        }
    };
}

function createRealisticMockClientsWithMetrics(currentMetrics) {
    return {
        'primary': {
            client: {
                get: jest.fn().mockImplementation((path) => {
                    if (path.includes('/status')) {
                        const vmidMatch = path.match(/\/(qemu|lxc)\/(\d+)\/status/);
                        if (vmidMatch) {
                            const vmid = parseInt(vmidMatch[2]);
                            const metrics = currentMetrics[vmid];
                            if (metrics) {
                                return Promise.resolve({ data: { data: metrics } });
                            }
                        }
                        return Promise.resolve({ data: { data: { cpu: 0.1, memory: 1073741824, disk: 5368709120 } } });
                    }
                    if (path.includes('/rrddata')) {
                        return Promise.resolve({ data: { data: [{ time: Date.now() / 1000, cpu: 0.1 }] } });
                    }
                    return Promise.resolve({ data: { data: [] } });
                })
            },
            config: { id: 'primary', name: 'Test Cluster' }
        }
    };
}

function createLargeRealisticCluster() {
    // Create a larger but still realistic cluster
    const nodes = ['desktop', 'delly', 'minipc', 'server1', 'server2'];
    const largeCluster = {
        nodes: nodes.map(name => ({ node: name, status: 'online', uptime: 86400 })),
        vms: [],
        containers: []
    };
    
    // Add realistic VMs and containers distributed across nodes
    let vmid = 100;
    nodes.forEach((node, nodeIndex) => {
        // Add some VMs per node
        for (let i = 0; i < 3; i++) {
            largeCluster.vms.push({
                vmid: vmid++,
                name: `vm-${node}-${i}`,
                status: Math.random() > 0.1 ? 'running' : 'stopped',
                node: node
            });
        }
        // Add some containers per node
        for (let i = 0; i < 8; i++) {
            largeCluster.containers.push({
                vmid: vmid++,
                name: `ct-${node}-${i}`,
                status: Math.random() > 0.05 ? 'running' : 'stopped',
                node: node
            });
        }
    });
    
    return createRealisticMockClients(largeCluster);
}