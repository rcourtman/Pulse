import type {
  DockerPlatformData,
  PBSPlatformData,
  PMGPlatformData,
} from './resourceDetailMappers';
import { formatInteger } from './resourceDetailMappers';
import type {
  PBSBackupJob,
  PBSGarbageJob,
  PBSPruneJob,
  PBSSyncJob,
  PBSVerifyJob,
} from '@/types/api';

export type ResourceDetailValueBreakdownEntry = {
  label: string;
  value: number;
  warn?: boolean;
};

const filterVisibleBreakdown = <T extends ResourceDetailValueBreakdownEntry>(
  entries: T[],
): T[] => {
  const nonZero = entries.filter((entry) => entry.value > 0);
  return nonZero.length > 0 ? nonZero : entries;
};

const formatCount = (value: number, singular: string, plural = `${singular}s`): string =>
  `${formatInteger(value)} ${value === 1 ? singular : plural}`;

const normalizeDelimitedLabel = (value: string): string =>
  value
    .trim()
    .split(/[\s_-]+/)
    .filter((part) => part.length > 0)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

const normalizePbsTaskStatus = (status?: string): string =>
  (status || '')
    .trim()
    .toLowerCase()
    .replace(/[_-]+/g, ' ')
    .replace(/\s+/g, ' ');

const PBS_ACTIVE_STATUS_TOKENS = [
  'running',
  'active',
  'queued',
  'pending',
  'starting',
  'started',
  'in progress',
] as const;

const PBS_INACTIVE_STATUS_TOKENS = [
  'ok',
  'idle',
  'stopped',
  'disabled',
  'error',
  'failed',
  'warning',
  'unknown',
  'success',
  'successful',
  'complete',
  'completed',
  'scheduled',
] as const;

const hasStatusToken = (status: string, token: string): boolean =>
  new RegExp(`(?:^|\\b)${token.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}(?:$|\\b)`, 'i').test(
    status,
  );

const isPbsTaskStatusActive = (status?: string): boolean => {
  const normalized = normalizePbsTaskStatus(status);
  if (!normalized) return false;
  if (PBS_INACTIVE_STATUS_TOKENS.some((token) => hasStatusToken(normalized, token))) {
    return false;
  }
  return PBS_ACTIVE_STATUS_TOKENS.some((token) => hasStatusToken(normalized, token));
};

const formatPbsTaskStatusLabel = (status?: string): string => {
  const normalized = normalizePbsTaskStatus(status);
  if (!normalized) return 'Active';
  return normalizeDelimitedLabel(normalized);
};

const joinPbsTaskContext = (...parts: Array<string | null | undefined>): string | null => {
  const filtered = parts
    .map((part) => part?.trim())
    .filter((part): part is string => Boolean(part));
  return filtered.length > 0 ? filtered.join(' · ') : null;
};

const formatPbsBackupWorkload = (type?: string, vmid?: string): string | null => {
  const normalizedType = (type || '').trim().toLowerCase();
  if (!normalizedType && !(vmid || '').trim()) return null;
  const typeLabel =
    normalizedType === 'vm'
      ? 'VM'
      : normalizedType === 'ct'
        ? 'Container'
        : normalizedType
          ? normalizeDelimitedLabel(normalizedType)
          : 'Workload';
  const workloadId = (vmid || '').trim();
  return workloadId ? `${typeLabel} ${workloadId}` : typeLabel;
};

const buildPbsTaskLabel = (taskType: string, id?: string): string => {
  const normalizedId = (id || '').trim();
  return normalizedId ? `${taskType} ${normalizedId}` : taskType;
};

export type PbsActiveTaskEntry = {
  id: string;
  label: string;
  context: string | null;
  statusLabel: string;
  error?: string;
};

export type PbsActivitySummary = {
  label: string | null;
  detail: string | null;
  activeTaskCount: number;
};

const buildPbsActiveTaskEntry = (
  taskType: string,
  job:
    | PBSBackupJob
    | PBSSyncJob
    | PBSVerifyJob
    | PBSPruneJob
    | PBSGarbageJob,
  context: string | null,
): PbsActiveTaskEntry | null => {
  if (!isPbsTaskStatusActive(job.status)) {
    return null;
  }
  return {
    id: job.id,
    label: buildPbsTaskLabel(taskType, job.id),
    context,
    statusLabel: formatPbsTaskStatusLabel(job.status),
    error: job.error?.trim() || undefined,
  };
};

export const getPbsJobTotal = (pbs: PBSPlatformData | undefined): number => {
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
  pbs: PBSPlatformData | undefined,
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

export const buildPbsActiveTasks = (pbs: PBSPlatformData | undefined): PbsActiveTaskEntry[] => {
  if (!pbs) return [];

  const tasks: PbsActiveTaskEntry[] = [];

  for (const job of pbs.backupJobs ?? []) {
    const entry = buildPbsActiveTaskEntry(
      'Backup',
      job,
      joinPbsTaskContext(job.store, formatPbsBackupWorkload(job.type, job.vmid)),
    );
    if (entry) tasks.push(entry);
  }

  for (const job of pbs.syncJobs ?? []) {
    const entry = buildPbsActiveTaskEntry(
      'Sync',
      job,
      joinPbsTaskContext(job.store, job.remote ? `Remote ${job.remote}` : null),
    );
    if (entry) tasks.push(entry);
  }

  for (const job of pbs.verifyJobs ?? []) {
    const entry = buildPbsActiveTaskEntry('Verify', job, joinPbsTaskContext(job.store));
    if (entry) tasks.push(entry);
  }

  for (const job of pbs.pruneJobs ?? []) {
    const entry = buildPbsActiveTaskEntry('Prune', job, joinPbsTaskContext(job.store));
    if (entry) tasks.push(entry);
  }

  for (const job of pbs.garbageJobs ?? []) {
    const entry = buildPbsActiveTaskEntry(
      'Garbage Collection',
      job,
      joinPbsTaskContext(job.store),
    );
    if (entry) tasks.push(entry);
  }

  return tasks;
};

export const getPbsActivitySummary = (
  pbs: PBSPlatformData | undefined,
): PbsActivitySummary => {
  const totalJobs = getPbsJobTotal(pbs);
  const activeTaskCount = buildPbsActiveTasks(pbs).length;

  if (activeTaskCount > 0) {
    return {
      label: `${formatInteger(activeTaskCount)} active`,
      detail:
        totalJobs > activeTaskCount ? `${formatInteger(totalJobs)} total` : null,
      activeTaskCount,
    };
  }

  return {
    label: totalJobs > 0 ? `${formatInteger(totalJobs)} jobs` : null,
    detail: null,
    activeTaskCount: 0,
  };
};

export const getPmgQueueBacklog = (pmg: PMGPlatformData | undefined): number =>
  !pmg ? 0 : (pmg.queueDeferred || 0) + (pmg.queueHold || 0);

export const buildPmgVisibleQueueBreakdown = (
  pmg: PMGPlatformData | undefined,
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
  pmg: PMGPlatformData | undefined,
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
  pbs: PBSPlatformData | undefined;
  pmg: PMGPlatformData | undefined;
}): string | null => {
  const { resourceType, docker, pbs, pmg } = args;

  if (resourceType === 'docker-host') {
    return `${formatCount(docker?.containerCount ?? 0, 'container')} · ${formatCount(
      docker?.updatesAvailableCount ?? 0,
      'update',
    )}`;
  }

  if (pbs) {
    const activeTaskCount = buildPbsActiveTasks(pbs).length;
    return `${formatCount(pbs.datastoreCount || 0, 'datastore')} · ${formatCount(
      activeTaskCount > 0 ? activeTaskCount : getPbsJobTotal(pbs),
      activeTaskCount > 0 ? 'active task' : 'job',
    )}`;
  }

  if (pmg) {
    return `${formatCount(pmg.queueTotal || 0, 'queued message')} · ${formatCount(
      getPmgQueueBacklog(pmg),
      'delayed message',
    )}`;
  }

  return null;
};
