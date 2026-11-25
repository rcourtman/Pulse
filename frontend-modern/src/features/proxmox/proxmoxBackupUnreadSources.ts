import type {
  ProtectionPosture,
  ProtectionProviderState,
  ProtectionUnreadScope,
  RecoveryPlatform,
} from '@/types/recovery';

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

// Only collection observations are known to use a configured connection name.
// A retained point's scope may instead be a datastore or another native ID.
const readerLabel = ({ provider, source }: ProtectionProviderState, scope: string) => {
  if (!scope) return undefined;
  if (provider === 'proxmox-pve' && source === 'pve-backup-enumeration') {
    return `PVE connection: ${scope}`;
  }
  if (provider === 'proxmox-pbs' && source === 'pbs-backup-enumeration') {
    return `PBS connection: ${scope}`;
  }
  return undefined;
};

export const collectUnreadBackupSources = (
  postures: Iterable<ProtectionPosture>,
): UnreadBackupSource[] => {
  const byKey = new Map<string, UnreadBackupSource & { reader?: string }>();
  const readers = new Set<string>();
  for (const posture of postures) {
    for (const provider of posture.providerStates) {
      const connection = provider.scope?.trim() ?? '';
      const reader = readerLabel(provider, connection);
      // A readable sibling can share the same node, datastore or namespace
      // name. Consider it before skipping complete history.
      if (reader) readers.add(reader);
      if (provider.historyCompleteness === 'complete') continue;
      for (const scope of provider.unreadScopes ?? []) {
        const label = scopeLabel(scope);
        if (!label) continue;
        const key = `${provider.provider}\u0000${connection}\u0000${label}`;
        const existing = byKey.get(key);
        byKey.set(key, {
          label,
          reader: reader ?? existing?.reader,
          provider: provider.provider,
          accessDenied: Boolean(scope.accessDenied) || Boolean(existing?.accessDenied),
        });
      }
    }
  }
  return [...byKey.values()]
    .map(({ label, reader, provider, accessDenied }) => ({
      label: reader && (readers.size > 1 || accessDenied) ? `${label} (${reader})` : label,
      provider,
      accessDenied,
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
  const actions = [];
  if (denied || mixed) {
    actions.push("Check the Pulse API token's read permissions where access was denied.");
  }
  if (!denied) {
    actions.push(
      `${mixed ? 'For the other unread sources, check' : 'Check'} ${reachabilityCheck(notDenied)}.`,
    );
  }
  return `${verb} ${list}, so it cannot confirm the backups stored there. ${actions.join(' ')}`;
};
