/**
 * Backup Data Validator
 * 
 * This module provides utilities to validate backup data against known ground truths
 * and help identify discrepancies in the backup system.
 */

// Ground truth data based on research
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

/**
 * Validates guest count against expected values
 * @param {Object} discoveryData - The discovery data from fetchDiscoveryData
 * @returns {Object} Validation result with details
 */
function validateGuestCount(discoveryData) {
  const actualVMs = discoveryData.vms?.length || 0;
  const actualContainers = discoveryData.containers?.length || 0;
  const actualTotal = actualVMs + actualContainers;
  
  const result = {
    valid: actualTotal === groundTruthData.totalGuests,
    expected: groundTruthData.totalGuests,
    actual: actualTotal,
    vms: actualVMs,
    containers: actualContainers,
    discrepancy: actualTotal - groundTruthData.totalGuests,
    details: []
  };
  
  if (!result.valid) {
    result.details.push(`Guest count mismatch: Expected ${result.expected}, got ${result.actual}`);
    
    // Check for known issue
    if (actualTotal === 20 && groundTruthData.totalGuests === 18) {
      result.details.push('Known issue: Pulse showing 20 guests instead of actual 18');
    }
  }
  
  // Group by endpoint for detailed analysis
  const guestsByEndpoint = {};
  [...(discoveryData.vms || []), ...(discoveryData.containers || [])].forEach(guest => {
    const endpoint = guest.endpointId || 'unknown';
    if (!guestsByEndpoint[endpoint]) {
      guestsByEndpoint[endpoint] = { vms: 0, containers: 0 };
    }
    if (guest.type === 'qemu') {
      guestsByEndpoint[endpoint].vms++;
    } else {
      guestsByEndpoint[endpoint].containers++;
    }
  });
  
  result.byEndpoint = guestsByEndpoint;
  
  return result;
}

/**
 * Validates PBS backup counts vs VM snapshots
 * @param {Object} pbsData - PBS data from fetchPbsData
 * @param {Object} pveBackups - PVE backup data
 * @returns {Object} Validation result
 */
function validateBackupCounts(pbsData, pveBackups) {
  let pbsBackupCount = 0;
  let pbsBackupsByGuest = {};
  
  // Count PBS backups
  if (pbsData && pbsData[0]?.datastores) {
    pbsData[0].datastores.forEach(ds => {
      (ds.snapshots || []).forEach(snap => {
        pbsBackupCount++;
        const guestKey = `${snap['backup-type']}/${snap['backup-id']}`;
        pbsBackupsByGuest[guestKey] = (pbsBackupsByGuest[guestKey] || 0) + 1;
      });
    });
  }
  
  const vmSnapshotCount = pveBackups?.guestSnapshots?.length || 0;
  
  const result = {
    valid: pbsBackupCount > 100 && vmSnapshotCount < 10, // Expected pattern
    pbsBackups: {
      total: pbsBackupCount,
      expected: groundTruthData.pbsBackupsTotal,
      byGuest: pbsBackupsByGuest
    },
    vmSnapshots: {
      total: vmSnapshotCount,
      expected: groundTruthData.vmSnapshots,
      list: pveBackups?.guestSnapshots || []
    },
    details: []
  };
  
  if (Math.abs(pbsBackupCount - groundTruthData.pbsBackupsTotal) > 10) {
    result.details.push(`PBS backup count differs from expected: ${pbsBackupCount} vs ${groundTruthData.pbsBackupsTotal}`);
  }
  
  if (vmSnapshotCount > groundTruthData.vmSnapshots) {
    result.details.push(`More VM snapshots than expected: ${vmSnapshotCount} vs ${groundTruthData.vmSnapshots}`);
  }
  
  return result;
}

/**
 * Validates backup ages for all guests
 * @param {Object} pbsData - PBS data
 * @param {Date} currentTime - Current time for age calculations
 * @returns {Object} Validation result with age analysis
 */
function validateBackupAges(pbsData, currentTime = new Date()) {
  const backupAges = new Map();
  const guestsWithoutBackups = new Set();
  const expectedGuests = new Set();
  
  // Build expected guest list
  for (let i = 100; i <= 106; i++) {
    expectedGuests.add(String(i));
  }
  for (let i = 200; i <= 400; i += 100) {
    expectedGuests.add(String(i));
  }
  
  // Analyze PBS backups
  if (pbsData && pbsData[0]?.datastores) {
    pbsData[0].datastores.forEach(ds => {
      (ds.snapshots || []).forEach(snap => {
        const backupTime = snap['backup-time'] * 1000; // Convert to milliseconds
        const ageHours = (currentTime.getTime() - backupTime) / (1000 * 60 * 60);
        const guestId = snap['backup-id'];
        
        if (!backupAges.has(guestId) || ageHours < backupAges.get(guestId)) {
          backupAges.set(guestId, ageHours);
        }
      });
    });
  }
  
  // Find guests without recent backups
  expectedGuests.forEach(guestId => {
    if (!backupAges.has(guestId) || backupAges.get(guestId) > 24) {
      guestsWithoutBackups.add(guestId);
    }
  });
  
  // Categorize by backup schedule
  const primaryJobGuests = [];
  const secondaryJobGuests = [];
  const issues = [];
  
  backupAges.forEach((age, guestId) => {
    const id = parseInt(guestId);
    
    if ([102, 200, 400].includes(id)) {
      secondaryJobGuests.push({ id: guestId, age });
      if (age < groundTruthData.expectedBackupAges.secondaryJobGuests.minHours ||
          age > groundTruthData.expectedBackupAges.secondaryJobGuests.maxHours + 1) {
        issues.push(`Guest ${guestId} backup age ${age.toFixed(1)}h outside expected range`);
      }
    } else {
      primaryJobGuests.push({ id: guestId, age });
      if (age < groundTruthData.expectedBackupAges.primaryJobGuests.minHours ||
          age > groundTruthData.expectedBackupAges.primaryJobGuests.maxHours + 1) {
        issues.push(`Guest ${guestId} backup age ${age.toFixed(1)}h outside expected range`);
      }
    }
  });
  
  // Check for VM 102 issue
  if (guestsWithoutBackups.has('102')) {
    issues.push('VM 102 has no recent backup (known issue)');
  }
  
  return {
    valid: issues.length === 0,
    backupAges: Object.fromEntries(backupAges),
    primaryJobGuests,
    secondaryJobGuests,
    guestsWithoutBackups: Array.from(guestsWithoutBackups),
    issues,
    summary: {
      totalGuests: expectedGuests.size,
      guestsWithBackups: backupAges.size,
      guestsWithRecentBackups: Array.from(backupAges.entries())
        .filter(([_, age]) => age < 24).length
    }
  };
}

/**
 * Validates PBS task categorization
 * @param {Array} pbsTasks - Raw PBS tasks
 * @param {Object} processedTasks - Processed tasks from processPbsTasks
 * @returns {Object} Validation result
 */
function validateTaskProcessing(pbsTasks, processedTasks) {
  const result = {
    valid: true,
    totalTasks: pbsTasks?.length || 0,
    categorized: {
      backup: processedTasks.backupTasks?.summary?.total || 0,
      verify: processedTasks.verificationTasks?.summary?.total || 0,
      sync: processedTasks.syncTasks?.summary?.total || 0,
      prune: processedTasks.pruneTasks?.summary?.total || 0
    },
    uncategorized: [],
    issues: []
  };
  
  // Check if all tasks were categorized
  const categorizedTotal = Object.values(result.categorized).reduce((a, b) => a + b, 0);
  
  if (categorizedTotal !== result.totalTasks) {
    result.valid = false;
    result.issues.push(`Task count mismatch: ${categorizedTotal} categorized out of ${result.totalTasks} total`);
    
    // Find uncategorized tasks
    const taskTypeMap = {
      backup: 'backup',
      verify: 'verify',
      sync: 'sync',
      prune: 'prune',
      garbage_collection: 'prune',
      gc: 'prune'
    };
    
    pbsTasks?.forEach(task => {
      const type = task.worker_type || task.type;
      if (!taskTypeMap[type]) {
        result.uncategorized.push(type);
      }
    });
  }
  
  // Check for backup task details
  const backupTasks = processedTasks.backupTasks?.recentTasks || [];
  const pbsBackupTasks = backupTasks.filter(t => t.pbsBackupRun);
  
  if (pbsBackupTasks.length === 0 && result.categorized.backup > 0) {
    result.issues.push('No PBS backup runs found in recent tasks');
  }
  
  return result;
}

/**
 * Performs comprehensive validation of all backup data
 * @param {Object} data - Object containing discoveryData, pbsData, etc.
 * @returns {Object} Complete validation report
 */
function validateAllBackupData(data) {
  const report = {
    timestamp: new Date().toISOString(),
    validations: {},
    overallValid: true,
    criticalIssues: [],
    warnings: []
  };
  
  // Guest count validation
  if (data.discoveryData) {
    report.validations.guestCount = validateGuestCount(data.discoveryData);
    if (!report.validations.guestCount.valid) {
      report.warnings.push('Guest count discrepancy detected');
    }
  }
  
  // Backup count validation
  if (data.pbsData && data.discoveryData?.pveBackups) {
    report.validations.backupCounts = validateBackupCounts(
      data.pbsData,
      data.discoveryData.pveBackups
    );
    if (!report.validations.backupCounts.valid) {
      report.criticalIssues.push('Backup count validation failed');
      report.overallValid = false;
    }
  }
  
  // Backup age validation
  if (data.pbsData) {
    report.validations.backupAges = validateBackupAges(data.pbsData);
    if (!report.validations.backupAges.valid) {
      report.validations.backupAges.issues.forEach(issue => {
        if (issue.includes('VM 102')) {
          report.warnings.push(issue);
        } else {
          report.criticalIssues.push(issue);
          report.overallValid = false;
        }
      });
    }
  }
  
  // Task processing validation
  if (data.pbsTasks && data.processedTasks) {
    report.validations.taskProcessing = validateTaskProcessing(
      data.pbsTasks,
      data.processedTasks
    );
    if (!report.validations.taskProcessing.valid) {
      report.warnings.push('Task processing issues detected');
    }
  }
  
  // Summary
  report.summary = {
    criticalIssues: report.criticalIssues.length,
    warnings: report.warnings.length,
    recommendation: report.overallValid 
      ? 'Backup data appears valid'
      : 'Critical issues found - investigate backup system'
  };
  
  return report;
}

/**
 * Generates a human-readable report from validation results
 * @param {Object} validationReport - Report from validateAllBackupData
 * @returns {String} Formatted report
 */
function generateValidationReport(validationReport) {
  let report = `Backup Data Validation Report
Generated: ${validationReport.timestamp}
========================================

`;

  // Overall Status
  report += `Overall Status: ${validationReport.overallValid ? '✓ PASS' : '✗ FAIL'}\n`;
  report += `Critical Issues: ${validationReport.criticalIssues.length}\n`;
  report += `Warnings: ${validationReport.warnings.length}\n\n`;
  
  // Guest Count
  if (validationReport.validations.guestCount) {
    const gc = validationReport.validations.guestCount;
    report += `Guest Count Validation:\n`;
    report += `  Expected: ${gc.expected} guests\n`;
    report += `  Actual: ${gc.actual} guests (${gc.vms} VMs, ${gc.containers} CTs)\n`;
    if (gc.byEndpoint) {
      report += `  By Endpoint:\n`;
      Object.entries(gc.byEndpoint).forEach(([endpoint, counts]) => {
        report += `    ${endpoint}: ${counts.vms} VMs, ${counts.containers} CTs\n`;
      });
    }
    report += '\n';
  }
  
  // Backup Counts
  if (validationReport.validations.backupCounts) {
    const bc = validationReport.validations.backupCounts;
    report += `Backup Count Validation:\n`;
    report += `  PBS Backups: ${bc.pbsBackups.total} (expected ~${bc.pbsBackups.expected})\n`;
    report += `  VM Snapshots: ${bc.vmSnapshots.total} (expected ${bc.vmSnapshots.expected})\n`;
    report += '\n';
  }
  
  // Backup Ages
  if (validationReport.validations.backupAges) {
    const ba = validationReport.validations.backupAges;
    report += `Backup Age Validation:\n`;
    report += `  Total Guests: ${ba.summary.totalGuests}\n`;
    report += `  Guests with backups: ${ba.summary.guestsWithBackups}\n`;
    report += `  Guests with recent backups (<24h): ${ba.summary.guestsWithRecentBackups}\n`;
    if (ba.guestsWithoutBackups.length > 0) {
      report += `  Guests without recent backups: ${ba.guestsWithoutBackups.join(', ')}\n`;
    }
    report += '\n';
  }
  
  // Issues
  if (validationReport.criticalIssues.length > 0) {
    report += `Critical Issues:\n`;
    validationReport.criticalIssues.forEach(issue => {
      report += `  - ${issue}\n`;
    });
    report += '\n';
  }
  
  if (validationReport.warnings.length > 0) {
    report += `Warnings:\n`;
    validationReport.warnings.forEach(warning => {
      report += `  - ${warning}\n`;
    });
    report += '\n';
  }
  
  report += `Recommendation: ${validationReport.summary.recommendation}\n`;
  
  return report;
}

module.exports = {
  validateGuestCount,
  validateBackupCounts,
  validateBackupAges,
  validateTaskProcessing,
  validateAllBackupData,
  generateValidationReport
};