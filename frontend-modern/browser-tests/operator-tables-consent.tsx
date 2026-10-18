// Production components and CSS with synthetic observations. No native, released or
// installed recovery claim; the current history API does not describe restore scope.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { StoragePoolsTable } from '../src/components/Storage/StoragePoolsTable';
import { DiskList } from '../src/components/Storage/DiskList';
import { KubernetesControllersTable } from '../src/features/kubernetes/KubernetesControllersTable';
import { ProxmoxCoverageTable } from '../src/features/proxmox/ProxmoxCoverageTable';
import { ProxmoxRecoverableTable } from '../src/features/proxmox/ProxmoxRecoverableTable';
import { ProxmoxBackupServersTable } from '../src/features/proxmox/ProxmoxBackupServersTable';
import { ProxmoxNodesTable } from '../src/features/proxmox/ProxmoxNodesTable';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { Table, TableBody } from '../src/components/shared/Table';
import { ToastContainer } from '../src/components/Toast/Toast';
import { UpdateHistorySection } from '../src/components/Settings/UpdateHistorySection';
import { buildProxmoxBackupRecoveryModel } from '../src/features/proxmox/proxmoxBackupRecoveryModel';
import { groupStorageRecords } from '../src/features/storageBackups/storageModelCore';
import { EMPTY_STORAGE_ALERT_STATE } from '../src/features/storageBackups/storageAlertState';
import { DarkModeContext, WebSocketContext } from '../src/contexts/appRuntime';
import { updateStore } from '../src/stores/updates';
import type { Resource } from '../src/types/resource';
import type { StorageRecord } from '../src/features/storageBackups/models';
import '../src/index.css';

const now = Date.parse('2026-10-03T12:00:00Z');
const resource = (id: string, type: Resource['type'], extra: Partial<Resource> = {}): Resource => ({
  id,
  type,
  name: id,
  displayName: id,
  platformId: 'fixture-estate',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'online',
  lastSeen: now,
  ...extra,
});
const pools: StorageRecord[] = [
  ['archive-tank', 'DEGRADED', 'resilver'],
  ['backup-pool', 'ONLINE', 'scrub'],
].map(([name, state, scan], index) => ({
  id: name,
  name,
  category: 'pool',
  health: index ? 'healthy' : 'warning',
  location: { label: index ? 'backup-nas' : 'archive-nas', scope: 'host' },
  source: {
    platform: 'truenas',
    family: 'onprem',
    origin: 'resource',
    adapterId: 'resource-storage',
  },
  capacity: { totalBytes: 2e12, usedBytes: 1.18e12, freeBytes: 0.82e12, usagePercent: 59 },
  capabilities: ['capacity', 'health'],
  observedAt: now,
  protectionLabel: `ZFS pool ${name} ${scan} is running (45.2%)`,
  protectionSummary: `ZFS pool ${name} ${scan} is running (45.2%)`,
  rebuildInProgress: true,
  details: {
    zfsPool: {
      state,
      scan: `${scan} in progress`,
      scanDetails: {
        function: scan.toUpperCase(),
        state: 'SCANNING',
        percentage: 45.2,
      },
      devices: [],
    },
  },
}));
const disks = [
  resource('disk-sda', 'physical_disk', {
    identity: { hostname: 'archive-nas' },
    canonicalIdentity: { hostname: 'archive-nas' },
    physicalDisk: {
      devPath: '/dev/sda',
      model: 'Archive HDD',
      serial: 'FIXTURE-1',
      diskType: 'hdd',
      sizeBytes: 2e12,
      health: 'FAILED',
      temperature: 42,
      storageRole: 'data',
      risk: {
        level: 'critical',
        reasons: [
          {
            code: 'smart-failed',
            severity: 'critical',
            summary: 'SMART failed. The full reason remains readable in the expanded disk header.',
          },
        ],
      },
    },
  }),
  resource('disk-nvme', 'physical_disk', {
    identity: { hostname: 'backup-nas' },
    canonicalIdentity: { hostname: 'backup-nas' },
    physicalDisk: {
      devPath: '/dev/nvme0n1',
      model: 'Backup SSD',
      serial: 'FIXTURE-2',
      diskType: 'nvme',
      sizeBytes: 1e12,
      health: 'PASSED',
      temperature: 44,
      wearout: 2,
      storageRole: 'cache',
    },
  }),
];
const controllers = [
  resource('nightly-import', 'k8s-job', {
    platformType: 'kubernetes',
    kubernetes: {
      resourceKind: 'Job',
      clusterName: 'prod',
      namespace: 'batch',
      startTime: '2026-10-03T11:00:00Z',
      completionTime: '2026-10-03T11:05:00Z',
      desiredReplicas: 1,
      succeeded: 1,
      active: 0,
      failed: 0,
    },
  }),
  resource('billing-rollup', 'k8s-cronjob', {
    platformType: 'kubernetes',
    kubernetes: {
      resourceKind: 'CronJob',
      clusterName: 'prod',
      namespace: 'batch',
      schedule: '*/5 * * * *',
      lastScheduleTime: '2026-10-03T11:00:00Z',
      lastSuccessfulTime: '2026-10-03T11:05:00Z',
      suspend: true,
      active: 0,
    },
  }),
  resource('node-exporter', 'k8s-daemonset', {
    platformType: 'kubernetes',
    kubernetes: {
      resourceKind: 'DaemonSet',
      clusterName: 'prod',
      namespace: 'observability',
      desiredNumberScheduled: 3,
      currentNumberScheduled: 3,
      numberReady: 2,
      numberUnavailable: 1,
    },
  }),
  resource('database', 'k8s-statefulset', {
    platformType: 'kubernetes',
    kubernetes: {
      resourceKind: 'StatefulSet',
      clusterName: 'prod',
      namespace: 'apps',
      desiredReplicas: 1,
      readyReplicas: 1,
      serviceName: 'database-headless',
    },
  }),
];
const workload = resource('ct-310', 'system-container', {
  name: 'artifact-cache-310',
  displayName: 'artifact-cache-310',
  status: 'running',
  proxmox: { vmid: 310, node: 'pve-edge', instance: 'homelab' },
});
const model = buildProxmoxBackupRecoveryModel({
  workloads: [workload],
  pbsBackups: [],
  tasks: [],
  nowMs: now,
  archives: [
    {
      id: 'file-310',
      storage: 'local',
      node: 'pve-edge',
      instance: 'homelab',
      type: 'ct',
      vmid: 310,
      time: '2026-09-20T02:00:00Z',
      ctime: Date.parse('2026-09-20T02:00:00Z') / 1000,
      size: 1048576,
      format: 'tar.zst',
      protected: false,
      volid: 'local:backup/vzdump-lxc-310-2026_09_20-02_00_00.tar.zst',
      isPBS: false,
      verified: false,
    },
  ],
  snapshots: [
    {
      id: 'snap-310',
      name: 'fresh-local-snapshot',
      node: 'pve-edge',
      instance: 'homelab',
      type: 'ct',
      vmid: 310,
      time: '2026-10-03T11:30:00Z',
      vmstate: false,
    },
  ],
});
const pbs = resource('pbs-archive', 'pbs', {
  platformType: 'proxmox-pbs',
  cpu: { current: 14 },
  memory: { current: 32, total: 8e9, used: 2.56e9, free: 5.44e9 },
  pbs: {
    instance: 'pbs-archive',
    datastores: [
      {
        name: 'archive',
        total: 2e12,
        used: 1.18e12,
        free: 0.82e12,
        usage: 59,
        status: 'available',
      },
    ],
  },
});
const node = resource('pve-edge', 'node', {
  cpu: { current: 12, cores: 4 },
  memory: { current: 32, total: 8e9, used: 2.56e9, free: 5.44e9 },
  disk: { current: 30, total: 1e12, used: 0.3e12, free: 0.7e12 },
  proxmox: { node: 'pve-edge', instance: 'homelab', uptime: 86400 },
});
function Fixture() {
  const [view, setView] = createSignal('Pools');
  const [disk, setDisk] = createSignal<string | null>(null);
  const [pool, setPool] = createSignal<string | null>(null);
  const [expanded, setExpanded] = createSignal(new Set<string>());
  const ws = {
    state: { pmg: [], pbs: [], nodes: [], resources: [], vms: [], containers: [] },
    activeAlerts: () => ({}),
    connected: () => true,
    initialDataReceived: () => true,
  };
  return (
    <WebSocketContext.Provider value={ws as any}>
      <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
        <ToastContainer />
        <main class="min-h-screen space-y-4 bg-surface p-4 text-base-content">
          <h1 class="text-lg font-semibold">Operator tables and rollback consent</h1>
          <nav aria-label="Fixture views" class="flex flex-wrap gap-2">
            {['Pools', 'Disks', 'Controllers', 'Backups', 'Consent'].map((name) => (
              <button
                type="button"
                class="min-h-11 rounded border border-border px-3"
                onClick={() => setView(name)}
              >
                {name}
              </button>
            ))}
          </nav>
          <Show when={view() === 'Pools'}>
            <section aria-label="Pool table">
              <StoragePoolsTable
                groupedRecords={groupStorageRecords(pools, 'none')}
                groupBy="none"
                expandedGroups={new Set(['All'])}
                toggleGroup={() => {}}
                expandedPoolId={pool()}
                setExpandedPoolId={setPool}
                storageGrowthBySeriesId={new Map()}
                storageGrowthColumnLabel="Growth"
                physicalDisks={[]}
                nodeOnlineByLabel={new Map()}
                highlightedRecordId={null}
                getRecordAlertState={() => EMPTY_STORAGE_ALERT_STATE}
                isLoading={false}
              />
            </section>
          </Show>
          <Show when={view() === 'Disks'}>
            <section aria-label="Disk table">
              <DiskList
                disks={disks}
                nodes={[]}
                selectedNode={null}
                searchTerm=""
                selectedDiskId={disk()}
                onSelectedDiskChange={setDisk}
              />
            </section>
          </Show>
          <Show when={view() === 'Controllers'}>
            <section aria-label="Controller table">
              <KubernetesControllersTable
                resources={controllers}
                emptyIcon={<span />}
                emptyTitle="No controllers"
                emptyDescription="No controllers"
                showToolbar={false}
              />
            </section>
          </Show>
          <Show when={view() === 'Backups'}>
            <section aria-label="Backup tables" class="space-y-4">
              <h2>Coverage</h2>
              <ProxmoxCoverageTable
                rows={model.coverageRows}
                hasAnyRows
                emptyIcon={<span />}
                emptyTitle="No backups"
                emptyDescription="No backups"
                sortKey={() => 'posture'}
                sortDirection={() => 'asc'}
                onSort={() => {}}
                expandedKeys={expanded()}
                onToggleExpand={(key) =>
                  setExpanded((current) =>
                    current.has(key)
                      ? new Set([...current].filter((item) => item !== key))
                      : new Set([...current, key]),
                  )
                }
                showTaskColumn={false}
              />
              <h2>By date</h2>
              <ProxmoxRecoverableTable
                artifacts={model.recoverableArtifacts}
                hasAnyArtifacts
                emptyIcon={<span />}
                emptyTitle="No artifacts"
                emptyDescription="No artifacts"
                sortKey={() => 'created'}
                sortDirection={() => 'desc'}
                onSort={() => {}}
                sizeMaxBytes={1048576}
              />
              <h2>Backup servers</h2>
              <ProxmoxBackupServersTable servers={[pbs]} />
              <h2>Proxmox nodes</h2>
              <ProxmoxNodesTable
                nodes={[node]}
                guests={[workload]}
                emptyIcon={<span />}
                emptyTitle="No nodes"
                emptyDescription="No nodes"
              />
              <h2>Workload row</h2>
              <Table>
                <TableBody>
                  <GuestRow
                    guest={
                      {
                        id: 'ct-310',
                        vmid: 310,
                        name: 'artifact-cache-310',
                        node: 'pve-edge',
                        instance: 'homelab',
                        status: 'running',
                        type: 'lxc',
                        cpu: 0.12,
                        cpus: 2,
                        memory: { usage: 30, total: 1e9, used: 0.3e9, free: 0.7e9 },
                        disk: { usage: 20, total: 1e10, used: 2e9, free: 8e9 },
                        uptime: 86400,
                        template: false,
                        lastBackup: 0,
                        tags: null,
                        lock: '',
                        lastSeen: new Date(now).toISOString(),
                      } as any
                    }
                    visibleColumnIds={['name', 'cpu', 'memory', 'disk', 'uptime']}
                    workloadTableLayoutMode={innerWidth < 440 ? 'phone' : 'compact'}
                  />
                </TableBody>
              </Table>
            </section>
          </Show>
          <Show when={view() === 'Consent'}>
            <section aria-label="Rollback history">
              <UpdateHistorySection />
            </section>
          </Show>
        </main>
      </DarkModeContext.Provider>
    </WebSocketContext.Provider>
  );
}
void updateStore.checkForUpdates();
render(
  () => (
    <Router>
      <Route path="/*" component={Fixture} />
    </Router>
  ),
  document.getElementById('root')!,
);
