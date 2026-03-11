import type { ProtectionRollup, RecoveryPoint } from '@/types/recovery';
import type { Resource } from '@/types/resource';

export type RecoveryArtifactMode = 'snapshot' | 'local' | 'remote';

export function getRecoveryRollupSubjectLabel(
  rollup: ProtectionRollup,
  resourcesById: Map<string, Resource>,
): string {
  const subjectResourceId = (rollup.subjectResourceId || '').trim();
  if (subjectResourceId) {
    const resource = resourcesById.get(subjectResourceId);
    const name = (resource?.name || '').trim();
    if (name) return name;
  }

  const ref = rollup.subjectRef || null;
  if (ref?.namespace && ref?.name) return `${ref.namespace}/${ref.name}`;
  if (ref?.name) return ref.name;
  if (subjectResourceId) return subjectResourceId;
  return rollup.rollupId;
}

export function getRecoveryPointTimestampMs(point: RecoveryPoint): number {
  const raw = String(point.completedAt || point.startedAt || '');
  const parsed = Date.parse(raw);
  return Number.isFinite(parsed) ? parsed : 0;
}

export function getRecoveryPointSubjectLabel(
  point: RecoveryPoint,
  resourcesById: Map<string, Resource>,
): string {
  const subjectResourceId = (point.subjectResourceId || '').trim();
  if (subjectResourceId) {
    const resource = resourcesById.get(subjectResourceId);
    const name = (resource?.name || '').trim();
    if (name) return name;
    return subjectResourceId;
  }

  const displayLabel = String(point.display?.subjectLabel || '').trim();
  if (displayLabel) return displayLabel;

  const ref = point.subjectRef || null;
  const namespace = String(ref?.namespace || '').trim();
  const name = String(ref?.name || '').trim();
  if (namespace && name) return `${namespace}/${name}`;
  if (name) return name;
  const id = String(ref?.id || '').trim();
  if (id) return id;
  return point.id;
}

export function getRecoveryPointRepositoryLabel(point: RecoveryPoint): string {
  const displayLabel = String(point.display?.repositoryLabel || '').trim();
  if (displayLabel) return displayLabel;

  const repository = point.repositoryRef || null;
  const repositoryName = String(repository?.name || '').trim();
  const repositoryType = String(repository?.type || '').trim();
  const repositoryClass = String(repository?.class || '').trim();
  if (repositoryName) return repositoryName;
  if (repositoryType && repositoryClass) return `${repositoryClass}:${repositoryType}`;
  if (repositoryType) return repositoryType;
  if (repositoryClass) return repositoryClass;
  return '';
}

export function getRecoveryPointDetailsSummary(point: RecoveryPoint): string {
  return String(point.display?.detailsSummary || '').trim();
}

export function normalizeRecoveryModeQueryValue(
  value: string | null | undefined,
): 'all' | RecoveryArtifactMode {
  const normalized = (value || '').trim().toLowerCase();
  if (normalized === 'snapshot' || normalized === 'local' || normalized === 'remote') {
    return normalized;
  }
  return 'all';
}
