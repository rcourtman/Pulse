// Synthetic bounded Backups/Overview resources, not installed host evidence.
import { createMemo, createSignal } from 'solid-js';
import { render } from 'solid-js/web';

import { ProxmoxBackupServersTable } from '../src/features/proxmox/ProxmoxBackupServersTable';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const seen = Date.now();
const suffixes = ['one', 'two', 'three'];
const services = suffixes.map((suffix, index): Resource => ({
  id: `pbs-${suffix}`,
  type: 'pbs',
  name: `backup-connection-${suffix}`,
  displayName: `backup-connection-${suffix}`,
  platformId: `pbs-${suffix}`,
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  sources: ['pbs'],
  status: 'online',
  lastSeen: seen,
  cpu: { current: 10 + index },
  memory: { current: 40 },
  metricsTarget: { resourceType: 'agent', resourceId: `pbs-${suffix}` },
  canonicalIdentity: { hostname: `10.2.0.${13 + index}` },
  pbs: {
    instanceId: `pbs-${suffix}`,
    hostname: `10.2.0.${13 + index}`,
    connectionHealth: 'healthy',
    version: '3.2.1',
    datastores: [{ name: 'tank', total: 1000, used: 400, available: 600, usagePercent: 40 }],
  },
}));
const hosts = suffixes.map((suffix): Resource => ({
  id: `agent-${suffix}`,
  type: 'agent',
  name: `Host label ${suffix}`,
  platformType: 'proxmox-pbs',
  sourceType: 'agent',
  sources: ['agent', 'pbs'],
  status: 'online',
  lastSeen: seen,
  agent: {
    agentId: `agent-${suffix}`,
    hostname: `host-${suffix}.example`,
    disks: [{ mountpoint: `/real-${suffix}`, total: 1000, used: 250, free: 750, usage: 25 }],
  },
  metricsTarget: { resourceType: 'agent', resourceId: `agent-${suffix}` },
}));
const guests = hosts.slice(0, 2).map((host, index): Resource => ({
  ...host,
  id: `vm-${suffixes[index]}`,
  type: 'vm',
  sources: ['proxmox', 'agent'],
  metricsTarget: { resourceType: 'vm', resourceId: `vm-${suffixes[index]}` },
}));
const unrelated = Array.from({ length: 6 }, (_, index): Resource => ({
  ...hosts[index % 3],
  id: `unrelated-${index}`,
  type: index < 3 ? 'vm' : 'agent',
  name: `backup-connection-${suffixes[index % 3]}`,
  displayName: `backup-connection-${suffixes[index % 3]}`,
  identity: { ips: [`10.9.0.${13 + index}`] },
  agent: {
    agentId: `unrelated-${index}`,
    hostname: `backup-connection-${suffixes[index % 3]}`,
    disks: [{ mountpoint: '/WRONG-HOST', total: 1000, used: 999, free: 1, usage: 99.9 }],
  },
  metricsTarget: { resourceType: 'vm', resourceId: `unrelated-${index}` },
}));

const Fixture = () => {
  const [mode, setMode] = createSignal('unlinked');
  const servers = createMemo(() => [
    ...services.map((server, index): Resource => ({
      ...server,
      pbs: {
        ...server.pbs!,
        linkedAgentId:
          ['linked', 'omitted', 'withdrawn'].includes(mode()) &&
          !(mode() === 'withdrawn' && index === 2)
            ? `agent-${suffixes[index]}`
            : undefined,
        nodeName: mode() === 'node' && index === 0 ? 'host-one.example' : undefined,
      },
    })),
    ...(mode() === 'omitted' ? [] : [...hosts, ...guests]),
    ...unrelated,
  ]);
  return (
    <main class="p-4 space-y-3">
      <h1>PBS identity boundary verification</h1>
      <div class="flex flex-wrap gap-2">
        <button type="button" onClick={() => setMode('linked')}>
          Corroborate links
        </button>
        <button type="button" onClick={() => setMode('omitted')}>
          Omit linked host rows
        </button>
        <button type="button" onClick={() => setMode('linked')}>
          Restore linked host rows
        </button>
        <button type="button" onClick={() => setMode('withdrawn')}>
          Withdraw third link
        </button>
        <button type="button" onClick={() => setMode('node')}>
          Report exact node hostname
        </button>
      </div>
      <output aria-label="Evidence state">{mode()}</output>
      <ProxmoxBackupServersTable servers={servers()} />
    </main>
  );
};

render(() => <Fixture />, document.getElementById('root') as HTMLElement);
