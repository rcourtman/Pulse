import type { ProtectionPosture, ProtectionUnreadScope } from '@/types/recovery';

// The server names what each provider's latest backup read could not reach.
// Every guest on that connection carries the same list, so collapse it to one
// page-level line: the operator fixes the storage, not each guest.

const scopeLabel = (scope: ProtectionUnreadScope): string => {
  const node = scope.node?.trim() ?? '';
  const storage = scope.storage?.trim() ?? '';
  const namespace = scope.namespace?.trim() ?? '';
  switch (scope.kind) {
    case 'node':
      return node ? `the storage list on node ${node}` : '';
    case 'storage':
      if (!storage) return '';
      return node ? `${storage} on ${node}` : storage;
    case 'datastore':
      return storage ? `the namespace list of datastore ${storage}` : '';
    case 'namespace':
      if (!storage) return '';
      return namespace
        ? `namespace ${namespace} on datastore ${storage}`
        : `the root namespace of datastore ${storage}`;
    default:
      return '';
  }
};

export interface UnreadBackupSource {
  label: string;
  accessDenied: boolean;
}

export const collectUnreadBackupSources = (
  postures: Iterable<ProtectionPosture>,
): UnreadBackupSource[] => {
  const byKey = new Map<string, UnreadBackupSource & { connection: string; base: string }>();
  for (const posture of postures) {
    for (const provider of posture.providerStates) {
      if (provider.historyCompleteness === 'complete') continue;
      for (const scope of provider.unreadScopes ?? []) {
        const base = scopeLabel(scope);
        if (!base) continue;
        const connection = provider.scope?.trim() ?? '';
        const key = `${provider.provider}\u0000${connection}\u0000${base}`;
        const existing = byKey.get(key);
        byKey.set(key, {
          label: base,
          base,
          connection,
          accessDenied: Boolean(scope.accessDenied) || Boolean(existing?.accessDenied),
        });
      }
    }
  }
  const entries = [...byKey.values()];
  // Name the connection only when the gap spans more than one of them.
  const connections = new Set(entries.map((entry) => entry.connection));
  return entries
    .map((entry) => ({
      label:
        connections.size > 1 && entry.connection
          ? `${entry.base} (${entry.connection})`
          : entry.base,
      accessDenied: entry.accessDenied,
    }))
    .sort((a, b) => a.label.localeCompare(b.label));
};

const NAMED = 3;

export const describeUnreadBackupSources = (
  sources: readonly UnreadBackupSource[],
): string | null => {
  if (sources.length === 0) return null;
  const names = sources.slice(0, NAMED).map((source) => source.label);
  if (sources.length > NAMED) names.push(`${sources.length - NAMED} more`);
  const list =
    names.length === 1
      ? names[0]
      : `${names.slice(0, -1).join(', ')} and ${names[names.length - 1]}`;
  const denied = sources.every((source) => source.accessDenied);
  const verb = denied ? 'Pulse does not have permission to read' : 'Pulse cannot read';
  const action = denied
    ? 'Check the permissions of the Pulse API token.'
    : 'Check that the storage is reachable from Proxmox.';
  return `${verb} ${list}, so it cannot confirm the backups stored there. ${action}`;
};
