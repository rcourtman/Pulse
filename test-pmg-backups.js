#!/usr/bin/env node

// Test script to verify PMG backup detection
// This simulates various PMG backup scenarios to ensure they're detected as "Host" type

const testCases = [
  // PBS backup formats (what PBS returns)
  {
    name: "PBS PMG backup (ct type with VMID 0)",
    backup: {
      backupType: "ct",
      vmid: "0",
      comment: "PMG host configuration backup",
      instance: "pbs-main"
    },
    expected: "Host"
  },
  {
    name: "PBS PMG backup (ct type with numeric VMID 0)",
    backup: {
      backupType: "ct", 
      vmid: 0,
      comment: "PMG host configuration backup",
      instance: "pbs-main"
    },
    expected: "Host"
  },
  {
    name: "PBS regular LXC backup (ct type with non-zero VMID)",
    backup: {
      backupType: "ct",
      vmid: "100",
      comment: "Regular container backup",
      instance: "pbs-main"
    },
    expected: "LXC"
  },
  
  // Storage backup formats (what PVE storage returns)
  {
    name: "Storage PMG backup (host type)",
    backup: {
      type: "host",
      vmid: 0,
      volid: "local:backup/pmgbackup-pmg-01-2024_01_15.tar.zst",
      notes: "PMG host config backup"
    },
    expected: "Host"
  },
  {
    name: "Storage PMG backup (lxc type with VMID 0)",
    backup: {
      type: "lxc",
      vmid: 0,
      volid: "local:backup/vzdump-lxc-0-2024_01_15.tar.gz",
      notes: "PMG configuration"
    },
    expected: "Host"
  },
  {
    name: "Storage regular LXC backup",
    backup: {
      type: "lxc",
      vmid: 101,
      volid: "local:backup/vzdump-lxc-101-2024_01_15.tar.gz",
      notes: "Container backup"
    },
    expected: "LXC"
  }
];

// Test function that mimics the frontend logic
function detectBackupType(backup) {
  // For PBS backups (have backupType field)
  if ('backupType' in backup) {
    // Check for VMID=0 which indicates host backup (handle both string and number)
    const isVmidZero = backup.vmid === '0' || backup.vmid === 0 || parseInt(String(backup.vmid)) === 0;
    
    if (isVmidZero || backup.backupType === 'host') {
      return 'Host';
    } else if (backup.backupType === 'vm' || backup.backupType === 'VM') {
      return 'VM';
    } else if (backup.backupType === 'ct' || backup.backupType === 'lxc') {
      return 'LXC';
    } else {
      return 'LXC'; // Default fallback
    }
  }
  
  // For storage backups (have type field)
  if ('type' in backup) {
    // Check for VMID=0 which indicates host backup
    const isVmidZero = backup.vmid === 0 || backup.vmid === '0' || parseInt(String(backup.vmid)) === 0;
    
    if (isVmidZero || backup.type === 'host') {
      return 'Host';
    } else if (backup.type === 'qemu' || backup.type === 'vm') {
      return 'VM';
    } else if (backup.type === 'lxc' || backup.type === 'ct') {
      return 'LXC';
    } else {
      return 'LXC'; // Default fallback
    }
  }
  
  return 'Unknown';
}

// Run tests
console.log('Testing PMG Backup Detection Logic\n');
console.log('='.repeat(50));

let passed = 0;
let failed = 0;

testCases.forEach(test => {
  const result = detectBackupType(test.backup);
  const success = result === test.expected;
  
  if (success) {
    console.log(`✓ ${test.name}`);
    console.log(`  Input: ${JSON.stringify(test.backup)}`);
    console.log(`  Expected: ${test.expected}, Got: ${result}\n`);
    passed++;
  } else {
    console.log(`✗ ${test.name}`);
    console.log(`  Input: ${JSON.stringify(test.backup)}`);
    console.log(`  Expected: ${test.expected}, Got: ${result} ← FAILED\n`);
    failed++;
  }
});

console.log('='.repeat(50));
console.log(`Results: ${passed} passed, ${failed} failed`);

if (failed > 0) {
  console.log('\n⚠️  Some tests failed! The detection logic needs adjustment.');
  process.exit(1);
} else {
  console.log('\n✅ All tests passed! The detection logic should work correctly.');
  process.exit(0);
}