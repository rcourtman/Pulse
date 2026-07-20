import type { Resource } from '@/types/resource';
import { getServiceHealthSummaryPresentation } from '@/utils/serviceHealthPresentation';
import type { IODistributionStats } from '@/components/Infrastructure/infrastructureSelectors';
import type { PBSPlatformData, PMGPlatformData } from './resourceDetailMappers';
import { getPbsActivitySummary, getPbsJobTotal } from './resourceDetailDrawerServiceModel';

type PBSServiceData = PBSPlatformData;
type PMGServiceData = PMGPlatformData;

export type PBSTableRow = {
  datastores: number | null;
  jobs: number | null;
  activity: string | null;
  activityDetail: string | null;
  activeTaskCount: number;
  health: string | null;
  tone: ReturnType<typeof getServiceHealthSummaryPresentation>['tone'];
};

export type PMGTableRow = {
  queue: number | null;
  deferred: number | null;
  hold: number | null;
  nodes: number | null;
  health: string | null;
  tone: ReturnType<typeof getServiceHealthSummaryPresentation>['tone'];
};

export interface IOEmphasis {
  className: string;
  showOutlierHint: boolean;
}

export const isResourceOnline = (resource: Resource) => {
  const status = resource.status?.toLowerCase();
  return status !== 'offline' && status !== 'stopped';
};

export const getPBSTableRow = (resource: Resource): PBSTableRow | null => {
  if (resource.type !== 'pbs') return null;
  const platformData = resource.platformData as
    { pbs?: PBSServiceData; pmg?: PMGServiceData } | undefined;
  const pbs = platformData?.pbs;
  const totalJobs = getPbsJobTotal(pbs);
  const activitySummary = getPbsActivitySummary(pbs);
  const health = pbs?.connectionHealth?.trim() || null;

  return {
    datastores: (pbs?.datastoreCount || 0) > 0 ? pbs?.datastoreCount || 0 : null,
    jobs: totalJobs > 0 ? totalJobs : null,
    activity: activitySummary.label,
    activityDetail: activitySummary.detail,
    activeTaskCount: activitySummary.activeTaskCount,
    health,
    tone: getServiceHealthSummaryPresentation(resource.status, health).tone,
  };
};

export const getPMGTableRow = (resource: Resource): PMGTableRow | null => {
  if (resource.type !== 'pmg') return null;
  const platformData = resource.platformData as
    { pbs?: PBSServiceData; pmg?: PMGServiceData } | undefined;
  const pmg = platformData?.pmg;
  const health = pmg?.connectionHealth?.trim() || null;
  const backlog = (pmg?.queueDeferred || 0) + (pmg?.queueHold || 0);

  return {
    queue: (pmg?.queueTotal || 0) > 0 ? pmg?.queueTotal || 0 : null,
    deferred: (pmg?.queueDeferred || 0) > 0 ? pmg?.queueDeferred || 0 : null,
    hold: (pmg?.queueHold || 0) > 0 ? pmg?.queueHold || 0 : null,
    nodes: (pmg?.nodeCount || 0) > 0 ? pmg?.nodeCount || 0 : null,
    health,
    tone:
      backlog > 0 ? 'warning' : getServiceHealthSummaryPresentation(resource.status, health).tone,
  };
};

export const getOutlierEmphasis = (value: number, stats: IODistributionStats): IOEmphasis => {
  if (!Number.isFinite(value) || value <= 0 || stats.max <= 0) {
    return { className: 'text-muted', showOutlierHint: false };
  }

  if (stats.count < 4) {
    const ratio = value / stats.max;
    if (ratio >= 0.995) {
      return { className: 'text-base-content font-medium', showOutlierHint: true };
    }
    return { className: 'text-muted', showOutlierHint: false };
  }

  if (stats.mad > 0) {
    const modifiedZ = (0.6745 * (value - stats.median)) / stats.mad;
    if (modifiedZ >= 6.5 && value >= stats.p99) {
      return { className: 'text-base-content font-semibold', showOutlierHint: true };
    }
    if (modifiedZ >= 5.5 && value >= stats.p97) {
      return { className: 'text-base-content font-medium', showOutlierHint: true };
    }
    return { className: 'text-muted', showOutlierHint: false };
  }

  if (value >= stats.p99)
    return { className: 'text-base-content font-semibold', showOutlierHint: true };
  if (value >= stats.p97)
    return { className: 'text-base-content font-medium', showOutlierHint: true };
  if (value > 0) return { className: 'text-muted', showOutlierHint: false };
  return { className: 'text-muted', showOutlierHint: false };
};
