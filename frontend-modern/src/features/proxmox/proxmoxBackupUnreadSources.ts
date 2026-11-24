import type { ProtectionPosture, ProtectionUnreadScope, RecoveryPlatform } from '@/types/recovery';

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
  provider: RecoveryPlatform;
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
          provider: provider.provider,
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
      provider: entry.provider,
      accessDenied: entry.accessDenied,
    }))
    .sort((a, b) => a.label.localeCompare(b.label));
};

const NAMED = 3;

const reachabilityCheck = (sources: readonly UnreadBackupSource[]): string => {
  if (sources.every(({ provider }) => provider === 'proxmox-pbs')) {
    return 'that Pulse can reach the storage through its Proxmox Backup Server connection';
  }
  if (sources.every(({ provider }) => provider === 'proxmox-pve')) {
    return 'that the storage is reachable from Proxmox';
  }
  return 'that the backup sources are reachable through their configured connections';
};

export const describeUnreadBackupSources = (
  sources: readonly UnreadBackupSource[],
): string | null => {
  if (sources.length === 0) return null;
  // A read without an explicit denial does not tell us why it failed. Keep
  // known refusals distinct and give the reachability check for the rest.
  const notDenied = sources.filter((source) => !source.accessDenied);
  const denied = notDenied.length === 0;
  const mixed = !denied && notDenied.length < sources.length;
  const names = sources
    .slice(0, NAMED)
    .map(({ label, accessDenied }) => (mixed && accessDenied ? `${label} (access denied)` : label));
  if (sources.length > NAMED) {
    const omitted = sources.slice(NAMED);
    const omittedDenied = mixed ? omitted.filter((source) => source.accessDenied).length : 0;
    names.push(`${omitted.length} more${omittedDenied ? ` (${omittedDenied} access denied)` : ''}`);
  }
  const list =
    names.length === 1
      ? names[0]
      : `${names.slice(0, -1).join(', ')} and ${names[names.length - 1]}`;
  const verb = denied ? 'Pulse does not have permission to read' : 'Pulse cannot read';
  const action = denied
    ? 'Check the permissions of the Pulse API token.'
    : `${mixed ? 'For the other unread sources, check' : 'Check'} ${reachabilityCheck(notDenied)}.`;
  return `${verb} ${list}, so it cannot confirm the backups stored there. ${action}`;
};
