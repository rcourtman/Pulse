import type { WorkloadReference } from './proxmoxBackupRecoveryModel';

export type BackupNodeSelection =
  | { state: 'all' }
  | { state: 'matched'; key: string; label: string }
  | { state: 'ambiguous' | 'missing' };

// Display names are not node identities: separate connections may repeat both
// a friendly name and a native node name. Incomplete identities stay local to
// their workload instead of borrowing another source's node.
export function backupNodeScope(workload: WorkloadReference) {
  const node = workload.nativeNode?.trim() ? workload.nativeNode : '';
  const display = workload.node?.trim() || node;
  if (!display) return undefined;
  const connection = workload.instance?.trim() ? workload.instance : '';
  const identity = connection && node ? [connection, node] : [connection, node, workload.key];
  const nodeLabel = display !== node && node ? `${display} (${node})` : display;
  return {
    key: `backup-node-v1:${JSON.stringify(identity)}`,
    label: [
      connection || 'Connection not reported',
      nodeLabel,
      ...(!connection || !node ? [workload.label] : []),
    ].join(' / '),
    alias: workload.node?.trim(),
  };
}

export function backupNodeOptions(workloads: readonly WorkloadReference[]) {
  const labels = new Map<string, string>();
  for (const workload of workloads) {
    const scope = backupNodeScope(workload);
    if (!scope) continue;
    const previous = labels.get(scope.key);
    // Conflicting display aliases must not make response order select a label.
    if (!previous || scope.label < previous) labels.set(scope.key, scope.label);
  }
  return [...labels]
    .map(([value, label]) => ({ value, label }))
    .sort(
      (a, b) =>
        a.label.localeCompare(b.label) || (a.value < b.value ? -1 : a.value > b.value ? 1 : 0),
    );
}

// Old URLs contain a bare display name. Keep them useful only while every
// matching observation belongs to one exact scope. Never silently broaden an
// ambiguous or vanished selection to All, including after live inventory edits.
export function resolveBackupNodeSelection(
  workloads: readonly WorkloadReference[],
  selected: string,
): BackupNodeSelection {
  if (!selected) return { state: 'all' };
  const options = backupNodeOptions(workloads);
  const exact = options.find((option) => option.value === selected);
  if (exact) return { state: 'matched', key: exact.value, label: exact.label };
  const aliases = new Set(
    workloads.flatMap((workload) => {
      const scope = backupNodeScope(workload);
      return scope?.alias === selected ? [scope.key] : [];
    }),
  );
  if (aliases.size === 0) return { state: 'missing' };
  if (aliases.size > 1) return { state: 'ambiguous' };
  const match = options.find((option) => aliases.has(option.value))!;
  return { state: 'matched', key: match.value, label: match.label };
}

export function backupNodeMatches(workload: WorkloadReference, selection: BackupNodeSelection) {
  return (
    selection.state === 'all' ||
    (selection.state === 'matched' && backupNodeScope(workload)?.key === selection.key)
  );
}
