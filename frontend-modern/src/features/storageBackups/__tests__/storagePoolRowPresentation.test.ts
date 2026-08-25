import { describe, expect, it, vi } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  buildStoragePoolRowModel,
  STORAGE_POOL_ROW_GROWTH_TEXT_CLASS,
  STORAGE_POOL_ROW_CLASS,
  STORAGE_POOL_ROW_EXPANDED_CLASS,
  STORAGE_POOL_ROW_HEIGHT_CLASS,
  STORAGE_POOL_ROW_NAME_TEXT_CLASS,
  STORAGE_POOL_ROW_PLACEHOLDER_CLASS,
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
    expect(STORAGE_POOL_ROW_HEIGHT_CLASS).toBe('h-[32px]');
    expect(STORAGE_POOL_ROW_NAME_TEXT_CLASS).toContain('font-semibold');
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

    expect(model.hostLabel).toBe('pbs01');
    expect(model.topologyLabel).toBe('Datastore');
    expect(model.stateLabel).toBe('Degraded');
    expect(model.stateToneClass).toContain('text-amber');
    expect(model.compactProtection).toBe('Protection Reduced');
    expect(model.capacityDeltaLabel).toBe('+40.00 GB');
    expect(model.capacityDeltaToneClass).toContain('text-amber-600');
    expect(model.freeBytes).toBe(200);
  });

  it('marks retained values stale and identifies the last successful refresh', () => {
    const now = new Date('2026-08-25T18:00:00Z');
    vi.useFakeTimers();
    vi.setSystemTime(now);

    const model = buildStoragePoolRowModel(
      baseRecord({
        statusLabel: 'Online',
        freshness: 'stale',
        observedAt: now.getTime() - 5 * 60 * 1000,
        freshnessError: 'Datastore.Audit permission denied',
      }),
    );

    expect(model.stateLabel).toBe('Stale');
    expect(model.stateToneClass).toContain('text-amber');
    expect(model.stateTitle).toBe(
      'Retained last-known storage values. Last successful refresh 5 mins ago. Datastore.Audit permission denied',
    );

    vi.useRealTimers();
  });
});
