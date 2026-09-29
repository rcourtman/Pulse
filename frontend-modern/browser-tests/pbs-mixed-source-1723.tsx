// Mock-backed current-component browser fixture for the #1723 side-by-side
// PBS API + Agent -> PBS API + PVE API + Agent transition. No live service.
import { createMemo, createSignal } from 'solid-js';
import { render } from 'solid-js/web';

import { ProxmoxBackupServersTable } from '../src/features/proxmox/ProxmoxBackupServersTable';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const seen = Date.now();
const pbs = {
  id: 'pbs-service',
  type: 'pbs',
  name: 'backup-connection',
  displayName: 'Backup connection',
  platformId: 'pbs-service',
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  sources: ['pbs'],
  status: 'online',
  lastSeen: seen,
  metricsTarget: { resourceType: 'agent', resourceId: 'pbs-service' },
  pbs: {
    instanceId: 'pbs-service',
    hostname: '10.0.0.5',
    // PBS API tokens cannot read /nodes; backend reports the corroborated ID.
    linkedAgentId: 'agent-uuid',
    connectionHealth: 'healthy',
    version: '3.2.1',
    datastores: [{ name: 'tank', total: 1000, used: 400, available: 600, usagePercent: 40 }],
  },
} as Resource;

const host = {
  id: 'agent-uuid',
  type: 'agent',
  name: 'backup-host.local',
  displayName: 'Backup host',
  platformId: 'agent-uuid',
  platformType: 'proxmox-pbs',
  sourceType: 'agent',
  sources: ['agent', 'pbs'],
  status: 'online',
  lastSeen: seen,
  agent: { agentId: 'agent-uuid', hostname: 'backup-host.local' },
  metricsTarget: { resourceType: 'agent', resourceId: 'agent-uuid' },
} as Resource;

const pveOnly = {
  ...host,
  id: 'pve-only',
  name: 'backup-connection',
  displayName: 'PVE node without Agent',
  sources: ['proxmox'],
  agent: undefined,
  platformData: { sources: ['proxmox'], proxmox: { nodeName: 'backup-connection' } },
  metricsTarget: { resourceType: 'agent', resourceId: 'pve-only' },
} as Resource;

const merged = {
  ...host,
  id: 'pve-merged-host',
  displayName: 'PVE plus PBS host',
  sources: ['proxmox', 'pbs', 'agent'],
  platformData: {
    sources: ['proxmox', 'pbs', 'agent'],
    agent: host.agent,
    proxmox: { nodeName: 'backup-host' },
  },
} as Resource;

const Fixture = () => {
  const [pveJoined, setPveJoined] = createSignal(false);
  const [hostOmitted, setHostOmitted] = createSignal(false);
  const servers = createMemo(() => [
    pbs,
    ...(pveJoined() ? [pveOnly] : []),
    ...(hostOmitted() ? [] : [pveJoined() ? merged : host]),
  ]);
  return (
    <main class="p-4 space-y-3">
      <h1>PBS mixed-source History fixture</h1>
      <div class="flex flex-wrap gap-2">
        <button type="button" onClick={() => setPveJoined(true)}>
          Join PVE API
        </button>
        <button type="button" onClick={() => setHostOmitted(true)}>
          Omit host row
        </button>
        <button type="button" onClick={() => setHostOmitted(false)}>
          Restore host row
        </button>
      </div>
      <output aria-label="Topology">{pveJoined() ? 'PBS + PVE + Agent' : 'PBS + Agent'}</output>
      <ProxmoxBackupServersTable servers={servers()} />
    </main>
  );
};

render(() => <Fixture />, document.getElementById('root') as HTMLElement);
