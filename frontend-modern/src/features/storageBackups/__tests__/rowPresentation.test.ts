import { describe, expect, it } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  getCompactStoragePoolImpactLabel,
  getCompactStoragePoolIssueLabel,
  getCompactStoragePoolIssueSummary,
  getCompactStoragePoolProtectionLabel,
  getCompactStoragePoolProtectionTitle,
  getStoragePoolIssueTextClass,
  getStoragePoolProtectionTextClass,
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

describe('storage row presentation', () => {
  it('returns rebuild protection tone when storage is rebuilding', () => {
    const record = { ...baseRecord(), rebuildInProgress: true };
    expect(getStoragePoolProtectionTextClass(record)).toBe('text-blue-700 dark:text-blue-300');
  });

  it('returns recoverability tone when protection is reduced', () => {
    const record = { ...baseRecord(), protectionReduced: true };
    expect(getStoragePoolProtectionTextClass(record)).toBe('text-red-700 dark:text-red-300');
  });

  it('returns warning issue tone for warning storage rows', () => {
    const record = { ...baseRecord(), incidentSeverity: 'warning' };
    expect(getStoragePoolIssueTextClass(record)).toBe('text-amber-700 dark:text-amber-300');
  });

  it('suppresses healthy/default protection and impact labels', () => {
    const record = {
      ...baseRecord(),
      protectionLabel: 'Healthy',
      impactSummary: 'No dependent resources',
    };
    expect(getCompactStoragePoolProtectionLabel(record)).toBe('—');
    expect(getCompactStoragePoolImpactLabel(record)).toBe('—');
  });

  it('keeps actionable issue labels and summaries canonical', () => {
    const record = {
      ...baseRecord(),
      issueLabel: 'Protection Reduced',
      issueSummary: 'Parity lost on array',
      protectionLabel: 'Unprotected',
      protectionSummary: 'Parity lost on array',
      protectionReduced: true,
      incidentSeverity: 'critical',
    };
    expect(getCompactStoragePoolIssueLabel(record)).toBe('Protection Reduced');
    expect(getCompactStoragePoolIssueSummary(record)).toBe('Parity lost on array');
    expect(getCompactStoragePoolProtectionTitle(record)).toBe('Parity lost on array');
  });

  it('keeps protection title aligned with cell label and never reuses a mismatched issue summary', () => {
    const record = {
      ...baseRecord(),
      issueLabel: 'No parity protection',
      issueSummary: 'Unraid array is running without parity protection',
      protectionLabel: 'Parity check',
      protectionSummary: 'Unraid array is running check',
      protectionReduced: true,
      rebuildInProgress: true,
      incidentSeverity: 'warning',
    };
    expect(getCompactStoragePoolProtectionLabel(record)).toBe('Parity check');
    expect(getCompactStoragePoolProtectionTitle(record)).toBe('Unraid array is running check');
    expect(getCompactStoragePoolProtectionTitle(record)).not.toContain('without parity');
  });

  it('derives zfs issue fallback from pool state and errors', () => {
    const record = {
      ...baseRecord(),
      details: {
        zfsPool: {
          state: 'DEGRADED',
          devices: [],
          readErrors: 2,
          checksumErrors: 1,
        },
      },
    } as unknown as StorageRecord;
    expect(getCompactStoragePoolIssueLabel(record)).toBe('DEGRADED');
    expect(getCompactStoragePoolIssueSummary(record)).toBe('2 read, 1 checksum errors');
  });
});
