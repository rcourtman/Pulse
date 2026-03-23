import type { DockerPlatformData } from './resourceDetailMappers';
import { formatInteger } from './resourceDetailMappers';

export type ResourceDetailValueBreakdownEntry = {
  label: string;
  value: number;
  warn?: boolean;
};

type PbsPlatformDataLike = {
  datastoreCount?: number;
  backupJobCount?: number;
  syncJobCount?: number;
  verifyJobCount?: number;
  pruneJobCount?: number;
  garbageJobCount?: number;
};

type PmgPlatformDataLike = {
  queueTotal?: number;
  queueActive?: number;
  queueDeferred?: number;
  queueHold?: number;
  queueIncoming?: number;
  mailCountTotal?: number;
  spamIn?: number;
  virusIn?: number;
};

const filterVisibleBreakdown = <T extends ResourceDetailValueBreakdownEntry>(
  entries: T[],
): T[] => {
  const nonZero = entries.filter((entry) => entry.value > 0);
  return nonZero.length > 0 ? nonZero : entries;
};

export const getPbsJobTotal = (pbs: PbsPlatformDataLike | undefined): number => {
  if (!pbs) return 0;
  return (
    (pbs.backupJobCount || 0) +
    (pbs.syncJobCount || 0) +
    (pbs.verifyJobCount || 0) +
    (pbs.pruneJobCount || 0) +
    (pbs.garbageJobCount || 0)
  );
};

export const buildPbsVisibleJobBreakdown = (
  pbs: PbsPlatformDataLike | undefined,
): ResourceDetailValueBreakdownEntry[] => {
  if (!pbs) return [];

  return filterVisibleBreakdown([
    { label: 'Backup', value: pbs.backupJobCount || 0 },
    { label: 'Sync', value: pbs.syncJobCount || 0 },
    { label: 'Verify', value: pbs.verifyJobCount || 0 },
    { label: 'Prune', value: pbs.pruneJobCount || 0 },
    { label: 'Garbage', value: pbs.garbageJobCount || 0 },
  ]);
};

export const getPmgQueueBacklog = (pmg: PmgPlatformDataLike | undefined): number =>
  !pmg ? 0 : (pmg.queueDeferred || 0) + (pmg.queueHold || 0);

export const buildPmgVisibleQueueBreakdown = (
  pmg: PmgPlatformDataLike | undefined,
): ResourceDetailValueBreakdownEntry[] => {
  if (!pmg) return [];

  return filterVisibleBreakdown([
    { label: 'Active', value: pmg.queueActive || 0 },
    { label: 'Deferred', value: pmg.queueDeferred || 0, warn: (pmg.queueDeferred || 0) > 0 },
    { label: 'Hold', value: pmg.queueHold || 0, warn: (pmg.queueHold || 0) > 0 },
    { label: 'Incoming', value: pmg.queueIncoming || 0 },
  ]);
};

export const buildPmgVisibleMailBreakdown = (
  pmg: PmgPlatformDataLike | undefined,
): ResourceDetailValueBreakdownEntry[] => {
  if (!pmg) return [];

  return filterVisibleBreakdown([
    { label: 'Mail', value: pmg.mailCountTotal || 0 },
    { label: 'Spam', value: pmg.spamIn || 0 },
    { label: 'Virus', value: pmg.virusIn || 0 },
  ]);
};

export const getServiceDetailsSummary = (args: {
  resourceType: string;
  docker: DockerPlatformData | undefined;
  pbs: PbsPlatformDataLike | undefined;
  pmg: PmgPlatformDataLike | undefined;
}): string | null => {
  const { resourceType, docker, pbs, pmg } = args;

  if (resourceType === 'docker-host') {
    return `${formatInteger(docker?.containerCount ?? 0)} containers · ${formatInteger(
      docker?.updatesAvailableCount ?? 0,
    )} updates`;
  }

  if (pbs) {
    return `${formatInteger(pbs.datastoreCount || 0)} datastores · ${formatInteger(
      getPbsJobTotal(pbs),
    )} jobs`;
  }

  if (pmg) {
    return `${formatInteger(pmg.queueTotal || 0)} queue total · ${formatInteger(
      getPmgQueueBacklog(pmg),
    )} backlog`;
  }

  return null;
};
