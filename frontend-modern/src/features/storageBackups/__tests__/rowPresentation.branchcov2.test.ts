import { describe, expect, it } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  getCompactStoragePoolImpactLabel,
  getCompactStoragePoolIssueLabel,
  getCompactStoragePoolIssueSummary,
  getStoragePoolIssueTextClass,
  getStoragePoolProtectionTextClass,
  getStoragePoolStateLabel,
  getStoragePoolStateTextClass,
  getStoragePoolStateTitle,
} from '@/features/storageBackups/rowPresentation';

const baseRecord = (): StorageRecord =>
  ({
    id: 'storage-1',
    name: 'tank',
    source: {
      platform: 'truenas',
      type: 'storage',
      label: 'TrueNAS',
    },
    category: 'pool',
    statusLabel: 'Healthy',
    health: 'healthy',
    capacity: { totalBytes: 0, usedBytes: 0, freeBytes: 0 },
    location: { label: 'tower', scope: 'node' },
    capabilities: ['capacity'],
    observedAt: Date.now(),
    refs: {},
  }) as unknown as StorageRecord;

// ---------------------------------------------------------------------------
// getStoragePoolIssueTextClass
// ---------------------------------------------------------------------------

describe('getStoragePoolIssueTextClass branch coverage', () => {
  it('returns red tone when incidentSeverity is "critical"', () => {
    const record = { ...baseRecord(), incidentSeverity: 'critical' };
    expect(getStoragePoolIssueTextClass(record)).toBe('text-red-700 dark:text-red-300');
  });

  it('returns red tone when incidentSeverity is "offline"', () => {
    const record = { ...baseRecord(), incidentSeverity: 'offline' };
    expect(getStoragePoolIssueTextClass(record)).toBe('text-red-700 dark:text-red-300');
  });

  it('falls back to record.health when incidentSeverity is absent and lowercases it', () => {
    // "  WARNING  " exercises both the `record.health` arm of the `||` chain and trim()/toLowerCase().
    const record = {
      ...baseRecord(),
      incidentSeverity: undefined,
      health: '  WARNING  ',
    } as unknown as StorageRecord;
    expect(getStoragePoolIssueTextClass(record)).toBe('text-amber-700 dark:text-amber-300');
  });

  it('falls back to the empty-string default when both incidentSeverity and health are falsy', () => {
    // Drives the `|| ''` tail of the `||` chain so severity becomes ''.
    const record = {
      ...baseRecord(),
      incidentSeverity: undefined,
      health: '' as unknown as StorageRecord['health'],
    } as unknown as StorageRecord;
    expect(getStoragePoolIssueTextClass(record)).toBe('text-base-content');
  });

  it('returns base-content for an unmatched severity like "unknown"', () => {
    const record = { ...baseRecord(), incidentSeverity: 'info' };
    expect(getStoragePoolIssueTextClass(record)).toBe('text-base-content');
  });
});

// ---------------------------------------------------------------------------
// getCompactStoragePoolIssueSummary
// ---------------------------------------------------------------------------

describe('getCompactStoragePoolIssueSummary branch coverage', () => {
  it('returns "" when the issue label resolves to "—"', () => {
    // Default healthy record: getCompactStoragePoolIssueLabel -> '—'.
    expect(getCompactStoragePoolIssueSummary(baseRecord())).toBe('');
  });

  it('returns the canonical issue summary when it is present and not "healthy"', () => {
    const record = {
      ...baseRecord(),
      issueLabel: 'Degraded',
      issueSummary: 'Pool is degraded',
    };
    expect(getCompactStoragePoolIssueSummary(record)).toBe('Pool is degraded');
  });

  it('falls through a "healthy" summary to the zfs read-errors branch', () => {
    const record = {
      ...baseRecord(),
      issueLabel: 'Healthy',
      issueSummary: 'Healthy',
      details: {
        zfsPool: {
          state: 'DEGRADED',
          devices: [],
          readErrors: 3,
          writeErrors: 0,
          checksumErrors: 0,
        },
      },
    } as unknown as StorageRecord;
    expect(getCompactStoragePoolIssueSummary(record)).toBe('3 read errors');
  });

  it('formats only write errors when read/checksum are zero', () => {
    const record = {
      ...baseRecord(),
      issueLabel: 'Healthy',
      issueSummary: '  ',
      details: {
        zfsPool: {
          state: 'DEGRADED',
          devices: [],
          readErrors: 0,
          writeErrors: 4,
          checksumErrors: 0,
        },
      },
    } as unknown as StorageRecord;
    expect(getCompactStoragePoolIssueSummary(record)).toBe('4 write errors');
  });

  it('combines all three error kinds in the documented order', () => {
    const record = {
      ...baseRecord(),
      issueLabel: 'Healthy',
      issueSummary: '',
      details: {
        zfsPool: {
          state: 'FAULTED',
          devices: [],
          readErrors: 1,
          writeErrors: 2,
          checksumErrors: 3,
        },
      },
    } as unknown as StorageRecord;
    expect(getCompactStoragePoolIssueSummary(record)).toBe('1 read, 2 write, 3 checksum errors');
  });

  it('returns "" when the pool is absent and the issue label is derived from a non-safe status', () => {
    // getCompactStoragePoolIssueLabel: empty issueLabel, no pool, statusLabel 'Failed' -> 'Failed' (not '—').
    // Then summary: issueSummary empty -> pool null -> ''.
    const record = {
      ...baseRecord(),
      statusLabel: 'Failed',
    } as unknown as StorageRecord;
    expect(getCompactStoragePoolIssueSummary(record)).toBe('');
  });

  it('returns "" when the pool exists but has no errors and the summary is "healthy"', () => {
    const record = {
      ...baseRecord(),
      issueLabel: 'Healthy',
      issueSummary: 'Healthy',
      details: {
        zfsPool: {
          state: 'DEGRADED',
          devices: [],
          readErrors: 0,
          writeErrors: 0,
          checksumErrors: 0,
        },
      },
    } as unknown as StorageRecord;
    expect(getCompactStoragePoolIssueSummary(record)).toBe('');
  });
});

// ---------------------------------------------------------------------------
// getStoragePoolProtectionTextClass
// ---------------------------------------------------------------------------

describe('getStoragePoolProtectionTextClass branch coverage', () => {
  it('returns the recoverability red tone via incidentCategory alone', () => {
    const record = {
      ...baseRecord(),
      incidentCategory: 'recoverability',
    };
    expect(getStoragePoolProtectionTextClass(record)).toBe('text-red-700 dark:text-red-300');
  });

  it('returns base-content for a healthy, fully-protected pool', () => {
    const record = {
      ...baseRecord(),
      protectionLabel: 'Healthy',
    };
    expect(getStoragePoolProtectionTextClass(record)).toBe('text-base-content');
  });

  it('returns the rebuild blue tone regardless of protection label', () => {
    const record = {
      ...baseRecord(),
      rebuildInProgress: true,
      protectionLabel: 'No parity',
    };
    expect(getStoragePoolProtectionTextClass(record)).toBe('text-blue-700 dark:text-blue-300');
  });

  it('treats "no parity" as factual (base-content) even when protectionReduced is set', () => {
    const record = {
      ...baseRecord(),
      protectionLabel: 'No parity',
      protectionReduced: true,
    };
    expect(getStoragePoolProtectionTextClass(record)).toBe('text-base-content');
  });
});

// ---------------------------------------------------------------------------
// getStoragePoolStateLabel
// ---------------------------------------------------------------------------

describe('getStoragePoolStateLabel branch coverage', () => {
  it('titleizes a present details.arrayState', () => {
    const record = {
      ...baseRecord(),
      details: { arrayState: 'STARTED' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateLabel(record)).toBe('Started');
  });

  it('returns "Online" when only the zfs pool state is "ONLINE"', () => {
    const record = {
      ...baseRecord(),
      details: {
        zfsPool: {
          state: 'ONLINE',
          devices: [],
        },
      },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateLabel(record)).toBe('Online');
  });

  it('returns the raw pool state when it is anything other than "ONLINE"', () => {
    const record = {
      ...baseRecord(),
      details: {
        zfsPool: {
          state: 'FAULTED',
          devices: [],
        },
      },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateLabel(record)).toBe('FAULTED');
  });

  it('titleizes the derived status when arrayState and pool are both absent', () => {
    // No details.status; health 'critical' -> getStorageRecordStatus -> 'critical' -> 'Critical'.
    const record = {
      ...baseRecord(),
      health: 'critical',
    } as unknown as StorageRecord;
    expect(getStoragePoolStateLabel(record)).toBe('Critical');
  });

  it('reads details.status (titleized) when arrayState and pool are absent', () => {
    const record = {
      ...baseRecord(),
      details: { status: 'available' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateLabel(record)).toBe('Available');
  });

  it('exercises the `details || {}` defensive arm when details is undefined', () => {
    // With no details at all, getRecordDetails returns {} via the `|| {}` branch;
    // arrayState -> '' -> pool none -> status from health 'warning' -> 'Degraded'.
    const record = {
      ...baseRecord(),
      details: undefined,
      health: 'warning',
    } as unknown as StorageRecord;
    expect(getStoragePoolStateLabel(record)).toBe('Degraded');
  });
});

// ---------------------------------------------------------------------------
// getCompactStoragePoolIssueLabel
// ---------------------------------------------------------------------------

describe('getCompactStoragePoolIssueLabel branch coverage', () => {
  it('collapses to "—" when the issue label matches the derived protection label', () => {
    const record = {
      ...baseRecord(),
      issueLabel: 'No parity',
      protectionLabel: 'No parity',
      protectionReduced: true,
    };
    expect(getCompactStoragePoolIssueLabel(record)).toBe('—');
  });

  it('returns the raw issue label when it differs from the protection label', () => {
    const record = {
      ...baseRecord(),
      issueLabel: 'Scrub failed',
      protectionLabel: 'Healthy',
    };
    expect(getCompactStoragePoolIssueLabel(record)).toBe('Scrub failed');
  });

  it('falls back to the zfs pool state when the issue label is empty and pool is not ONLINE', () => {
    const record = {
      ...baseRecord(),
      details: {
        zfsPool: {
          state: 'DEGRADED',
          devices: [],
        },
      },
    } as unknown as StorageRecord;
    expect(getCompactStoragePoolIssueLabel(record)).toBe('DEGRADED');
  });

  it('skips an ONLINE zfs pool and falls back to a non-safe statusLabel', () => {
    const record = {
      ...baseRecord(),
      details: {
        zfsPool: {
          state: 'ONLINE',
          devices: [],
        },
      },
      statusLabel: 'Faulted',
    } as unknown as StorageRecord;
    expect(getCompactStoragePoolIssueLabel(record)).toBe('Faulted');
  });

  it('returns "—" when issue label, non-ONLINE pool, and a safe status are all absent', () => {
    const record = {
      ...baseRecord(),
      statusLabel: 'online',
    } as unknown as StorageRecord;
    expect(getCompactStoragePoolIssueLabel(record)).toBe('—');
  });

  it('returns "—" for a default healthy record with no issue signal', () => {
    expect(getCompactStoragePoolIssueLabel(baseRecord())).toBe('—');
  });
});

// ---------------------------------------------------------------------------
// getStoragePoolStateTextClass
// ---------------------------------------------------------------------------

describe('getStoragePoolStateTextClass branch coverage', () => {
  it('maps a "critical" state label to the red tone', () => {
    const record = {
      ...baseRecord(),
      details: { arrayState: 'CRITICAL' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateTextClass(record)).toBe('text-red-700 dark:text-red-300');
  });

  it('maps a raw "FAULTED" pool state to the red tone', () => {
    const record = {
      ...baseRecord(),
      details: {
        zfsPool: {
          state: 'FAULTED',
          devices: [],
        },
      },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateTextClass(record)).toBe('text-red-700 dark:text-red-300');
  });

  it('maps an "offline" derived status to the red tone', () => {
    const record = {
      ...baseRecord(),
      health: 'offline',
    } as unknown as StorageRecord;
    expect(getStoragePoolStateTextClass(record)).toBe('text-red-700 dark:text-red-300');
  });

  it('maps a "Warning" arrayState to the amber tone', () => {
    const record = {
      ...baseRecord(),
      details: { arrayState: 'WARNING' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateTextClass(record)).toBe('text-amber-700 dark:text-amber-300');
  });

  it('maps a "Warn" arrayState (the warn alias) to the amber tone', () => {
    const record = {
      ...baseRecord(),
      details: { arrayState: 'WARN' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateTextClass(record)).toBe('text-amber-700 dark:text-amber-300');
  });

  it('maps a "Degraded" derived status (health warning) to the amber tone', () => {
    const record = {
      ...baseRecord(),
      health: 'warning',
    } as unknown as StorageRecord;
    expect(getStoragePoolStateTextClass(record)).toBe('text-amber-700 dark:text-amber-300');
  });

  it('keeps a healthy/online state at base-content', () => {
    const record = {
      ...baseRecord(),
      details: { arrayState: 'STARTED' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateTextClass(record)).toBe('text-base-content');
  });
});

// ---------------------------------------------------------------------------
// getStoragePoolStateTitle
// ---------------------------------------------------------------------------

describe('getStoragePoolStateTitle branch coverage', () => {
  it('prefers a non-healthy summary when the label is not "Started"', () => {
    const record = {
      ...baseRecord(),
      issueLabel: 'Degraded',
      issueSummary: 'One disk is offline',
      details: { arrayState: 'DEGRADED' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateTitle(record)).toBe('One disk is offline');
  });

  it('returns the label when the summary equals the label "Started" (suppresses summary)', () => {
    const record = {
      ...baseRecord(),
      issueLabel: 'Degraded',
      issueSummary: 'Some warning',
      details: { arrayState: 'STARTED' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateTitle(record)).toBe('Started');
  });

  it('falls back to record.issueSummary when getCompactStoragePoolIssueSummary is empty', () => {
    // issueLabel 'Healthy' -> getCompactStoragePoolIssueSummary returns '' (label is '—' upstream).
    // But issueSummary is non-healthy, so the `|| getStorageRecordIssueSummary` arm feeds the summary.
    const record = {
      ...baseRecord(),
      issueLabel: 'Healthy',
      issueSummary: 'Manual maintenance',
    } as unknown as StorageRecord;
    expect(getStoragePoolStateTitle(record)).toBe('Manual maintenance');
  });

  it('returns the resolved label when there is no summary at all', () => {
    const record = {
      ...baseRecord(),
      details: { arrayState: 'STARTED' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateTitle(record)).toBe('Started');
  });
});

// ---------------------------------------------------------------------------
// getCompactStoragePoolImpactLabel
// ---------------------------------------------------------------------------

describe('getCompactStoragePoolImpactLabel branch coverage', () => {
  it('returns the impact summary when consumerCount > 0', () => {
    const record = {
      ...baseRecord(),
      consumerCount: 2,
      impactSummary: '2 consumers',
    };
    expect(getCompactStoragePoolImpactLabel(record)).toBe('2 consumers');
  });

  it('falls back to "—" when protectedWorkloadCount > 0 but impactSummary is blank', () => {
    const record = {
      ...baseRecord(),
      protectedWorkloadCount: 1,
      impactSummary: '   ',
    };
    expect(getCompactStoragePoolImpactLabel(record)).toBe('—');
  });

  it('returns the impact summary when affectedDatastoreCount > 0', () => {
    const record = {
      ...baseRecord(),
      affectedDatastoreCount: 3,
      impactSummary: '3 datastores affected',
    };
    expect(getCompactStoragePoolImpactLabel(record)).toBe('3 datastores affected');
  });

  it('returns "—" when all three counts are zero', () => {
    const record = {
      ...baseRecord(),
      consumerCount: 0,
      protectedWorkloadCount: 0,
      affectedDatastoreCount: 0,
      impactSummary: 'ignored',
    };
    expect(getCompactStoragePoolImpactLabel(record)).toBe('—');
  });

  it('returns "—" when all counts are absent and impactSummary is set', () => {
    const record = {
      ...baseRecord(),
      impactSummary: 'No dependent resources',
    };
    expect(getCompactStoragePoolImpactLabel(record)).toBe('—');
  });
});
