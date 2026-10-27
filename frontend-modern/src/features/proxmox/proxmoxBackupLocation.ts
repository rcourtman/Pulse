import {
  getProxmoxBackupSourcePresentation,
  type ProxmoxBackupSourceKind,
} from './proxmoxBackupSourcePresentation';

export interface BackupLocationScope {
  locationKey?: string;
  locationLabel?: string;
  /** The backup reader's connection, never inferred from the guest's placement. */
  locationConnection?: string;
  /** Read compatibility only; never write this lossy identity to a new URL. */
  legacyLocationKey?: string;
}

export function backupLocationScope(
  sourceKind: ProxmoxBackupSourceKind,
  instance: string | undefined,
  repository: string | undefined,
): BackupLocationScope {
  const parts = [instance?.trim(), repository?.trim()].filter((part): part is string =>
    Boolean(part),
  );
  if (parts.length === 0) return {};
  const legacyParts = [...new Set(parts.map((part) => part.toLowerCase()))];
  return {
    // A connection and repository are positional, case-sensitive identities,
    // not a colon-delimited display label. Preserve missing positions as well.
    locationKey: JSON.stringify([sourceKind, instance ?? '', repository ?? '']),
    locationLabel: `${getProxmoxBackupSourcePresentation(sourceKind).badgeLabel} · ${instance?.trim() || 'Unknown connection'} / ${repository?.trim() || 'Unknown location'}`,
    locationConnection: instance?.trim() || undefined,
    legacyLocationKey: [sourceKind, ...legacyParts].join(':'),
  };
}

export function resolveBackupLocationFilter(
  selected: string,
  locations: readonly BackupLocationScope[],
): { key: string | undefined; unresolved: boolean } {
  if (!selected) return { key: undefined, unresolved: false };
  if (locations.some((location) => location.locationKey === selected)) {
    return { key: selected, unresolved: false };
  }
  // Older saved views folded case, omitted empty fields and used an ambiguous
  // delimiter. Keep them usable only when the current inventory has one exact
  // interpretation. A collision must not broaden the filter to both sources.
  const matches = new Set(
    locations
      .filter((location) => location.legacyLocationKey === selected)
      .map((location) => location.locationKey)
      .filter((key): key is string => Boolean(key)),
  );
  return matches.size === 1
    ? { key: [...matches][0], unresolved: false }
    : { key: undefined, unresolved: true };
}
