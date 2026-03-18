import { describe, expect, it } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  buildStoragePoolRowModel,
  getStoragePoolExpandIconClass,
  getStoragePoolImpactTextClass,
  STORAGE_POOL_ROW_CLASS,
  STORAGE_POOL_ROW_EXPAND_BUTTON_CLASS,
  STORAGE_POOL_ROW_EXPAND_ICON_BASE_CLASS,
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
    expect(STORAGE_POOL_ROW_NAME_TEXT_CLASS).toContain('font-semibold');
    expect(STORAGE_POOL_ROW_SOURCE_BADGE_CLASS).toContain('text-[9px]');
    expect(STORAGE_POOL_ROW_EXPAND_BUTTON_CLASS).toContain('hover:bg-surface-hover');
    expect(STORAGE_POOL_ROW_PLACEHOLDER_CLASS).toBe('text-muted');
    expect(STORAGE_POOL_ROW_USAGE_FALLBACK_CLASS).toBe('text-[11px] text-muted');
    expect(STORAGE_POOL_ROW_EXPAND_ICON_BASE_CLASS).toContain('transition-transform');

    const model = buildStoragePoolRowModel(baseRecord());

    expect(model.platformLabel).toBe('PBS');
    expect(model.platformToneClass).toContain('bg-indigo-100');
    expect(model.hostLabel).toBe('pbs01');
    expect(model.topologyLabel).toBe('Datastore');
    expect(model.compactProtection).toBe('Protection Reduced');
    expect(model.compactIssue).toBe('Capacity Pressure');
    expect(model.compactImpact).toBe('—');
    expect(model.freeBytes).toBe(200);
    expect(getStoragePoolExpandIconClass(true)).toContain('rotate-90');
    expect(getStoragePoolImpactTextClass('—')).toContain('text-muted');
  });
});
