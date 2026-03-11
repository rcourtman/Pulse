import { describe, expect, it } from 'vitest';
import {
  CEPH_SUMMARY_CARD_BAR_WRAP_CLASS,
  CEPH_SUMMARY_CARD_CLASS,
  CEPH_SUMMARY_CARD_CLUSTER_COUNT_CLASS,
  CEPH_SUMMARY_CARD_GRID_CLASS,
  CEPH_SUMMARY_CARD_HEADER_CLASS,
  CEPH_SUMMARY_CARD_HEALTH_BADGE_CLASS,
  CEPH_SUMMARY_CARD_HEADING_CLASS,
  CEPH_SUMMARY_CARD_INFO_WRAP_CLASS,
  CEPH_SUMMARY_CARD_MESSAGE_CLASS,
  CEPH_SUMMARY_CARD_TOP_LEFT_CLASS,
  CEPH_SUMMARY_CARD_TOP_RIGHT_CLASS,
  CEPH_SUMMARY_CARD_TOP_ROW_CLASS,
  CEPH_SUMMARY_CARD_TOTAL_CLASS,
  CEPH_SUMMARY_CARD_TITLE_CLASS,
  CEPH_SUMMARY_CARD_USAGE_CLASS,
  getCephSummaryClusterCards,
  getCephSummaryHeaderPresentation,
} from '@/features/storageBackups/cephSummaryCardPresentation';
import type { CephSummaryStats } from '@/features/storageBackups/cephSummaryPresentation';

const summary: CephSummaryStats = {
  clusters: [
    {
      id: 'cluster-1',
      instance: 'cluster-main',
      name: 'cluster-main Ceph',
      health: 'HEALTH_OK',
      healthMessage: 'Healthy',
      totalBytes: 300,
      usedBytes: 120,
      availableBytes: 180,
      usagePercent: 40,
      numMons: 3,
      numMgrs: 2,
      numOsds: 6,
      numOsdsUp: 6,
      numOsdsIn: 6,
      numPGs: 128,
      pools: undefined,
      services: undefined,
      lastUpdated: Date.now(),
    },
  ],
  totalBytes: 300,
  usedBytes: 120,
  availableBytes: 180,
  usagePercent: 40,
};

describe('cephSummaryCardPresentation', () => {
  it('builds canonical ceph summary header and cards', () => {
    expect(CEPH_SUMMARY_CARD_TOP_ROW_CLASS).toContain('justify-between');
    expect(CEPH_SUMMARY_CARD_TOP_LEFT_CLASS).toContain('space-y-0.5');
    expect(CEPH_SUMMARY_CARD_HEADING_CLASS).toContain('uppercase');
    expect(CEPH_SUMMARY_CARD_CLUSTER_COUNT_CLASS).toBe('text-sm text-muted');
    expect(CEPH_SUMMARY_CARD_TOP_RIGHT_CLASS).toBe('text-right');
    expect(CEPH_SUMMARY_CARD_TOTAL_CLASS).toContain('font-semibold');
    expect(CEPH_SUMMARY_CARD_USAGE_CLASS).toBe('text-[11px] text-muted');
    expect(CEPH_SUMMARY_CARD_GRID_CLASS).toContain('sm:grid-cols-2');
    expect(CEPH_SUMMARY_CARD_CLASS).toContain('border-border');
    expect(CEPH_SUMMARY_CARD_HEADER_CLASS).toContain('justify-between');
    expect(CEPH_SUMMARY_CARD_INFO_WRAP_CLASS).toBe('min-w-0');
    expect(CEPH_SUMMARY_CARD_TITLE_CLASS).toContain('truncate');
    expect(CEPH_SUMMARY_CARD_MESSAGE_CLASS).toContain('max-w-[240px]');
    expect(CEPH_SUMMARY_CARD_HEALTH_BADGE_CLASS).toContain('text-[10px]');
    expect(CEPH_SUMMARY_CARD_BAR_WRAP_CLASS).toBe('mt-2');

    expect(getCephSummaryHeaderPresentation(summary)).toEqual({
      heading: 'Ceph Summary',
      clusterCountLabel: '1 cluster detected',
      totalLabel: '300 B',
      usageLabel: '40% used',
    });

    expect(getCephSummaryClusterCards(summary)).toEqual([
      expect.objectContaining({
        id: 'cluster-1',
        title: 'cluster-main Ceph',
        healthLabel: 'OK',
        usedBytes: 120,
        freeBytes: 180,
        totalBytes: 300,
      }),
    ]);
  });
});
