import type { RecoverableArtifact } from './proxmoxBackupRecoveryModel';

type LocationArtifact = Pick<
  RecoverableArtifact,
  | 'locationKey'
  | 'locationLabel'
  | 'locationGroupKey'
  | 'locationGroupLabel'
  | 'legacyLocationKey'
  | 'legacyLocationGroupKey'
>;

export type BackupLocationSelection =
  | { state: 'all' }
  | { state: 'matched'; key: string; label: string }
  | { state: 'ambiguous' | 'missing' };

// Saved URLs used case-folded, delimiter-joined keys. Resolve them only when
// every observation agrees on one exact source tuple, never by first match.
// Store-wide aliases intentionally span local nodes, not other repositories.
export function resolveBackupLocationSelection(
  artifacts: readonly LocationArtifact[],
  selected: string,
): BackupLocationSelection {
  if (!selected) return { state: 'all' };
  const exact = new Map<string, string>();
  const legacy = new Map<string, string>();
  for (const artifact of artifacts) {
    const scopes = [
      [artifact.locationKey, artifact.locationLabel, artifact.legacyLocationKey],
      [artifact.locationGroupKey, artifact.locationGroupLabel, artifact.legacyLocationGroupKey],
    ];
    for (const [key, label, alias] of scopes) {
      if (!key || !label) continue;
      if (key === selected) exact.set(key, label);
      if (alias === selected) legacy.set(key, label);
    }
  }
  const matches = exact.size > 0 ? exact : legacy;
  if (matches.size === 0) return { state: 'missing' };
  if (matches.size > 1) return { state: 'ambiguous' };
  const [key, label] = matches.entries().next().value!;
  return { state: 'matched', key, label };
}
