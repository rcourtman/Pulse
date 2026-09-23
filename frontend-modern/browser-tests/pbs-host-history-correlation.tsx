// Browser fixture: a Proxmox Backups PBS host whose Pulse agent is surfaced
// both as a PVE guest (with agent telemetry) and as a standalone source=pbs
// host row. Reproduces #1723: before the fix the two identity matches were
// treated as ambiguous, so the PBS row kept its service metrics target and the
// History tab showed "Collecting history" for a host that has history.
// Also covers PBS-only and co-installed bare-metal PVE/PBS with no guest.
// Synthetic props only; the check script intercepts the metrics-history request.
import { createSignal, onCleanup } from 'solid-js';
import { render } from 'solid-js/web';

import { ProxmoxBackupServersTable } from '../src/features/proxmox/ProxmoxBackupServersTable';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const sharedAgent = { agentId: 'agent-proxback', hostname: 'proxback-vm' };

const pbs = {
  id: 'pbs-1',
  type: 'pbs',
  name: 'proxback',
  displayName: 'proxback',
  platformId: 'pbs-1',
  platformType: 'proxmox-pbs',
  sourceType: 'hybrid',
  sources: ['pbs'],
  status: 'online',
  lastSeen: Date.now(),
  cpu: { current: 4.7 },
  memory: { current: 20.3, total: 8000, used: 1624, free: 6376 },
  pbs: {
    instanceId: 'proxback',
    // The connection is configured by an address the agent never reports, so
    // the reported node name below is the only machine identity that links the
    // PBS service row to its host agent (#1723).
    hostname: '10.0.0.5',
    nodeName: 'proxback-vm',
    version: '3.2.1',
    connectionHealth: 'healthy',
    datastores: [
      { name: 'tank', total: 1000, used: 400, available: 600, usagePercent: 40 },
      { name: 'archive', total: 2000, used: 600, available: 1400, usagePercent: 30 },
    ],
  },
  // The PBS service target: correct for the service, but it has no host series.
  metricsTarget: { resourceType: 'agent', resourceId: 'pbs-1' },
  discoveryTarget: { resourceType: 'agent', resourceId: 'pbs-1' },
  platformData: {
    sources: ['pbs'],
    pbs: {
      instanceId: 'proxback',
      hostname: '10.0.0.5',
      nodeName: 'proxback-vm',
      datastoreCount: 2,
    },
  },
} as unknown as Resource;

const guest = {
  id: 'vm-100',
  type: 'vm',
  name: 'proxback-vm',
  displayName: 'proxback-vm',
  platformId: 'proxmox:100',
  platformType: 'proxmox-pve',
  sourceType: 'hybrid',
  sources: ['proxmox', 'agent'],
  status: 'online',
  lastSeen: Date.now(),
  cpu: { current: 15.4 },
  memory: { current: 16.9, total: 8000, used: 1352, free: 6648 },
  agent: sharedAgent,
  metricsTarget: { resourceType: 'vm', resourceId: 'proxmox:100' },
  discoveryTarget: { resourceType: 'agent', resourceId: 'proxback-vm' },
  platformData: { sources: ['proxmox', 'agent'], agent: sharedAgent },
} as unknown as Resource;

const standalone = {
  id: 'agent-proxback',
  type: 'agent',
  name: 'proxback-vm',
  displayName: 'proxback-vm',
  platformId: 'agent-proxback',
  platformType: 'proxmox-pbs',
  sourceType: 'hybrid',
  sources: ['agent', 'pbs'],
  status: 'online',
  lastSeen: Date.now(),
  agent: sharedAgent,
  metricsTarget: { resourceType: 'agent', resourceId: 'agent-proxback' },
  discoveryTarget: { resourceType: 'agent', resourceId: 'proxback-vm' },
  platformData: { sources: ['agent', 'pbs'], agent: sharedAgent },
} as unknown as Resource;

// Side-by-side services on one bare-metal host: the node is not a VM, and
// host history belongs to the standalone agent rather than a guest series.
const node = {
  ...standalone,
  id: 'node-proxback',
  type: 'node',
  platformId: 'proxmox/proxback',
  platformType: 'proxmox-pve',
  sources: ['proxmox', 'agent'],
  metricsTarget: { resourceType: 'node', resourceId: 'proxmox/proxback' },
  platformData: { sources: ['proxmox', 'agent'], agent: sharedAgent },
} as Resource;

const query = new URLSearchParams(window.location.search);
const topology = query.get('topology') ?? 'guest';
const resources =
  topology === 'pbs-only'
    ? [pbs, standalone]
    : topology === 'side-by-side'
      ? [pbs, node, standalone]
      : [pbs, guest, standalone];

const Fixture = () => {
  const [servers, setServers] = createSignal(structuredClone(resources));
  const [snapshot, setSnapshot] = createSignal(0);
  // Simulates a snapshot in which the merged host correlation target is
  // transiently absent, which is what gates the drawer's History tab.
  const [dropMetricsTarget, setDropMetricsTarget] = createSignal(false);
  // Simulates a live refresh that briefly omits the correlated host row while
  // the PBS server row remains, which is what flipped the drawer's Identity and
  // History target between the host series and the service key (#1723).
  const [dropHostRows, setDropHostRows] = createSignal(false);
  let timer: ReturnType<typeof setTimeout> | undefined;
  onCleanup(() => clearTimeout(timer));
  const refresh = () => {
    const count = snapshot() + 1;
    let next = structuredClone(resources);
    next[1].cpu = { current: 15.4 + count };
    const tank = next[0].pbs!.datastores!.find((store) => store.name === 'tank')!;
    tank.used = 400 + count * 10;
    tank.available = 600 - count * 10;
    tank.usagePercent = 40 + count;
    if (query.get('order') !== 'stable' && count % 2 === 1) {
      next[0].pbs!.datastores!.reverse();
    }
    if (dropMetricsTarget()) {
      for (const resource of next) delete resource.metricsTarget;
    }
    if (dropHostRows()) {
      next = next.filter((resource) => resource.type === 'pbs');
    }
    setServers(next);
    setSnapshot(count);
  };
  return (
    <>
      <button onClick={refresh}>Refresh resource snapshot</button>
      <button
        onClick={() => {
          clearTimeout(timer);
          timer = setTimeout(refresh, 100);
        }}
      >
        Schedule automatic snapshot
      </button>
      <button
        onClick={() => {
          setDropMetricsTarget(true);
          refresh();
        }}
      >
        Drop metrics target
      </button>
      <button
        onClick={() => {
          setDropMetricsTarget(false);
          refresh();
        }}
      >
        Restore metrics target
      </button>
      <button
        onClick={() => {
          setDropHostRows(true);
          refresh();
        }}
      >
        Drop correlated host
      </button>
      <button
        onClick={() => {
          setDropHostRows(false);
          refresh();
        }}
      >
        Restore correlated host
      </button>
      <output aria-label="Snapshot number">{snapshot()}</output>
      <ProxmoxBackupServersTable servers={servers()} />
    </>
  );
};

render(() => <Fixture />, document.getElementById('root') as HTMLElement);
