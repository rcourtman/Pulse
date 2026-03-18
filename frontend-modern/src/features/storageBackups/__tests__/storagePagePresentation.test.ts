import { describe, expect, it } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  getCephClusterCardTitle,
  getCephSummaryClusterCountLabel,
  getCephSummaryHeading,
  getCephSummaryTotalLabel,
  getCephSummaryUsageLabel,
  getStorageLoadingMessage,
  getStorageEmptyStateMessage,
  getStoragePageBannerActionLabel,
  getStoragePageBannerMessage,
  getStorageTableHeading,
  STORAGE_BANNER_ACTION_BUTTON_CLASS,
  STORAGE_CONTENT_CARD_BODY_CLASS,
  STORAGE_CONTENT_CARD_HEADER_CLASS,
  STORAGE_CONTROLS_NODE_DIVIDER_CLASS,
  STORAGE_CONTROLS_NODE_SELECT_CLASS,
  STORAGE_PAGE_BANNER_ROW_CLASS,
  STORAGE_PAGE_BANNER_TEXT_CLASS,
  STORAGE_POOLS_BODY_CLASS,
  STORAGE_POOLS_EMPTY_STATE_CLASS,
  STORAGE_POOLS_HEADER_ROW_CLASS,
  STORAGE_POOLS_LOADING_STATE_CLASS,
  STORAGE_POOLS_SCROLL_WRAP_CLASS,
  STORAGE_POOLS_TABLE_CLASS,
  STORAGE_POOL_TABLE_COLUMNS,
  STORAGE_VIEW_OPTIONS,
  shouldShowCephSummaryCard,
} from '@/features/storageBackups/storagePagePresentation';

const makeRecord = (): StorageRecord =>
  ({
    id: 'storage-1',
    name: 'ceph-store',
    category: 'pool',
    health: 'healthy',
    location: { label: 'cluster-main', scope: 'cluster' },
    capacity: { totalBytes: 100, usedBytes: 40, freeBytes: 60, usagePercent: 40 },
    capabilities: ['capacity'],
    source: {
      platform: 'proxmox-pve',
      family: 'virtualization',
      origin: 'resource',
      adapterId: 'resource-storage',
    },
    observedAt: Date.now(),
    refs: { platformEntityId: '' },
  }) as StorageRecord;

describe('storagePagePresentation', () => {
  it('formats ceph summary card labels canonically', () => {
    expect(getCephSummaryHeading()).toBe('Ceph Summary');
    expect(getCephSummaryClusterCountLabel(1)).toBe('1 cluster detected');
    expect(getCephSummaryClusterCountLabel(2)).toBe('2 clusters detected');
    expect(getCephSummaryTotalLabel(1024)).toContain('1.00 KB');
    expect(getCephSummaryUsageLabel(40)).toBe('40% used');
    expect(getCephClusterCardTitle('')).toBe('Ceph Cluster');
    expect(getCephClusterCardTitle('homelab')).toBe('homelab');
  });

  it('formats storage page banners and table copy canonically', () => {
    expect(getStoragePageBannerMessage('reconnecting')).toBe(
      'Reconnecting to backend data stream…',
    );
    expect(getStoragePageBannerActionLabel('reconnecting')).toBe('Retry now');
    expect(getStoragePageBannerActionLabel('fetch-error')).toBeNull();
    expect(STORAGE_BANNER_ACTION_BUTTON_CLASS).toContain('border-amber-300');
    expect(STORAGE_PAGE_BANNER_ROW_CLASS).toContain('justify-between');
    expect(STORAGE_PAGE_BANNER_TEXT_CLASS).toContain('text-amber-800');
    expect(STORAGE_CONTROLS_NODE_SELECT_CLASS).toContain('focus:ring-blue-500');
    expect(STORAGE_CONTROLS_NODE_DIVIDER_CLASS).toBe('h-5 w-px bg-surface-hover hidden sm:block');
    expect(STORAGE_CONTENT_CARD_HEADER_CLASS).toContain('uppercase');
    expect(STORAGE_CONTENT_CARD_BODY_CLASS).toBe('p-2');
    expect(STORAGE_POOLS_EMPTY_STATE_CLASS).toBe('p-6 text-sm text-muted');
    expect(STORAGE_POOLS_LOADING_STATE_CLASS).toBe('p-6 text-sm text-muted');
    expect(STORAGE_POOLS_SCROLL_WRAP_CLASS).toBe('overflow-x-auto');
    expect(STORAGE_POOLS_TABLE_CLASS).toBe('w-full text-xs');
    expect(STORAGE_POOLS_HEADER_ROW_CLASS).toContain('bg-surface-alt');
    expect(STORAGE_POOLS_BODY_CLASS).toBe('divide-y divide-border');
    expect(getStorageTableHeading('pools')).toBe('Storage Pools');
    expect(getStorageTableHeading('disks')).toBe('Physical Disks');
    expect(getStorageLoadingMessage()).toBe('Loading storage resources...');
    expect(getStorageEmptyStateMessage()).toBe('No storage records match the current filters.');
  });

  it('exports canonical storage view and table column contracts', () => {
    expect(STORAGE_VIEW_OPTIONS).toEqual([
      { value: 'pools', label: 'Pools' },
      { value: 'disks', label: 'Physical Disks' },
    ]);
    expect(STORAGE_POOL_TABLE_COLUMNS.map((column) => column.label)).toEqual([
      'Storage',
      'Source',
      'Type',
      'Host',
      'Protection',
      'Usage',
      'Impact',
      'Primary Issue',
      '',
    ]);
  });

  it('shows the ceph summary card only for pool views with visible ceph records', () => {
    const summary = {
      clusters: [{ id: 'cluster-1' }] as any[],
      totalBytes: 100,
      usedBytes: 40,
      availableBytes: 60,
      usagePercent: 40,
    };
    const record = makeRecord();
    expect(shouldShowCephSummaryCard('pools', summary, [record], () => true)).toBe(true);
    expect(shouldShowCephSummaryCard('disks', summary, [record], () => true)).toBe(false);
    expect(shouldShowCephSummaryCard('pools', { ...summary, clusters: [] }, [record], () => true)).toBe(
      false,
    );
  });
});
