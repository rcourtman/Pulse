import { describe, expect, it } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  getCompactStoragePoolImpactLabel,
  getCompactStoragePoolIssueLabel,
  getStoragePoolStateLabel,
  getStoragePoolStateTitle,
} from '@/features/storageBackups/rowPresentation';

// Minimal valid-enough record. Cast through `unknown` so deliberate gaps/malformed
// values still satisfy the strict StorageRecord shape under the real tsconfig.
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
// getCompactStoragePoolIssueLabel
//
// Shape under test:
//   label = (issueLabel || '').trim()
//   protection = getCompactStoragePoolProtectionLabel(record).trim()
//   if (label && label.toLowerCase() !== 'healthy') {        // OUTER
//     if (protection && protection !== '—' &&
//         protection.toLowerCase() === label.toLowerCase())  // INNER (3 conjuncts)
//       return '—'
//     return label
//   }
//   if (pool?.state && pool.state !== 'ONLINE') return pool.state
//   if (normalizedStatus && !SAFE.includes(normalizedStatus)) return statusLabel || 'Issue'
//   return '—'
// ---------------------------------------------------------------------------
describe('getCompactStoragePoolIssueLabel branch coverage', () => {
  it('returns "—" when a non-healthy issue label matches the derived protection label (case-insensitive)', () => {
    // INNER all-true: protection truthy, protection !== '—', lowercased equality.
    const record = {
      ...baseRecord(),
      issueLabel: 'Scrub Failed',
      protectionLabel: 'scrub failed',
    };
    expect(getCompactStoragePoolIssueLabel(record)).toBe('—');
  });

  it('returns the raw issue label when protection is a present but differing value (INNER false via lowercase mismatch)', () => {
    // protection is a real ('no parity'), non-'—' value that does not equal the issue label.
    const record = {
      ...baseRecord(),
      issueLabel: 'Drive failed',
      protectionLabel: 'No parity',
      protectionReduced: true,
    };
    expect(getCompactStoragePoolIssueLabel(record)).toBe('Drive failed');
  });

  it('returns the raw issue label when protection collapses to "—" (INNER false via the `protection !== "—"` guard)', () => {
    // protectionLabel 'Healthy' -> getCompactStoragePoolProtectionLabel returns '—'.
    const record = {
      ...baseRecord(),
      issueLabel: 'Drive failed',
      protectionLabel: 'Healthy',
    };
    expect(getCompactStoragePoolIssueLabel(record)).toBe('Drive failed');
  });

  it('skips an issue label that is exactly "Healthy" and falls through to a non-safe statusLabel (OUTER false via healthy equality)', () => {
    const record = {
      ...baseRecord(),
      issueLabel: 'Healthy',
      statusLabel: 'Failed',
    };
    expect(getCompactStoragePoolIssueLabel(record)).toBe('Failed');
  });

  it('skips an issue label that is "HEALTHY" in any case (OUTER false via case-insensitive healthy equality)', () => {
    const record = {
      ...baseRecord(),
      issueLabel: 'HEALTHY',
      statusLabel: 'Faulted',
    };
    expect(getCompactStoragePoolIssueLabel(record)).toBe('Faulted');
  });

  it('treats an empty-string issue label as absent and falls through to the zfs pool state', () => {
    const record = {
      ...baseRecord(),
      issueLabel: '',
      details: {
        zfsPool: { state: 'DEGRADED', devices: [] },
      },
    } as unknown as StorageRecord;
    expect(getCompactStoragePoolIssueLabel(record)).toBe('DEGRADED');
  });

  it('treats a whitespace-only issue label as absent (trim -> "") and falls through to statusLabel', () => {
    const record = {
      ...baseRecord(),
      issueLabel: '   ',
      statusLabel: 'Faulted',
    };
    expect(getCompactStoragePoolIssueLabel(record)).toBe('Faulted');
  });

  it('skips an ONLINE zfs pool (pool.state !== "ONLINE" guard false) and uses a non-safe statusLabel', () => {
    const record = {
      ...baseRecord(),
      details: {
        zfsPool: { state: 'ONLINE', devices: [] },
      },
      statusLabel: 'Faulted',
    } as unknown as StorageRecord;
    expect(getCompactStoragePoolIssueLabel(record)).toBe('Faulted');
  });

  it('returns "—" for every "safe" statusLabel value (online/available/running/healthy)', () => {
    const safeValues = ['online', 'available', 'running', 'healthy'];
    for (const statusLabel of safeValues) {
      const record = { ...baseRecord(), statusLabel };
      expect(getCompactStoragePoolIssueLabel(record)).toBe('—');
    }
  });

  it('returns the raw (untrimmed) statusLabel when it is padded but non-safe', () => {
    // normalizedStatus is computed from the trimmed/lowercased value, but the
    // function returns record.statusLabel verbatim -> padded result is a real
    // observable quirk worth pinning.
    const record = {
      ...baseRecord(),
      statusLabel: '  Failed  ',
    };
    expect(getCompactStoragePoolIssueLabel(record)).toBe('  Failed  ');
  });

  it('returns "—" when issue label is healthy, no pool exists, and statusLabel is empty', () => {
    const record = {
      ...baseRecord(),
      issueLabel: 'Healthy',
      statusLabel: '',
    };
    expect(getCompactStoragePoolIssueLabel(record)).toBe('—');
  });
});

// ---------------------------------------------------------------------------
// getStoragePoolStateLabel
//
// Shape under test:
//   arrayState = details.arrayState (string-only, trimmed-by-caller of getRecordStringDetail)
//   if (arrayState) return titleize(arrayState)              // titleize splits on /[\s_-]+/
//   if (pool?.state) return pool.state === 'ONLINE' ? 'Online' : pool.state
//   status = getStorageRecordStatus(record)
//   return status ? titleize(status) : '—'
//
// NOTE: getStorageRecordStatus always returns a non-empty string (final fallback
// 'unknown'), so the trailing `: '—'` arm is unreachable in practice; see report.
// ---------------------------------------------------------------------------
describe('getStoragePoolStateLabel branch coverage', () => {
  it('titleizes a single-word details.arrayState', () => {
    const record = {
      ...baseRecord(),
      details: { arrayState: 'STARTED' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateLabel(record)).toBe('Started');
  });

  it('titleizes a multi-word arrayState by splitting on whitespace', () => {
    const record = {
      ...baseRecord(),
      details: { arrayState: 'PARTIALLY DEGRADED' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateLabel(record)).toBe('Partially Degraded');
  });

  it('titleizes an arrayState that uses underscore separators', () => {
    // Exercises the `/[\s_-]+/` split in titleize with the `_` separator class.
    const record = {
      ...baseRecord(),
      details: { arrayState: 'degraded_online' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateLabel(record)).toBe('Degraded Online');
  });

  it('falls through when details.arrayState is a non-string (defensive getRecordStringDetail branch)', () => {
    // A numeric arrayState is coerced to '' by getRecordStringDetail; with no
    // pool and a healthy record, status resolves to 'available' -> 'Available'.
    const record = {
      ...baseRecord(),
      details: { arrayState: 42 },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateLabel(record)).toBe('Available');
  });

  it('falls through when details.arrayState is the empty string', () => {
    const record = {
      ...baseRecord(),
      details: { arrayState: '' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateLabel(record)).toBe('Available');
  });

  it('returns "Online" when only the zfs pool state is "ONLINE"', () => {
    const record = {
      ...baseRecord(),
      details: { zfsPool: { state: 'ONLINE', devices: [] } },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateLabel(record)).toBe('Online');
  });

  it('returns the raw pool state when it is anything other than "ONLINE"', () => {
    const record = {
      ...baseRecord(),
      details: { zfsPool: { state: 'FAULTED', devices: [] } },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateLabel(record)).toBe('FAULTED');
  });

  it('skips a zfs pool whose state is the empty string (pool?.state truthiness guard false)', () => {
    // toZfsPool accepts state='' (it is a string), but `pool?.state` is falsy,
    // so the function falls through to the status arm.
    const record = {
      ...baseRecord(),
      details: { zfsPool: { state: '', devices: [] } },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateLabel(record)).toBe('Available');
  });

  it('titleizes the health-derived status when arrayState and pool are both absent', () => {
    const record = {
      ...baseRecord(),
      health: 'critical',
    } as unknown as StorageRecord;
    expect(getStoragePoolStateLabel(record)).toBe('Critical');
  });

  it('titleizes an explicit details.status when arrayState and pool are absent', () => {
    const record = {
      ...baseRecord(),
      details: { status: 'available' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateLabel(record)).toBe('Available');
  });

  it('exercises the `details || {}` defensive arm when details is undefined', () => {
    const record = {
      ...baseRecord(),
      details: undefined,
      health: 'warning',
    } as unknown as StorageRecord;
    // health 'warning' -> status 'degraded' -> titleized 'Degraded'.
    expect(getStoragePoolStateLabel(record)).toBe('Degraded');
  });
});

// ---------------------------------------------------------------------------
// getStoragePoolStateTitle
//
// Shape under test:
//   label = getStoragePoolStateLabel(record)
//   summary = getCompactStoragePoolIssueSummary(record).trim() || getStorageRecordIssueSummary(record).trim()
//   if (summary && summary.toLowerCase() !== 'healthy' && label.toLowerCase() !== 'started')
//     return summary
//   return label === '—' ? '' : label
//
// NOTE: getStoragePoolStateLabel never returns '—' (status is always non-empty),
// so the `label === '—' ? ''` arm is unreachable in practice; see report.
// ---------------------------------------------------------------------------
describe('getStoragePoolStateTitle branch coverage', () => {
  it('returns a non-healthy summary when the label is not "Started" (main branch)', () => {
    const record = {
      ...baseRecord(),
      issueLabel: 'Degraded',
      issueSummary: 'One disk is offline',
      details: { arrayState: 'DEGRADED' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateTitle(record)).toBe('One disk is offline');
  });

  it('suppresses the summary when the resolved label is "Started" and returns the label', () => {
    const record = {
      ...baseRecord(),
      issueLabel: 'Degraded',
      issueSummary: 'Some warning',
      details: { arrayState: 'STARTED' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateTitle(record)).toBe('Started');
  });

  it('suppresses a "healthy" summary (case-insensitive) and returns the resolved label', () => {
    // getCompactStoragePoolIssueSummary returns '' (issue summary is 'Healthy'),
    // so summary falls back to getStorageRecordIssueSummary -> 'Healthy' ->
    // 'healthy' -> suppressed.
    const record = {
      ...baseRecord(),
      issueLabel: 'Degraded',
      issueSummary: 'Healthy',
      details: { arrayState: 'DEGRADED' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateTitle(record)).toBe('Degraded');
  });

  it('uses getStorageRecordIssueSummary when getCompactStoragePoolIssueSummary is empty', () => {
    // issueLabel 'Healthy' -> getCompactStoragePoolIssueLabel '—' -> compact
    // summary ''; issueSummary 'Manual maintenance' is non-healthy and the label
    // ('Degraded') is not 'started', so the fallback summary is returned.
    const record = {
      ...baseRecord(),
      issueLabel: 'Healthy',
      issueSummary: 'Manual maintenance',
      details: { arrayState: 'DEGRADED' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateTitle(record)).toBe('Manual maintenance');
  });

  it('returns the resolved label when there is no summary signal at all', () => {
    const record = {
      ...baseRecord(),
      details: { arrayState: 'STARTED' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateTitle(record)).toBe('Started');
  });

  it('returns an empty-ish label verbatim via the trailing `: label` arm when summary is absent', () => {
    // No issue signal; arrayState 'STOPPING' -> label 'Stopping'. Summary '' ->
    // both clauses of the if false -> return label.
    const record = {
      ...baseRecord(),
      details: { arrayState: 'STOPPING' },
    } as unknown as StorageRecord;
    expect(getStoragePoolStateTitle(record)).toBe('Stopping');
  });
});

// ---------------------------------------------------------------------------
// getCompactStoragePoolImpactLabel
//
// Shape under test:
//   if ((consumerCount || 0) > 0 ||
//       (protectedWorkloadCount || 0) > 0 ||
//       (affectedDatastoreCount || 0) > 0)
//     return (impactSummary || '').trim() || '—'
//   return '—'
// ---------------------------------------------------------------------------
describe('getCompactStoragePoolImpactLabel branch coverage', () => {
  it('returns the trimmed impact summary when consumerCount > 0', () => {
    const record = {
      ...baseRecord(),
      consumerCount: 2,
      impactSummary: '2 consumers',
    };
    expect(getCompactStoragePoolImpactLabel(record)).toBe('2 consumers');
  });

  it('trims surrounding whitespace from a present impactSummary', () => {
    const record = {
      ...baseRecord(),
      consumerCount: 1,
      impactSummary: '  5 vms  ',
    };
    expect(getCompactStoragePoolImpactLabel(record)).toBe('5 vms');
  });

  it('enters the block via protectedWorkloadCount > 0 alone and falls to "—" when impactSummary is blank', () => {
    const record = {
      ...baseRecord(),
      protectedWorkloadCount: 1,
      impactSummary: '   ',
    };
    expect(getCompactStoragePoolImpactLabel(record)).toBe('—');
  });

  it('enters the block via affectedDatastoreCount > 0 alone and returns the impact summary', () => {
    const record = {
      ...baseRecord(),
      affectedDatastoreCount: 3,
      impactSummary: '3 datastores affected',
    };
    expect(getCompactStoragePoolImpactLabel(record)).toBe('3 datastores affected');
  });

  it('returns "—" via the inner `|| "—"` when impactSummary is undefined but a count is set', () => {
    const record = {
      ...baseRecord(),
      consumerCount: 1,
      impactSummary: undefined,
    };
    expect(getCompactStoragePoolImpactLabel(record)).toBe('—');
  });

  it('coerces a null consumerCount to 0 via `consumerCount || 0` and still enters via another count', () => {
    const record = {
      ...baseRecord(),
      consumerCount: null,
      protectedWorkloadCount: 2,
      impactSummary: '2 protected',
    } as unknown as StorageRecord;
    expect(getCompactStoragePoolImpactLabel(record)).toBe('2 protected');
  });

  it('returns "—" when all three counts are explicitly zero', () => {
    const record = {
      ...baseRecord(),
      consumerCount: 0,
      protectedWorkloadCount: 0,
      affectedDatastoreCount: 0,
      impactSummary: 'ignored',
    };
    expect(getCompactStoragePoolImpactLabel(record)).toBe('—');
  });

  it('returns "—" when all counts are absent (undefined) even if impactSummary is set', () => {
    const record = {
      ...baseRecord(),
      impactSummary: 'No dependent resources',
    };
    expect(getCompactStoragePoolImpactLabel(record)).toBe('—');
  });

  it('returns "—" when all counts are null and impactSummary is set', () => {
    const record = {
      ...baseRecord(),
      consumerCount: null,
      protectedWorkloadCount: null,
      affectedDatastoreCount: null,
      impactSummary: 'No dependent resources',
    } as unknown as StorageRecord;
    expect(getCompactStoragePoolImpactLabel(record)).toBe('—');
  });
});
