import {
  compactDetailSections,
  type DetailRow,
  type DetailSection,
} from '@/components/shared/detailSectionModel';
import type { Alert } from '@/types/api';
import type { ResourceAvailabilityMeta } from '@/types/resource';
import { getAvailabilityProbePresentation } from '@/utils/availabilityProbePresentation';

export interface DrawerMetricAlert {
  severity: 'warning' | 'critical';
  threshold?: number;
}

// An active alert on a trend card's metric colours that card and draws the
// alert's own trigger. The most severe active alert wins.
export const getDrawerMetricAlert = (
  alerts: readonly Alert[] | undefined,
  metric: string,
): DrawerMetricAlert | undefined => {
  let result: DrawerMetricAlert | undefined;
  for (const alert of alerts ?? []) {
    if (alert.type.trim().toLowerCase() !== metric || alert.level === 'info') continue;
    const severity = alert.level === 'critical' ? 'critical' : 'warning';
    if (result?.severity === 'critical' && severity === 'warning') continue;
    result = {
      severity,
      threshold:
        typeof alert.threshold === 'number' && Number.isFinite(alert.threshold)
          ? alert.threshold
          : undefined,
    };
  }
  return result;
};

const isExceptionRow = (row: DetailRow): boolean => row.tone === 'warning' || row.tone === 'danger';

// The drawer demotes inventory to a collapsed Details section, but a fact that
// says something is wrong or untrustworthy must stay in front whichever section
// owns it. Detail rows already carry that judgement as a warning/danger tone.
export const collectDrawerExceptionRows = (sections: readonly DetailSection[]): DetailRow[] => {
  const seen = new Set<string>();
  const rows: DetailRow[] = [];
  for (const section of sections) {
    for (const row of section.rows) {
      if (!isExceptionRow(row)) continue;
      const key = `${row.label}\u0000${row.value}`;
      if (seen.has(key)) continue;
      seen.add(key);
      rows.push(row);
    }
  }
  return rows;
};

// Lifted rows move rather than repeat: each fact appears once in the drawer.
export const withoutDrawerExceptionRows = (sections: readonly DetailSection[]): DetailSection[] =>
  compactDetailSections(
    sections.map((section) => ({
      ...section,
      rows: section.rows.filter((row) => !isExceptionRow(row)),
    })),
  );

// "Passing" needs a healthy result that is still fresh. A success whose
// evidence has expired, or whose freshness is unknown, is not summarised away:
// its card stays in front, where it says Stale.
export const isDrawerAvailabilityHealthy = (
  check: ResourceAvailabilityMeta,
  now: number,
): boolean => {
  const up = check.aggregateState ? check.aggregateState === 'healthy' : check.available === true;
  if (!up) return false;
  const presentation = getAvailabilityProbePresentation(
    {
      type: 'network-endpoint',
      platformType: 'availability',
      status: 'online',
      availability: check,
    },
    new Date(now),
  );
  return presentation?.freshnessLabel === 'fresh';
};
