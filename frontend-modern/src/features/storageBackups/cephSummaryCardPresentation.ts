import type { CephSummaryStats } from '@/features/storageBackups/cephSummaryPresentation';
import { getCephHealthLabel, getCephHealthStyles } from '@/features/storageBackups/storageDomain';
import {
  getCephClusterCardTitle,
  getCephSummaryClusterCountLabel,
  getCephSummaryHeading,
  getCephSummaryTotalLabel,
  getCephSummaryUsageLabel,
} from '@/features/storageBackups/storagePagePresentation';
import type { CephCluster } from '@/types/api';

export type CephSummaryHeaderPresentation = {
  heading: string;
  clusterCountLabel: string;
  totalLabel: string;
  usageLabel: string;
};

export type CephSummaryClusterCardPresentation = {
  id: string;
  title: string;
  healthLabel: string;
  healthClass: string;
  healthMessage: string;
  usedBytes: number;
  freeBytes: number;
  totalBytes: number;
};

export const CEPH_SUMMARY_CARD_TOP_ROW_CLASS = 'flex flex-wrap items-center justify-between gap-3';
export const CEPH_SUMMARY_CARD_TOP_LEFT_CLASS = 'space-y-0.5';
export const CEPH_SUMMARY_CARD_HEADING_CLASS = 'text-xs font-semibold uppercase tracking-wide text-muted';
export const CEPH_SUMMARY_CARD_CLUSTER_COUNT_CLASS = 'text-sm text-muted';
export const CEPH_SUMMARY_CARD_TOP_RIGHT_CLASS = 'text-right';
export const CEPH_SUMMARY_CARD_TOTAL_CLASS = 'text-sm font-semibold text-base-content';
export const CEPH_SUMMARY_CARD_USAGE_CLASS = 'text-[11px] text-muted';
export const CEPH_SUMMARY_CARD_GRID_CLASS = 'mt-3 grid gap-3 sm:grid-cols-2';
export const CEPH_SUMMARY_CARD_CLASS = 'rounded-md border border-border bg-surface p-3';
export const CEPH_SUMMARY_CARD_HEADER_CLASS = 'flex items-start justify-between gap-2';
export const CEPH_SUMMARY_CARD_INFO_WRAP_CLASS = 'min-w-0';
export const CEPH_SUMMARY_CARD_TITLE_CLASS = 'text-sm font-semibold text-base-content truncate';
export const CEPH_SUMMARY_CARD_MESSAGE_CLASS = 'text-[11px] text-muted truncate max-w-[240px]';
export const CEPH_SUMMARY_CARD_HEALTH_BADGE_CLASS = 'px-1.5 py-0.5 rounded text-[10px] font-medium';
export const CEPH_SUMMARY_CARD_BAR_WRAP_CLASS = 'mt-2';

export const getCephSummaryHeaderPresentation = (
  summary: CephSummaryStats,
): CephSummaryHeaderPresentation => ({
  heading: getCephSummaryHeading(),
  clusterCountLabel: getCephSummaryClusterCountLabel(summary.clusters.length),
  totalLabel: getCephSummaryTotalLabel(summary.totalBytes),
  usageLabel: getCephSummaryUsageLabel(summary.usagePercent),
});

export const getCephSummaryClusterCardPresentation = (
  cluster: CephCluster,
): CephSummaryClusterCardPresentation => ({
  id: cluster.id,
  title: getCephClusterCardTitle(cluster.name),
  healthLabel: getCephHealthLabel(cluster.health),
  healthClass: getCephHealthStyles(cluster.health),
  healthMessage: cluster.healthMessage || '',
  usedBytes: cluster.usedBytes,
  freeBytes: cluster.availableBytes,
  totalBytes: cluster.totalBytes,
});

export const getCephSummaryClusterCards = (
  summary: CephSummaryStats,
): CephSummaryClusterCardPresentation[] =>
  summary.clusters.map(getCephSummaryClusterCardPresentation);
