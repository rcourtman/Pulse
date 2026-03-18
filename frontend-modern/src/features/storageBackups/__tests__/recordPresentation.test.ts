import { describe, expect, it } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  getStorageRecordActionSummary,
  getStorageRecordHostLabel,
  getStorageRecordIssueLabel,
  getStorageRecordNodeHints,
  getStorageRecordPlatformLabel,
  getStorageRecordShared,
  getStorageRecordStats,
  getStorageRecordStatus,
  getStorageRecordUsagePercent,
  getStorageRecordZfsPool,
} from '@/features/storageBackups/recordPresentation';

const makeRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord => ({
  id: 'storage-1',
  name: 'tank',
  category: 'pool',
  health: 'healthy',
  location: { label: 'truenas01/pool/tank', scope: 'host' },
  capacity: { totalBytes: 1_000, usedBytes: 400, freeBytes: 600, usagePercent: 40 },
  capabilities: ['capacity', 'health'],
  source: {
    platform: 'truenas',
    family: 'onprem',
    origin: 'resource',
    adapterId: 'resource-storage',
  },
  observedAt: Date.now(),
  ...overrides,
});

describe('recordPresentation', () => {
  it('derives canonical node hints and labels from storage details', () => {
    const record = makeRecord({
      hostLabel: '',
      platformLabel: '',
      details: {
        node: 'truenas01',
        parentName: 'tower',
        nodeHints: ['cluster-a', 'tower'],
      },
      refs: { platformEntityId: 'node-7' },
    });

    expect(getStorageRecordNodeHints(record)).toEqual([
      'truenas01',
      'tower',
      'cluster-a',
      'tower',
      'truenas01',
      'truenas01/pool/tank',
      'node-7',
    ]);
    expect(getStorageRecordHostLabel(record)).toBe('tower');
    expect(getStorageRecordPlatformLabel(record)).toBe('TrueNAS');
  });

  it('falls back to canonical default summaries when storage posture is absent', () => {
    const record = makeRecord({
      issueLabel: '',
      actionSummary: '',
      details: {},
    });

    expect(getStorageRecordStatus(record)).toBe('available');
    expect(getStorageRecordIssueLabel(record)).toBe('Healthy');
    expect(getStorageRecordActionSummary(record)).toBe('Monitor');
  });

  it('calculates usage and shared stats canonically', () => {
    const sharedA = makeRecord({
      id: 'a',
      name: 'main',
      source: { platform: 'proxmox-pbs', family: 'onprem', origin: 'resource', adapterId: 'a' },
      capacity: { totalBytes: 1_000, usedBytes: 500, freeBytes: 500, usagePercent: null },
      details: { shared: true },
    });
    const sharedB = makeRecord({
      id: 'b',
      name: 'main',
      source: { platform: 'proxmox-pbs', family: 'onprem', origin: 'resource', adapterId: 'b' },
      capacity: { totalBytes: 1_000, usedBytes: 500, freeBytes: 500, usagePercent: null },
      details: { shared: true },
    });
    const local = makeRecord({
      id: 'c',
      name: 'local-zfs',
      health: 'warning',
      capacity: { totalBytes: 400, usedBytes: 200, freeBytes: 200, usagePercent: null },
    });

    expect(getStorageRecordShared(sharedA)).toBe(true);
    expect(getStorageRecordUsagePercent(sharedA)).toBe(50);
    expect(getStorageRecordStats([sharedA, sharedB, local])).toEqual({
      totalBytes: 1_400,
      usedBytes: 700,
      usagePercent: 50,
      byHealth: {
        healthy: 2,
        warning: 1,
        critical: 0,
        offline: 0,
        unknown: 0,
      },
    });
  });

  it('returns canonical zfs pool payloads only when the detail is valid', () => {
    const valid = makeRecord({
      details: {
        zfsPool: {
          state: 'DEGRADED',
          devices: [],
        },
      },
    });
    const invalid = makeRecord({
      details: {
        zfsPool: {
          state: 7,
          devices: [],
        },
      },
    });

    expect(getStorageRecordZfsPool(valid)).toEqual({
      state: 'DEGRADED',
      devices: [],
    });
    expect(getStorageRecordZfsPool(invalid)).toBeNull();
  });
});
