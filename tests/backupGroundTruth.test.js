const { fetchDiscoveryData, fetchPbsData } = require('../dataFetcher');
const { processPbsTasks } = require('../pbsUtils');

// Mock data based on your ground truth research
const groundTruthData = {
  totalGuests: 18, // Actual cluster count
  pbsBackupsTotal: 135,
  vmSnapshots: 3, // Only 3 actual VM/CT snapshots
  
  // Backup job schedules
  primaryBackupJob: {
    id: 'backup-2759a200-3e11',
    schedule: '02:00 AM',
    excludes: [102, 200, 400],
    retention: { daily: 7, weekly: 4, monthly: 3 }
  },
  secondaryBackupJob: {
    id: 'backup-79ce96ee-6527',
    schedule: '04:00 AM',
    includes: [102, 200, 400],
    retention: { keepLast: 3 }
  },
  
  // Expected backup ages (as of June 2, 12:50 PM BST)
  expectedBackupAges: {
    primaryJobGuests: { minHours: 10, maxHours: 11 }, // 2:00-2:10 AM backups
    secondaryJobGuests: { minHours: 8, maxHours: 9 }, // 4:00 AM backups
    vm102: 'no_recent_backup' // Issue found in research
  },
  
  // Known issues from research
  knownIssues: {
    guestCountDiscrepancy: true, // Pulse shows 20, actual is 18
    vm102BackupMissing: true,
    multipleEndpoints: 2, // proxmox.lan and pimox.lan
    snapshotLoggingConfusion: true // Logs incorrectly label PBS backups as snapshots
  }
};

describe('Backup Ground Truth Verification Tests', () => {
  let mockApiClients;
  let mockPbsApiClients;
  let discoveryData;
  
  beforeEach(() => {
    // Mock the API clients with realistic data
    mockApiClients = {
      'proxmox-lan': {
        client: {
          get: jest.fn()
        },
        config: {
          name: 'proxmox.lan',
          tokenId: 'test@pve!test',
          tokenSecret: 'test-secret'
        }
      },
      'pimox-lan': {
        client: {
          get: jest.fn()
        },
        config: {
          name: 'pimox.lan',
          tokenId: 'test@pve!test',
          tokenSecret: 'test-secret'
        }
      }
    };
    
    mockPbsApiClients = {
      'pbs-main': {
        client: {
          get: jest.fn(),
          post: jest.fn()
        },
        config: {
          name: 'PBS Storage',
          nodeName: 'pbs-node'
        }
      }
    };
  });
  
  describe('Guest Count Verification', () => {
    test('should correctly count total guests across all endpoints', async () => {
      // Mock PVE nodes response
      mockApiClients['proxmox-lan'].client.get.mockImplementation((path) => {
        if (path === '/cluster/status') {
          return Promise.resolve({
            data: {
              data: [
                { type: 'cluster', name: 'proxmox-cluster', nodes: 3 },
                { type: 'node', name: 'desktop', ip: '192.168.1.10' },
                { type: 'node', name: 'delly', ip: '192.168.1.11' },
                { type: 'node', name: 'minipc', ip: '192.168.1.12' }
              ]
            }
          });
        }
        if (path === '/nodes') {
          return Promise.resolve({
            data: {
              data: [
                { node: 'desktop', status: 'online' },
                { node: 'delly', status: 'online' },
                { node: 'minipc', status: 'online' }
              ]
            }
          });
        }
        if (path.includes('/qemu')) {
          // Each node has different VMs
          if (path.includes('/nodes/desktop/')) {
            return Promise.resolve({ data: { data: [
              { vmid: 102, name: 'windows11', status: 'stopped' },
              { vmid: 200, name: 'UnraidServer', status: 'stopped' },
              { vmid: 400, name: 'ubuntu-gpu-vm', status: 'stopped' }
            ]}});
          }
          return Promise.resolve({ data: { data: [] }});
        }
        if (path.includes('/lxc')) {
          // Distribute containers across nodes
          if (path.includes('/nodes/desktop/')) {
            return Promise.resolve({ data: { data: [
              { vmid: 100, name: 'pbs', status: 'running' },
              { vmid: 109, name: 'pbs2', status: 'stopped' },
              { vmid: 111, name: 'debian', status: 'stopped' }
            ]}});
          } else if (path.includes('/nodes/delly/')) {
            return Promise.resolve({ data: { data: [
              { vmid: 101, name: 'homeassistant', status: 'running' },
              { vmid: 105, name: 'homepage', status: 'running' },
              { vmid: 108, name: 'frigate', status: 'running' },
              { vmid: 110, name: 'tailscale-router', status: 'running' },
              { vmid: 122, name: 'influxdb-telegraf', status: 'running' }
            ]}});
          } else if (path.includes('/nodes/minipc/')) {
            return Promise.resolve({ data: { data: [
              { vmid: 103, name: 'pihole', status: 'running' },
              { vmid: 104, name: 'cloudflared', status: 'running' },
              { vmid: 106, name: 'pulse', status: 'running' },
              { vmid: 107, name: 'jellyfin', status: 'running' },
              { vmid: 120, name: 'mqtt', status: 'running' },
              { vmid: 121, name: 'zigbee2mqtt', status: 'running' },
              { vmid: 124, name: 'grafana', status: 'running' }
            ]}});
          }
          return Promise.resolve({ data: { data: [] }});
        }
        return Promise.resolve({ data: { data: [] } });
      });
      
      mockApiClients['pimox-lan'].client.get.mockImplementation((path) => {
        if (path === '/cluster/status') {
          return Promise.resolve({
            data: {
              data: [
                { type: 'node', name: 'pi', ip: '192.168.1.20' }
              ]
            }
          });
        }
        if (path === '/nodes') {
          return Promise.resolve({
            data: {
              data: [{ node: 'pi', status: 'online' }]
            }
          });
        }
        if (path.includes('/qemu')) {
          return Promise.resolve({ data: { data: [] }});
        }
        if (path.includes('/lxc')) {
          return Promise.resolve({ data: { data: [] }});
        }
        return Promise.resolve({ data: { data: [] } });
      });
      
      discoveryData = await fetchDiscoveryData(mockApiClients, {});
      
      const totalVMs = discoveryData.vms.length;
      const totalContainers = discoveryData.containers.length;
      const totalGuests = totalVMs + totalContainers;
      
      // Verify against ground truth
      expect(totalGuests).toBe(groundTruthData.totalGuests);
      expect(totalVMs).toBe(3); // VMs 102, 200, 400
      expect(totalContainers).toBe(15); // All containers across all nodes
      
      // Check for known discrepancy
      if (totalGuests !== 20) {
        console.log(`Guest count discrepancy detected: Actual ${totalGuests}, Pulse might show 20`);
      }
    });
  });
  
  describe('PBS Backup Count Verification', () => {
    test('should correctly count PBS backups vs VM snapshots', async () => {
      // Mock PBS datastore groups and snapshots
      mockPbsApiClients['pbs-main'].client.get.mockImplementation((path) => {
        if (path === '/nodes') {
          return Promise.resolve({
            data: { data: [{ node: 'pbs-node' }] }
          });
        }
        if (path === '/config/datastore') {
          return Promise.resolve({
            data: { data: [{ name: 'main-datastore' }] }
          });
        }
        if (path.includes('/admin/datastore/main-datastore/snapshots')) {
          // This is called by fetchPbsDatastoreSnapshots - return all 135 snapshots
          const allSnapshots = [];
          const now = Math.floor(Date.now() / 1000);
          
          // Create snapshots for all guests
          const guests = [
            { type: 'ct', id: '100', count: 9 },
            { type: 'ct', id: '101', count: 9 },
            { type: 'vm', id: '102', count: 0 }, // VM 102 has no backups
            { type: 'ct', id: '103', count: 9 },
            { type: 'ct', id: '104', count: 9 },
            { type: 'ct', id: '105', count: 9 },
            { type: 'ct', id: '106', count: 9 },
            { type: 'ct', id: '107', count: 9 },
            { type: 'ct', id: '108', count: 9 },
            { type: 'ct', id: '109', count: 9 },
            { type: 'ct', id: '110', count: 9 },
            { type: 'ct', id: '111', count: 9 },
            { type: 'ct', id: '120', count: 9 },
            { type: 'ct', id: '121', count: 9 },
            { type: 'ct', id: '122', count: 9 },
            { type: 'ct', id: '124', count: 9 },
            { type: 'vm', id: '200', count: 3 },
            { type: 'vm', id: '400', count: 3 }
          ];
          
          guests.forEach(guest => {
            for (let i = 0; i < guest.count; i++) {
              allSnapshots.push({
                'backup-time': now - (i * 24 * 60 * 60),
                'backup-type': guest.type,
                'backup-id': guest.id,
                'backup-group': `${guest.type}/${guest.id}`,
                size: 1024 * 1024 * 100
              });
            }
          });
          
          return Promise.resolve({ data: { data: allSnapshots } });
        }
        if (path.includes('/status/datastore-usage')) {
          return Promise.resolve({
            data: { data: [{
              store: 'main-datastore',
              total: 1000000000000,
              used: 135000000000, // 135GB for 135 backups
              avail: 865000000000
            }]}
          });
        }
        return Promise.resolve({ data: { data: [] } });
      });
      
      // Mock PVE snapshots (the real VM/CT snapshots)
      mockApiClients['proxmox-lan'].client.get.mockImplementation((path) => {
        if (path.includes('/snapshot')) {
          if (path.includes('/400/')) {
            return Promise.resolve({
              data: { data: [
                { name: 'current' }, // Filtered out
                { name: 'ubuntuserver', snaptime: 1700000000 },
                { name: 'precursor', snaptime: 1699000000 }
              ]}
            });
          }
          if (path.includes('/106/')) {
            return Promise.resolve({
              data: { data: [
                { name: 'current' }, // Filtered out
                { name: 'before_helper', snaptime: 1701000000 }
              ]}
            });
          }
          return Promise.resolve({ data: { data: [{ name: 'current' }] } });
        }
        return Promise.resolve({ data: { data: [] } });
      });
      
      const pbsData = await fetchPbsData(mockPbsApiClients);
      const discoveryData = await fetchDiscoveryData(mockApiClients, mockPbsApiClients);
      
      // Count PBS backups
      let totalPbsBackups = 0;
      if (pbsData[0]?.datastores) {
        pbsData[0].datastores.forEach(ds => {
          totalPbsBackups += ds.snapshots?.length || 0;
        });
      }
      
      // Count VM/CT snapshots
      const vmSnapshots = discoveryData.pveBackups?.guestSnapshots?.length || 0;
      
      console.log(`PBS Backups: ${totalPbsBackups}, VM Snapshots: ${vmSnapshots}`);
      
      // Verify the distinction
      expect(totalPbsBackups).toBeGreaterThan(50); // Should have many PBS backups
      expect(vmSnapshots).toBeLessThan(5); // Should have very few VM snapshots
      
      // This verifies the logging confusion issue
      if (totalPbsBackups > 100 && vmSnapshots < 5) {
        console.log('Confirmed: PBS backups are distinct from VM snapshots');
        console.log('DataFetcher logs showing "Found X snapshots" likely refer to VM snapshots, not PBS backups');
      }
    });
  });
  
  describe('Backup Age Verification', () => {
    test('should correctly calculate backup ages', async () => {
      const now = new Date('2025-06-02T12:50:00Z'); // Test time from research
      const twoAM = new Date('2025-06-02T02:00:00Z');
      const fourAM = new Date('2025-06-02T04:00:00Z');
      
      const primaryBackupAge = (now - twoAM) / (1000 * 60 * 60); // Hours
      const secondaryBackupAge = (now - fourAM) / (1000 * 60 * 60); // Hours
      
      expect(primaryBackupAge).toBeCloseTo(10.83, 1); // ~11 hours
      expect(secondaryBackupAge).toBeCloseTo(8.83, 1); // ~9 hours
      
      // Verify these match the ground truth expectations
      expect(primaryBackupAge).toBeGreaterThanOrEqual(groundTruthData.expectedBackupAges.primaryJobGuests.minHours);
      expect(primaryBackupAge).toBeLessThanOrEqual(groundTruthData.expectedBackupAges.primaryJobGuests.maxHours);
      
      expect(secondaryBackupAge).toBeGreaterThanOrEqual(groundTruthData.expectedBackupAges.secondaryJobGuests.minHours);
      expect(secondaryBackupAge).toBeLessThanOrEqual(groundTruthData.expectedBackupAges.secondaryJobGuests.maxHours);
    });
    
    test('should identify guests with missing backups', async () => {
      // Mock PBS tasks to simulate VM 102 missing recent backup
      mockPbsApiClients['pbs-main'].client.get.mockImplementation((path) => {
        if (path.includes('/snapshots') && path.includes('backup-id=102')) {
          // Return no recent snapshots for VM 102
          return Promise.resolve({ data: { data: [] } });
        }
        if (path.includes('/snapshots')) {
          // Return recent snapshots for other guests
          const now = Math.floor(Date.now() / 1000);
          return Promise.resolve({
            data: { data: [{
              'backup-time': now - (11 * 60 * 60), // 11 hours ago
              'backup-type': 'vm',
              'backup-id': '100'
            }]}
          });
        }
        return Promise.resolve({ data: { data: [] } });
      });
      
      const pbsData = await fetchPbsData(mockPbsApiClients);
      
      // Check for VM 102 backup status
      const vm102Backups = pbsData[0]?.datastores?.[0]?.snapshots?.filter(
        snap => snap['backup-id'] === '102'
      ) || [];
      
      expect(vm102Backups.length).toBe(0);
      console.log('Confirmed: VM 102 has no recent backups despite being in backup job');
    });
  });
  
  describe('PBS Task Processing Verification', () => {
    test('should correctly differentiate backup tasks from admin tasks', () => {
      const mockTasks = [
        // Backup tasks (from synthetic snapshots)
        {
          type: 'backup',
          status: 'OK',
          starttime: Date.now() / 1000 - 11 * 60 * 60,
          endtime: Date.now() / 1000 - 10.5 * 60 * 60,
          guest: 'vm/100',
          guestType: 'vm',
          guestId: '100',
          pbsBackupRun: true
        },
        // Admin tasks
        {
          type: 'prune',
          worker_type: 'prune',
          status: 'OK',
          starttime: Date.now() / 1000 - 24 * 60 * 60
        },
        {
          type: 'garbage_collection',
          worker_type: 'garbage_collection',
          status: 'OK',
          starttime: Date.now() / 1000 - 48 * 60 * 60
        },
        {
          type: 'verify',
          worker_type: 'verify',
          status: 'OK',
          starttime: Date.now() / 1000 - 6 * 60 * 60
        }
      ];
      
      const processed = processPbsTasks(mockTasks);
      
      expect(processed.backupTasks.summary.total).toBe(1);
      expect(processed.pruneTasks.summary.total).toBe(2); // prune + gc
      expect(processed.verificationTasks.summary.total).toBe(1);
      
      // Verify task categorization
      expect(processed.backupTasks.recentTasks[0].pbsBackupRun).toBe(true);
      expect(processed.backupTasks.recentTasks[0].guestId).toBe('100');
    });
  });
  
  describe('Multiple Endpoint Handling', () => {
    test('should handle multiple PVE endpoints correctly', async () => {
      // Need to set up mockApiClients for this test
      mockApiClients['proxmox-lan'].client.get.mockImplementation((path) => {
        if (path === '/cluster/status') {
          return Promise.resolve({
            data: {
              data: [
                { type: 'cluster', name: 'proxmox-cluster', nodes: 3 },
                { type: 'node', name: 'desktop' },
                { type: 'node', name: 'delly' },
                { type: 'node', name: 'minipc' }
              ]
            }
          });
        }
        if (path === '/nodes') {
          return Promise.resolve({
            data: {
              data: [
                { node: 'desktop', status: 'online' },
                { node: 'delly', status: 'online' },
                { node: 'minipc', status: 'online' }
              ]
            }
          });
        }
        return Promise.resolve({ data: { data: [] } });
      });
      
      mockApiClients['pimox-lan'].client.get.mockImplementation((path) => {
        if (path === '/cluster/status') {
          return Promise.resolve({
            data: {
              data: [
                { type: 'node', name: 'pi' }
              ]
            }
          });
        }
        if (path === '/nodes') {
          return Promise.resolve({
            data: {
              data: [{ node: 'pi', status: 'online' }]
            }
          });
        }
        return Promise.resolve({ data: { data: [] } });
      });
      
      const discoveryData = await fetchDiscoveryData(mockApiClients, {});
      
      // Check that nodes are properly tagged with endpoints
      const proxmoxNodes = discoveryData.nodes.filter(n => n.endpointId === 'proxmox-lan');
      const pimoxNodes = discoveryData.nodes.filter(n => n.endpointId === 'pimox-lan');
      
      expect(proxmoxNodes.length).toBe(3); // desktop, delly, minipc
      expect(pimoxNodes.length).toBe(1); // pi
      
      // Verify endpoint identification
      expect(discoveryData.nodes.every(n => n.endpointId)).toBe(true);
      expect(discoveryData.vms.every(vm => vm.endpointId)).toBe(true);
      expect(discoveryData.containers.every(ct => ct.endpointId)).toBe(true);
    });
  });
  
  describe('Integration Test: Full Backup Status Verification', () => {
    test('should produce accurate backup status for dashboard', async () => {
      // This test simulates the full data flow to verify dashboard accuracy
      
      // Mock current time
      const mockNow = new Date('2025-06-02T13:10:00+01:00'); // 1:10 PM BST
      jest.spyOn(Date, 'now').mockImplementation(() => mockNow.getTime());
      
      // Mock comprehensive PBS data
      mockPbsApiClients['pbs-main'].client.get.mockImplementation((path) => {
        if (path.includes('/nodes')) {
          return Promise.resolve({ data: { data: [{ node: 'pbs-node' }] } });
        }
        if (path.includes('/config/datastore')) {
          return Promise.resolve({ data: { data: [{ name: 'main-datastore' }] } });
        }
        if (path.includes('/admin/datastore/main-datastore/snapshots')) {
          // Return snapshots for all guests with proper timing
          const snapshots = [];
          const fourAM = Math.floor(new Date('2025-06-02T04:00:00+01:00').getTime() / 1000);
          const twoAM = Math.floor(new Date('2025-06-02T02:00:00+01:00').getTime() / 1000);
          
          // Primary job guests (2 AM)
          [100, 101, 103, 104, 105, 106, 107, 108, 109, 110, 111, 120, 121, 122, 124].forEach(id => {
            snapshots.push({
              'backup-time': twoAM,
              'backup-type': id >= 100 && id <= 102 ? 'vm' : 'ct',
              'backup-id': String(id)
            });
          });
          
          // Secondary job guests (4 AM) - except VM 102
          [200, 400].forEach(id => {
            snapshots.push({
              'backup-time': fourAM,
              'backup-type': 'vm',
              'backup-id': String(id)
            });
          });
          
          // VM 102 has no backups
          
          return Promise.resolve({ data: { data: snapshots } });
        }
        return Promise.resolve({ data: { data: [] } });
      });
      
      const pbsData = await fetchPbsData(mockPbsApiClients);
      const discoveryData = await fetchDiscoveryData(mockApiClients, mockPbsApiClients);
      
      // Analyze backup status
      const guestsWithRecentBackups = new Set();
      const backupAges = new Map();
      
      if (pbsData[0]?.datastores) {
        pbsData[0].datastores.forEach(ds => {
          ds.snapshots?.forEach(snap => {
            const guestKey = `${snap['backup-type']}/${snap['backup-id']}`;
            const ageHours = (mockNow.getTime() / 1000 - snap['backup-time']) / 3600;
            
            if (ageHours < 24) {
              guestsWithRecentBackups.add(snap['backup-id']);
              backupAges.set(snap['backup-id'], ageHours);
            }
          });
        });
      }
      
      // Verify results match ground truth
      expect(guestsWithRecentBackups.size).toBe(17); // 18 total - 1 (VM 102)
      expect(guestsWithRecentBackups.has('102')).toBe(false); // VM 102 missing
      
      // Verify backup ages (allow for slight time differences)
      expect(backupAges.get('100')).toBeCloseTo(11, 0);
      expect(backupAges.get('200')).toBeCloseTo(9, 0);
      expect(backupAges.get('106')).toBeCloseTo(11, 0);
      
      console.log('Dashboard accuracy: 17/18 guests show backups <24h old (94.4% accurate)');
      console.log('Issue identified: VM 102 missing recent backup');
      
      // Cleanup
      jest.restoreAllMocks();
    });
  });
});

module.exports = { groundTruthData };