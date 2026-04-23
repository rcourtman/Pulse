import { describe, expect, it } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  buildStoragePoolRowModel,
  STORAGE_POOL_ROW_GROWTH_TEXT_CLASS,
  STORAGE_POOL_ROW_CLASS,
  STORAGE_POOL_ROW_EXPANDED_CLASS,
  STORAGE_POOL_ROW_HEIGHT_CLASS,
  STORAGE_POOL_ROW_NAME_TEXT_CLASS,
  STORAGE_POOL_ROW_PLACEHOLDER_CLASS,
  STORAGE_POOL_ROW_SOURCE_BADGE_CLASS,
  STORAGE_POOL_ROW_USAGE_FALLBACK_CLASS,
} from '@/features/storageBackups/storagePoolRowPresentation';

const baseRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord =>
  ({
    id: 'storage-1',
    name: 'tank',
    source: {
      platform: 'proxmox-pbs',
      type: 'storage',
      label: 'PBS',
    },
    category: 'backup-repository',
    health: 'warning',
    statusLabel: 'Degraded',
    hostLabel: 'pbs01',
    topologyLabel: 'Datastore',
    protectionLabel: 'Protection Reduced',
    issueLabel: 'Capacity Pressure',
    issueSummary: 'Datastore nearly full',
    impactSummary: 'Puts backups for 2 protected workloads at risk',
    location: { label: 'pbs01', scope: 'host' },
    capacity: { totalBytes: 1000, usedBytes: 800, freeBytes: null, usagePercent: 80 },
    capabilities: [],
    observedAt: 0,
    refs: {},
    details: {},
    ...overrides,
  }) as StorageRecord;

describe('storage pool row presentation', () => {
  it('builds canonical row identity and summary fields', () => {
    expect(STORAGE_POOL_ROW_CLASS).toContain('cursor-pointer');
    expect(STORAGE_POOL_ROW_HEIGHT_CLASS).toBe('h-[38px]');
    expect(STORAGE_POOL_ROW_NAME_TEXT_CLASS).toContain('font-semibold');
    expect(STORAGE_POOL_ROW_SOURCE_BADGE_CLASS).toContain('text-[9px]');
    expect(STORAGE_POOL_ROW_EXPANDED_CLASS).toBe('bg-surface-alt');
    expect(STORAGE_POOL_ROW_GROWTH_TEXT_CLASS).toContain('font-mono');
    expect(STORAGE_POOL_ROW_PLACEHOLDER_CLASS).toBe('text-muted');
    expect(STORAGE_POOL_ROW_USAGE_FALLBACK_CLASS).toBe('text-[11px] text-muted');

    const model = buildStoragePoolRowModel(baseRecord(), {
      deltaBytes: 40 * 1024 * 1024 * 1024,
      label: '+40.00 GB',
      title: 'Used capacity grew by 40.00 GB over 24h.',
      toneClass: 'text-amber-600 dark:text-amber-300',
    });

    expect(model.platformLabel).toBe('PBS');
    expect(model.platformToneClass).toContain('bg-indigo-100');
    expect(model.hostLabel).toBe('pbs01');
    expect(model.topologyLabel).toBe('Datastore');
    expect(model.compactProtection).toBe('Protection Reduced');
    expect(model.capacityDeltaLabel).toBe('+40.00 GB');
    expect(model.capacityDeltaToneClass).toContain('text-amber-600');
    expect(model.compactIssue).toBe('Capacity Pressure');
    expect(model.compactImpact).toBe('—');
    expect(model.freeBytes).toBe(200);
  });
});
