import type { ProtectionRollup, RecoveryPoint } from '@/types/recovery';
import { getResourceTypePresentation } from '@/utils/resourceTypePresentation';
import {
  getWorkloadTypePresentation,
  normalizeWorkloadTypePresentationKey,
} from '@/utils/workloadTypePresentation';
import {
  normalizeRecoveryOutcome,
  type RecoveryOutcome,
} from '@/utils/recoveryOutcomePresentation';
import type { RecoveryIssueTone } from '@/utils/recoveryIssuePresentation';

export const STALE_ISSUE_THRESHOLD_MS = 7 * 24 * 60 * 60 * 1000;
export const AGING_THRESHOLD_MS = 2 * 24 * 60 * 60 * 1000;

export const RECOVERY_GROUP_HEADER_ROW_CLASS = 'bg-surface-alt hover:bg-surface-alt';
export const RECOVERY_GROUP_HEADER_TEXT_CLASS =
  'py-1.5 pr-3 pl-4 text-[12px] sm:text-sm font-semibold text-base-content';
export const RECOVERY_ADVANCED_FILTER_LABEL_CLASS = 'text-[11px] font-medium text-muted';
export const RECOVERY_ADVANCED_FILTER_FIELD_CLASS =
  'min-h-[2.25rem] w-full rounded-md border border-border bg-surface px-2.5 py-1.5 text-sm text-base-content outline-none focus:border-blue-500';

const titleize = (value: string): string =>
  (value || '')
    .split('-')
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

export function getRecoveryEventTimeTextClass(
  timestamp: number,
  nowMs: number = Date.now(),
): string {
  if (!timestamp) return 'text-muted';
  const ageMs = nowMs - timestamp;
  if (ageMs <= 24 * 60 * 60 * 1000) return 'text-emerald-600 dark:text-emerald-400 font-medium';
  if (ageMs <= 7 * 24 * 60 * 60 * 1000) return 'text-amber-600 dark:text-amber-400 font-medium';
  if (ageMs <= 30 * 24 * 60 * 60 * 1000) return 'text-orange-600 dark:text-orange-400';
  return 'text-muted';
}

const normalizeRecoverySubjectTypeKey = (value: string): string =>
  value
    .replace(/^k8s-/, '')
    .replace(/^proxmox-/, '')
    .replace(/^truenas-/, '');

const normalizeRecoverySubjectWorkloadType = (
  value: string,
): Parameters<typeof getWorkloadTypePresentation>[0] => {
  const normalized = normalizeRecoverySubjectTypeKey(value.trim().toLowerCase());
  switch (normalized) {
    case 'lxc':
    case 'ct':
    case 'container':
      return 'system-container';
    case 'vm-backup':
      return 'vm';
    default:
      return normalized;
  }
};

const getRecoverySubjectTypePresentation = (point: RecoveryPoint) => {
  const raw = String(point.display?.subjectType || point.subjectRef?.type || '')
    .trim()
    .toLowerCase();
  if (!raw) return null;

  const workloadType = normalizeRecoverySubjectWorkloadType(raw);
  if (normalizeWorkloadTypePresentationKey(workloadType)) {
    const presentation = getWorkloadTypePresentation(workloadType);
    return {
      label: presentation.label,
      badgeClasses: presentation.className,
    };
  }

  const resourcePresentation = getResourceTypePresentation(normalizeRecoverySubjectTypeKey(raw));
  if (resourcePresentation) return resourcePresentation;
  return null;
};

export function getRecoverySubjectTypeBadgeClass(point: RecoveryPoint): string {
  const presentation = getRecoverySubjectTypePresentation(point);
  if (presentation) return presentation.badgeClasses;
  return 'bg-surface-alt text-base-content';
}

export function getRecoverySubjectTypeLabel(point: RecoveryPoint): string {
  const raw = String(point.display?.subjectType || point.subjectRef?.type || '')
    .trim()
    .toLowerCase();
  if (!raw) return '';
  const presentation = getRecoverySubjectTypePresentation(point);
  if (presentation) {
    if (
      presentation.label === normalizeRecoverySubjectTypeKey(raw) &&
      presentation.badgeClasses === 'bg-surface-alt text-base-content'
    ) {
      return titleize(normalizeRecoverySubjectTypeKey(raw));
    }
    return presentation.label;
  }
  return titleize(normalizeRecoverySubjectTypeKey(raw));
}

export function getRecoveryArtifactColumnHeaderClass(id: string): string {
  switch (id) {
    case 'time':
      return 'w-[76px] text-right';
    case 'type':
      return 'w-[72px] text-center';
    case 'subject':
      return 'w-[248px]';
    case 'entityId':
      return 'w-[84px]';
    case 'cluster':
      return 'w-[120px]';
    case 'nodeAgent':
      return 'w-[120px]';
    case 'namespace':
      return 'w-[120px]';
    case 'source':
      return 'w-[78px] text-center';
    case 'verified':
      return 'w-[56px] text-center';
    case 'size':
      return 'w-[92px] text-right';
    case 'method':
      return 'w-[84px] text-center';
    case 'repository':
      return 'w-[160px]';
    case 'details':
      return 'w-[220px]';
    case 'outcome':
      return 'w-[88px] text-center';
    default:
      return '';
  }
}

export function getRecoveryArtifactRowClass(selected: boolean): string {
  return selected
    ? 'bg-blue-50/40 dark:bg-blue-900/20 outline outline-1 -outline-offset-1 outline-blue-200/80 dark:outline-blue-800/80'
    : 'hover:bg-surface-hover';
}

export function getRecoveryRollupTimestampMs(rollup: ProtectionRollup): number {
  const raw = rollup.lastSuccessAt || rollup.lastAttemptAt || '';
  const ms = raw ? Date.parse(raw) : 0;
  return Number.isFinite(ms) ? ms : 0;
}

export function isRecoveryRollupStale(rollup: ProtectionRollup, nowMs: number): boolean {
  const successMs = rollup.lastSuccessAt ? Date.parse(rollup.lastSuccessAt) : 0;
  if (Number.isFinite(successMs) && successMs > 0)
    return nowMs - successMs >= STALE_ISSUE_THRESHOLD_MS;
  const attemptMs = rollup.lastAttemptAt ? Date.parse(rollup.lastAttemptAt) : 0;
  if (Number.isFinite(attemptMs) && attemptMs > 0)
    return nowMs - attemptMs >= STALE_ISSUE_THRESHOLD_MS;
  return false;
}

export function getRecoveryRollupIssueTone(
  rollup: ProtectionRollup,
  nowMs: number,
): RecoveryIssueTone {
  const outcome: RecoveryOutcome = normalizeRecoveryOutcome(rollup.lastOutcome);
  if (outcome === 'failed') return 'rose';
  if (outcome === 'running') return 'blue';
  if (outcome === 'warning' || isRecoveryRollupStale(rollup, nowMs)) return 'amber';
  return 'none';
}

export function getRecoveryRollupAgeTextClass(
  rollup: ProtectionRollup,
  nowMs: number,
): string {
  const ts = getRecoveryRollupTimestampMs(rollup);
  if (!ts || ts <= 0) return 'text-muted';
  const ageMs = nowMs - ts;
  if (ageMs >= STALE_ISSUE_THRESHOLD_MS) return 'text-rose-700 dark:text-rose-300';
  if (ageMs >= AGING_THRESHOLD_MS) return 'text-amber-700 dark:text-amber-300';
  return 'text-muted';
}
