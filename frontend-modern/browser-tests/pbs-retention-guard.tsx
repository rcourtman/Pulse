// Synthetic refresh states exercising the production table and History drawer.
import { createMemo, createSignal } from 'solid-js';
import { render } from 'solid-js/web';

import { ProxmoxBackupServersTable } from '../src/features/proxmox/ProxmoxBackupServersTable';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const seen = Date.now();
const service: Resource = {
  id: 'pbs-service',
  type: 'pbs',
  name: 'backup-connection',
  displayName: 'backup-connection',
  platformId: 'pbs-service',
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  sources: ['pbs'],
  status: 'online',
  lastSeen: seen,
  cpu: { current: 10 },
  memory: { current: 40 },
  metricsTarget: { resourceType: 'agent', resourceId: 'pbs-service' },
  pbs: {
    instanceId: 'pbs-service',
    hostname: 'pbs-machine',
    nodeName: 'pbs-machine',
    connectionHealth: 'healthy',
    version: '3.2.1',
    datastores: [{ name: 'tank', total: 1000, used: 400, available: 600, usagePercent: 40 }],
  },
};
const host = (id: string): Resource => ({
  id,
  type: 'agent',
  name: 'pbs-machine',
  displayName: 'pbs-machine',
  platformId: id,
  platformType: 'proxmox-pbs',
  sourceType: 'agent',
  sources: ['agent'],
  status: 'online',
  lastSeen: seen,
  agent: {
    agentId: id,
    hostname: 'pbs-machine',
    disks: [{ mountpoint: '/old-host-only', total: 1000, used: 250, free: 750, usage: 25 }],
  },
  metricsTarget: { resourceType: 'agent', resourceId: id },
});
const hosts = [host('host-a'), host('host-b')];

const Fixture = () => {
  const [mode, setMode] = createSignal('service');
  const servers = createMemo(() => [
    {
      ...service,
      pbs: {
        ...service.pbs!,
        nodeName: mode() === 'replacement' ? 'replacement-machine' : 'pbs-machine',
      },
    },
    ...(mode() === 'matched' ? [hosts[0]] : mode() === 'ambiguous' ? hosts : []),
  ]);
  return (
    <main class="p-4 space-y-3">
      <h1>PBS retention regression verification</h1>
      <div class="flex flex-wrap gap-2">
        <button type="button" onClick={() => setMode('matched')}>
          Match host
        </button>
        <button type="button" onClick={() => setMode('omitted')}>
          Omit unchanged host
        </button>
        <button type="button" onClick={() => setMode('replacement')}>
          Change PBS machine
        </button>
        <button type="button" onClick={() => setMode('ambiguous')}>
          Ambiguous hosts
        </button>
        <button type="button" onClick={() => setMode('after-ambiguity')}>
          Omit ambiguous hosts
        </button>
      </div>
      <output aria-label="Evidence state">{mode()}</output>
      <ProxmoxBackupServersTable servers={servers()} />
    </main>
  );
};
render(() => <Fixture />, document.getElementById('root') as HTMLElement);
