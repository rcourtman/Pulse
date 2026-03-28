import type { ProtectionRollup, RecoveryOutcome, RecoveryPoint } from '@/types/recovery';
import {
  getRecoveryItemTypeBadgeClass,
  getRecoveryItemTypeLabel,
  getRecoveryPointItemTypeKey,
} from '@/utils/recoveryItemTypePresentation';
import { getRecoveryLocationFacetLabel } from '@/utils/recoveryLocationPresentation';
import {
  getRecoveryOutcomeTextClass,
  normalizeRecoveryOutcome,
} from '@/utils/recoveryOutcomePresentation';
import type { RecoveryIssueTone } from '@/utils/recoveryIssuePresentation';

export const STALE_ISSUE_THRESHOLD_MS = 7 * 24 * 60 * 60 * 1000;
export const AGING_THRESHOLD_MS = 2 * 24 * 60 * 60 * 1000;

export const RECOVERY_GROUP_HEADER_ROW_CLASS = 'bg-surface-alt/55 hover:bg-surface-alt/55';
export const RECOVERY_GROUP_HEADER_TEXT_CLASS =
  'py-1 pr-3 pl-4 text-[11px] font-medium text-base-content';
export const RECOVERY_ADVANCED_FILTER_LABEL_CLASS = 'text-[11px] font-medium text-muted';
export const RECOVERY_ADVANCED_FILTER_FIELD_CLASS =
  'min-h-[2.25rem] w-full rounded-md border border-border bg-surface px-2.5 py-1.5 text-sm text-base-content outline-none focus:border-blue-500';
export const RECOVERY_GROUP_NO_TIMESTAMP_LABEL = 'No Timestamp';
export const RECOVERY_PROTECTED_SEARCH_PLACEHOLDER = 'Search protected items...';
export const RECOVERY_HISTORY_SEARCH_PLACEHOLDER = 'Search recovery history...';
export const RECOVERY_SEARCH_HISTORY_EMPTY_MESSAGE = 'Recent searches appear here.';
export const RECOVERY_ARTIFACT_METADATA_TEXT_CLASS =
  'text-[11px] font-medium text-base-content/80';
export const RECOVERY_ARTIFACT_COLUMN_LABELS: Record<string, string> = {
  cluster: getRecoveryLocationFacetLabel('cluster'),
  nodeAgent: getRecoveryLocationFacetLabel('node'),
  namespace: getRecoveryLocationFacetLabel('namespace'),
  type: 'Item Type',
  item: 'Item',
  platform: 'Platform',
};

const RECOVERY_ARTIFACT_COLUMN_SPECS: Record<string, { headerClass: string; minWidthPx: number }> = {
  time: { headerClass: 'w-[76px] text-right', minWidthPx: 76 },
  type: { headerClass: 'w-[96px] text-center', minWidthPx: 96 },
  item: { headerClass: 'w-[248px]', minWidthPx: 248 },
  platform: { headerClass: 'w-[78px] text-center', minWidthPx: 78 },
  entityId: { headerClass: 'w-[84px]', minWidthPx: 84 },
  cluster: { headerClass: 'w-[120px]', minWidthPx: 120 },
  nodeAgent: { headerClass: 'w-[120px]', minWidthPx: 120 },
  namespace: { headerClass: 'w-[120px]', minWidthPx: 120 },
  verified: { headerClass: 'w-[56px] text-center', minWidthPx: 56 },
  size: { headerClass: 'w-[92px] text-right', minWidthPx: 92 },
  method: { headerClass: 'w-[84px] text-center', minWidthPx: 84 },
  repository: { headerClass: 'w-[160px]', minWidthPx: 160 },
  details: { headerClass: 'w-[220px]', minWidthPx: 220 },
  outcome: { headerClass: 'w-[88px] text-center', minWidthPx: 88 },
};

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

export function getRecoveryGroupNoTimestampLabel(): string {
  return RECOVERY_GROUP_NO_TIMESTAMP_LABEL;
}

export function getRecoveryProtectedSearchPlaceholder(): string {
  return RECOVERY_PROTECTED_SEARCH_PLACEHOLDER;
}

export function getRecoveryHistorySearchPlaceholder(): string {
  return RECOVERY_HISTORY_SEARCH_PLACEHOLDER;
}

export function getRecoverySearchHistoryEmptyMessage(): string {
  return RECOVERY_SEARCH_HISTORY_EMPTY_MESSAGE;
}

export function getRecoveryArtifactColumnLabel(id: string, fallback?: string): string {
  return RECOVERY_ARTIFACT_COLUMN_LABELS[id] || fallback || id;
}

export function getRecoveryPointItemTypeBadgeClass(point: RecoveryPoint): string {
  return getRecoveryItemTypeBadgeClass(getRecoveryPointItemTypeKey(point));
}

export function getRecoveryPointItemTypeLabel(point: RecoveryPoint): string {
  return getRecoveryItemTypeLabel(getRecoveryPointItemTypeKey(point));
}

export function getRecoveryArtifactMetadataTextClass(
  tone: 'default' | 'muted' = 'default',
): string {
  return tone === 'muted'
    ? `${RECOVERY_ARTIFACT_METADATA_TEXT_CLASS} text-muted`
    : RECOVERY_ARTIFACT_METADATA_TEXT_CLASS;
}

export function getRecoveryArtifactOutcomeTextClass(outcome: RecoveryOutcome): string {
  return `text-[11px] font-medium ${getRecoveryOutcomeTextClass(outcome)}`;
}

export function getRecoverySubjectTypeBadgeClass(point: RecoveryPoint): string {
  return getRecoveryPointItemTypeBadgeClass(point);
}

export function getRecoverySubjectTypeLabel(point: RecoveryPoint): string {
  return getRecoveryPointItemTypeLabel(point);
}

export function getRecoveryArtifactColumnHeaderClass(id: string): string {
  return RECOVERY_ARTIFACT_COLUMN_SPECS[id]?.headerClass || '';
}

export function getRecoveryArtifactTableMinWidth(columnIds: readonly string[]): string {
  const totalWidth = columnIds.reduce(
    (sum, id) => sum + (RECOVERY_ARTIFACT_COLUMN_SPECS[id]?.minWidthPx || 140),
    0,
  );
  return `${Math.max(980, totalWidth)}px`;
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
