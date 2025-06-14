#!/usr/bin/env node

/**
 * Backup Validation Runner
 * 
 * This script can be run to validate live backup data against ground truths.
 * Usage: node runBackupValidation.js [--live]
 */

const { fetchDiscoveryData, fetchPbsData } = require('../dataFetcher');
const { processPbsTasks } = require('../pbsUtils');
const { createApiClientInstance } = require('../apiClients');
const { 
  validateAllBackupData, 
  generateValidationReport 
} = require('./backupDataValidator');

// Load config if running against live data
let config = null;
if (process.argv.includes('--live')) {
  try {
    config = require('../config.json');
  } catch (error) {
    console.error('Error loading config.json:', error.message);
    process.exit(1);
  }
}

/**
 * Runs validation against mock data
 */
async function runMockValidation() {
  console.log('Running validation against mock data...\n');
  
  // Create mock data similar to test setup
  const mockDiscoveryData = {
    nodes: [
      { node: 'desktop', endpointId: 'proxmox-lan', status: 'online' },
      { node: 'delly', endpointId: 'proxmox-lan', status: 'online' },
      { node: 'minipc', endpointId: 'proxmox-lan', status: 'online' },
      { node: 'pi', endpointId: 'pimox-lan', status: 'online' }
    ],
    vms: [
      { vmid: 100, name: 'vm100', type: 'qemu', endpointId: 'proxmox-lan' },
      { vmid: 102, name: 'vm102', type: 'qemu', endpointId: 'proxmox-lan' },
      { vmid: 200, name: 'vm200', type: 'qemu', endpointId: 'proxmox-lan' }
    ],
    containers: Array.from({ length: 15 }, (_, i) => ({
      vmid: 103 + i,
      name: `ct${103 + i}`,
      type: 'lxc',
      endpointId: i < 14 ? 'proxmox-lan' : 'pimox-lan'
    })),
    pveBackups: {
      backupTasks: [],
      storageBackups: [],
      guestSnapshots: [
        { name: 'ubuntuserver', vmid: 400, type: 'qemu' },
        { name: 'precursor', vmid: 400, type: 'qemu' },
        { name: 'before_helper', vmid: 106, type: 'lxc' }
      ]
    }
  };
  
  // Create mock PBS data
  const now = Date.now() / 1000;
  const mockPbsData = [{
    pbsEndpointId: 'pbs-main',
    pbsInstanceName: 'PBS Storage',
    status: 'ok',
    datastores: [{
      name: 'main-datastore',
      snapshots: []
    }]
  }];
  
  // Add mock snapshots
  const guests = [100, 103, 104, 105, 106, 200, 400];
  guests.forEach(guestId => {
    const isSecondaryJob = [102, 200, 400].includes(guestId);
    const backupTime = isSecondaryJob 
      ? now - (9 * 60 * 60)  // 9 hours ago
      : now - (11 * 60 * 60); // 11 hours ago
    
    // Skip VM 102 to simulate missing backup
    if (guestId !== 102) {
      mockPbsData[0].datastores[0].snapshots.push({
        'backup-time': backupTime,
        'backup-type': guestId <= 200 ? 'vm' : 'ct',
        'backup-id': String(guestId)
      });
    }
  });
  
  // Create mock PBS tasks
  const mockPbsTasks = mockPbsData[0].datastores[0].snapshots.map(snap => ({
    type: 'backup',
    status: 'OK',
    starttime: snap['backup-time'],
    endtime: snap['backup-time'] + 300,
    guest: `${snap['backup-type']}/${snap['backup-id']}`,
    guestType: snap['backup-type'],
    guestId: snap['backup-id'],
    pbsBackupRun: true
  }));
  
  const processedTasks = processPbsTasks(mockPbsTasks);
  
  // Run validation
  const validationData = {
    discoveryData: mockDiscoveryData,
    pbsData: mockPbsData,
    pbsTasks: mockPbsTasks,
    processedTasks: processedTasks
  };
  
  const report = validateAllBackupData(validationData);
  console.log(generateValidationReport(report));
}

/**
 * Runs validation against live data
 */
async function runLiveValidation() {
  console.log('Running validation against live data...\n');
  
  try {
    // Initialize API clients
    const apiClients = {};
    const pbsApiClients = {};
    
    // Initialize PVE clients
    if (config.pveEndpoints) {
      for (const [key, endpoint] of Object.entries(config.pveEndpoints)) {
        try {
          apiClients[key] = {
            client: await createApiClientInstance({
              ...endpoint,
              type: 'pve'
            }),
            config: endpoint
          };
          console.log(`✓ Connected to PVE endpoint: ${endpoint.name || key}`);
        } catch (error) {
          console.error(`✗ Failed to connect to PVE endpoint ${key}:`, error.message);
        }
      }
    }
    
    // Initialize PBS clients
    if (config.pbsEndpoints) {
      for (const [key, endpoint] of Object.entries(config.pbsEndpoints)) {
        try {
          pbsApiClients[key] = {
            client: await createApiClientInstance({
              ...endpoint,
              type: 'pbs'
            }),
            config: endpoint
          };
          console.log(`✓ Connected to PBS endpoint: ${endpoint.name || key}`);
        } catch (error) {
          console.error(`✗ Failed to connect to PBS endpoint ${key}:`, error.message);
        }
      }
    }
    
    console.log('\nFetching data...');
    
    // Fetch all data
    const [discoveryData, pbsData] = await Promise.all([
      fetchDiscoveryData(apiClients, pbsApiClients),
      fetchPbsData(pbsApiClients)
    ]);
    
    console.log('Processing PBS tasks...');
    
    // Get raw PBS tasks for validation
    let pbsTasks = [];
    if (pbsData[0]?.backupTasks?.recentTasks) {
      pbsTasks = pbsData[0].backupTasks.recentTasks;
    }
    
    // Process tasks
    const processedTasks = processPbsTasks(pbsTasks);
    
    // Run validation
    const validationData = {
      discoveryData,
      pbsData,
      pbsTasks,
      processedTasks
    };
    
    const report = validateAllBackupData(validationData);
    console.log('\n' + generateValidationReport(report));
    
    // Save detailed report if issues found
    if (!report.overallValid || report.warnings.length > 0) {
      const fs = require('fs');
      const reportPath = `backup-validation-${Date.now()}.json`;
      fs.writeFileSync(reportPath, JSON.stringify(report, null, 2));
      console.log(`\nDetailed report saved to: ${reportPath}`);
    }
    
  } catch (error) {
    console.error('Error during live validation:', error);
    process.exit(1);
  }
}

/**
 * Main entry point
 */
async function main() {
  console.log('Pulse Backup Data Validator\n');
  
  if (process.argv.includes('--live')) {
    if (!config) {
      console.error('No config.json found. Cannot run live validation.');
      process.exit(1);
    }
    await runLiveValidation();
  } else {
    await runMockValidation();
    console.log('\nTo run against live data, use: node runBackupValidation.js --live');
  }
}

// Run if called directly
if (require.main === module) {
  main().catch(console.error);
}

module.exports = { runMockValidation, runLiveValidation };