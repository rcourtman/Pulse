import type { ProtectionRollup, RecoveryPoint } from '@/types/recovery';
import type { Resource } from '@/types/resource';
import { getRecoveryArtifactModePresentation } from '@/utils/recoveryArtifactModePresentation';
import { normalizeRecoveryOutcome } from '@/utils/recoveryOutcomePresentation';
import { getPreferredResourceDisplayName } from '@/utils/resourceIdentity';
import { titleCaseDelimitedLabel } from '@/utils/textPresentation';

export type RecoveryArtifactMode = 'snapshot' | 'local' | 'remote';

const getRecoveryLinkedResourceLabel = (
  subjectResourceId: string,
  resourcesById: Map<string, Resource>,
): string => {
  if (!subjectResourceId) return '';
  const resource = resourcesById.get(subjectResourceId);
  if (!resource) return '';
  const label = getPreferredResourceDisplayName(resource).trim();
  if (!label) return '';
  if (label.toLowerCase() === subjectResourceId.toLowerCase()) return '';
  return label;
};

export function getRecoveryRollupSubjectLabel(
  rollup: ProtectionRollup,
  resourcesById: Map<string, Resource>,
): string {
  const subjectResourceId = (rollup.subjectResourceId || '').trim();
  const displayLabel = String(rollup.display?.subjectLabel || '').trim();
  const linkedResourceLabel = getRecoveryLinkedResourceLabel(subjectResourceId, resourcesById);
  if (linkedResourceLabel) return linkedResourceLabel;
  if (displayLabel) return displayLabel;

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
  const displayLabel = String(point.display?.subjectLabel || '').trim();
  const linkedResourceLabel = getRecoveryLinkedResourceLabel(subjectResourceId, resourcesById);
  if (linkedResourceLabel) return linkedResourceLabel;
  if (displayLabel) return displayLabel;

  const ref = point.subjectRef || null;
  const namespace = String(ref?.namespace || '').trim();
  const name = String(ref?.name || '').trim();
  if (namespace && name) return `${namespace}/${name}`;
  if (name) return name;
  const id = String(ref?.id || '').trim();
  if (id) return id;
  if (subjectResourceId) return subjectResourceId;
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

export function getRecoveryPointKindLabel(value: string | null | undefined): string {
  const normalized = (value || '').trim().toLowerCase();
  if (!normalized) return 'n/a';

  switch (normalized) {
    case 'backup':
      return 'Backup';
    case 'snapshot':
      return 'Snapshot';
    case 'other':
      return 'Other';
    default:
      return titleCaseDelimitedLabel(value, {
        fallback: 'n/a',
        preserveShortAllCaps: true,
      });
  }
}

export function getRecoveryPointModeLabel(value: string | null | undefined): string {
  const normalized = normalizeRecoveryModeQueryValue(value);
  if (normalized !== 'all') {
    return getRecoveryArtifactModePresentation(normalized).label;
  }

  return titleCaseDelimitedLabel(value, {
    fallback: 'n/a',
    preserveShortAllCaps: true,
  });
}

export function getRecoveryPointOutcomeLabel(value: string | null | undefined): string {
  return titleCaseDelimitedLabel(normalizeRecoveryOutcome(value), {
    fallback: 'Unknown',
    preserveShortAllCaps: true,
  });
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
