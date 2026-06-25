import { Show, createMemo, createResource, type Component } from 'solid-js';
import { apiFetch } from '@/utils/apiClient';
import type {
  PBSBackupsPayload,
  PBSBackupsResponse,
  PVEBackupsPayload,
  PVEBackupsResponse,
} from '@/types/api';
import type { Resource } from '@/types/resource';
import { buildProxmoxBackupRecoveryModel } from './proxmoxBackupRecoveryModel';
import { ProxmoxBackupsCoverageStrip } from './ProxmoxBackupsCoverageStrip';

// Promotes Pulse's cross-source backup coverage signal (Proxmox guests x PBS)
// onto the front-door Overview, so the first thing an operator sees is the
// protection posture of the estate — the value no individual tool gives —
// rather than a re-render of Proxmox's own node/workload telemetry. The strip
// reuses the same recovery model and presentation as the Backups tab; the
// backups payloads are fetched async (createResource) so the Overview paint is
// not blocked and the strip fills in once coverage resolves.

async function fetchPVEBackups(): Promise<PVEBackupsPayload> {
  const response = await apiFetch('/api/backups/pve');
  if (!response.ok) {
    throw new Error(`Failed to load PVE backups (${response.status})`);
  }
  const payload = (await response.json()) as PVEBackupsResponse;
  return (
    payload?.data ?? {
      backupTasks: [],
      storageBackups: [],
      guestSnapshots: [],
    }
  );
}

async function fetchPBSBackups(): Promise<PBSBackupsPayload> {
  const response = await apiFetch('/api/backups/pbs');
  if (!response.ok) {
    throw new Error(`Failed to load PBS backups (${response.status})`);
  }
  const payload = (await response.json()) as PBSBackupsResponse;
  return payload?.data ?? { backups: [] };
}

export const ProxmoxOverviewCoverageStrip: Component<{
  workloads: readonly Resource[];
}> = (props) => {
  const [backups] = createResource(fetchPVEBackups);
  const [pbsBackups] = createResource(fetchPBSBackups);

  const nowMs = createMemo(() => Date.now());
  const recoveryModel = createMemo(() =>
    buildProxmoxBackupRecoveryModel({
      workloads: props.workloads,
      pbsBackups: pbsBackups()?.backups ?? [],
      archives: backups()?.storageBackups ?? [],
      snapshots: backups()?.guestSnapshots ?? [],
      tasks: backups()?.backupTasks ?? [],
      nowMs: nowMs(),
    }),
  );
  const coverage = () => recoveryModel().coverageSummary;

  return (
    <Show when={backups() !== undefined && pbsBackups() !== undefined}>
      <ProxmoxBackupsCoverageStrip
        title="Backup coverage"
        tail={<span>{props.workloads.length} targets</span>}
        segments={[
          {
            key: 'current',
            value: coverage().current,
            label: 'current',
            toneClass: 'bg-emerald-500',
          },
          {
            key: 'attention',
            value: coverage().attention,
            label: 'attention',
            toneClass: 'bg-amber-500',
            muted: coverage().attention === 0,
          },
          {
            key: 'uncovered',
            value: coverage().uncovered,
            label: 'uncovered',
            toneClass: 'bg-red-500',
            muted: coverage().uncovered === 0,
          },
        ]}
      />
    </Show>
  );
};

export default ProxmoxOverviewCoverageStrip;
