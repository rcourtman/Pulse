import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  asNumberOrNull,
  buildStorageCapacity,
  buildStorageSource,
  canonicalStorageIdentityKey,
  getStringArray,
  metricsTargetForStorageResource,
  normalizeStorageResourceHealth,
} from '@/features/storageBackups/storageAdapterCore';

const makeRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord => ({
  id: 'storage-1',
  name: 'tank',
  category: 'pool',
  health: 'healthy',
  location: { label: 'pve1', scope: 'node' },
  capacity: { totalBytes: 100, usedBytes: 40, freeBytes: 60, usagePercent: 40 },
  capabilities: ['capacity'],
  source: {
    platform: 'proxmox-pve',
    family: 'virtualization',
    origin: 'resource',
    adapterId: 'resource-storage',
  },
  observedAt: Date.now(),
  ...overrides,
});

const makeResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'resource-storage-1',
    type: 'storage',
    name: 'local-zfs',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    ...overrides,
  }) as Resource;

describe('storageAdapterCore', () => {
  it('normalizes low-level numeric and string-array inputs', () => {
    expect(asNumberOrNull(12)).toBe(12);
    expect(asNumberOrNull(Number.NaN)).toBeNull();
    expect(getStringArray(['a', 7, 'b', ''])).toEqual(['a', 'b']);
  });

  it('builds canonical identity, source, capacity, and metrics target payloads', () => {
    expect(
      canonicalStorageIdentityKey(
        makeRecord({
          source: { platform: 'truenas', family: 'onprem', origin: 'resource', adapterId: 'x' },
          location: { label: 'tower', scope: 'host' },
          category: 'dataset',
        }),
      ),
    ).toBe('truenas|tower|tank|dataset');

    expect(buildStorageSource('proxmox-pbs', 'resource-storage')).toEqual({
      platform: 'proxmox-pbs',
      family: 'virtualization',
      adapterId: 'resource-storage',
      origin: 'resource',
    });

    expect(buildStorageCapacity(100, 40, 60, 40)).toEqual({
      totalBytes: 100,
      usedBytes: 40,
      freeBytes: 60,
      usagePercent: 40,
    });

    expect(
      metricsTargetForStorageResource(
        makeResource({
          metricsTarget: { resourceType: 'storage', resourceId: 'pool:tank' },
        }),
      ),
    ).toEqual({
      resourceType: 'storage',
      resourceId: 'pool:tank',
    });
  });

  it('derives canonical storage health from incident severity, health tags, and status', () => {
    expect(normalizeStorageResourceHealth('online', ['health:degraded'])).toBe('warning');
    expect(normalizeStorageResourceHealth('offline', ['health:healthy'], 'critical')).toBe(
      'critical',
    );
    expect(normalizeStorageResourceHealth(undefined, undefined)).toBe('unknown');
  });
});
