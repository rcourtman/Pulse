// Final Overview history/caption/update-access bytes through real consuming components.
// This fixture never operates on a guest, credential, service or backup.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { useWorkloads } from '../src/hooks/useWorkloads';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { NodeDrawer } from '../src/components/Workloads/NodeDrawer';
import Docs from '../src/pages/Docs';
import type { Resource } from '../src/types/resource';
import type { Alert, Node } from '../src/types/api';
import '../src/index.css';

const os =
  'Microsoft Windows Server 2025 Datacenter Evaluation with retained build identity 26100.1742';
const capacity = 10 * 1024 ** 3;
const alert = (id: string, resourceId: string, type = 'memory'): Alert =>
  ({
    id,
    resourceId,
    resourceName: 'integration-guest',
    type,
    level: 'warning',
    value: 92,
    threshold: 85,
    message: 'Synthetic metric warning',
    startTime: new Date().toISOString(),
  }) as Alert;
function GuestFixture() {
  const [mode, setMode] = createSignal('unknown');
  const [memoryMode, setMemoryMode] = createSignal('cache-low');
  const [display, setDisplay] = createSignal<'bars' | 'sparklines'>('bars');
  const observedAt = '2026-10-13T12:00:00Z';
  const memory = () => {
    const unavailable = ['unknown', 'powered-off'].includes(memoryMode());
    const current = memoryMode() === 'current-zero' ? 0 : memoryMode() === 'cache-low' ? 25 : 92;
    return {
      current,
      total: capacity,
      used: (capacity * current) / 100,
      ...(unavailable ? { usageUnavailable: true } : {}),
      observation: {
        state: unavailable ? ('unavailable' as const) : ('current' as const),
        source:
          memoryMode() === 'powered-off'
            ? 'powered-off'
            : memoryMode().startsWith('cache')
              ? 'status-mem'
              : 'agent',
        observedAt,
      },
    };
  };
  const [open, setOpen] = createSignal(false);
  const [backup, setBackup] = createSignal('2026-10-13T12:30:00Z');
  const snapshot = (): Resource[] => [
    {
      id: 'fixture:pve1:101',
      type: 'vm',
      name: 'integration-guest',
      displayName: 'integration-guest',
      platformId: 'fixture',
      platformType: 'proxmox-pve',
      platformScopes: ['proxmox-pve'],
      sourceType: 'api',
      sources: ['proxmox'],
      status: 'online',
      lastSeen: Date.now(),
      metricsTarget: { resourceType: 'vm', resourceId: 'fixture:pve1:101' },
      cpu: { current: 10 },
      memory: memory(),
      agent: { stale: memoryMode() === 'stale', lastReportAt: observedAt },
      ...(mode() !== 'unknown'
        ? { disk: { current: 50, used: capacity / 2, total: capacity } }
        : {}),
      proxmox: {
        lastBackup: backup(),
        backupInProgress: mode() === 'retained',
        vmid: 101,
        nodeName: 'pve1',
        instance: 'fixture',
        runtimeStatus: 'running',
        osName: os,
        guestAgentStatus:
          mode() === 'unknown' ? 'not-running' : mode() === 'retained' ? 'deferred' : 'available',
        diskStatusReason:
          mode() === 'unknown'
            ? 'agent-not-running'
            : mode() === 'retained'
              ? 'prev-vm-locked'
              : mode() === 'denied'
                ? 'permission-denied'
                : '',
        lock: mode() === 'retained' ? 'backup' : '',
        disks:
          mode() === 'unknown'
            ? []
            : [
                {
                  mountpoint: '/data',
                  type: 'ext4',
                  total: capacity,
                  used: capacity / 2,
                  usage: 50,
                },
              ],
      },
    },
  ];
  const state = useWorkloads(() => true, { resourceSnapshot: snapshot });
  const guest = () => state.workloads()[0];
  (window as any).__integration = {
    mode: setMode,
    memory: setMemoryMode,
    display: setDisplay,
    backup: setBackup,
    evidence: () => ({ mode: mode(), projected: guest(), os }),
  };
  return (
    <main class="mx-auto min-h-screen max-w-4xl space-y-3 bg-surface p-3 text-base-content">
      <h1>Combined guest drawer</h1>
      <p class="text-xs text-muted">Synthetic observations; no guest commands.</p>
      <Show when={guest()}>
        <table class="w-full table-fixed">
          <tbody>
            <GuestRow
              guest={guest()}
              visibleColumnIds={['name', 'memory', 'disk']}
              metricDisplayMode={display()}
              metricHoverMode="value"
              isExpanded={open()}
              workloadTableLayoutMode={window.innerWidth < 768 ? 'phone' : 'wide'}
              onClick={() => setOpen(!open())}
            />
          </tbody>
        </table>
        <Show when={open()}>
          <GuestDrawer
            guest={guest()}
            alerts={[alert('fixture-warning', 'fixture:pve1:101')]}
            onClose={() => setOpen(false)}
          />
        </Show>
      </Show>
    </main>
  );
}
function NodeFixture() {
  const [node, setNode] = createSignal<Node>({
    id: 'fixture-pve1',
    name: 'pve1',
    instance: 'fixture',
    host: 'pve1',
    status: 'online',
    type: 'node',
    cpu: 0.1,
    cpuTemperature: 55,
    temperature: {
      available: true,
      cpuPackage: 55,
      cores: [{ core: 0, temp: 50 }],
      hasGPU: true,
      gpu: 95,
    },
    memory: { total: capacity, used: capacity / 4, free: capacity * 0.75, usage: 25 },
    disk: { total: capacity, used: capacity / 2, free: capacity / 2, usage: 50 },
    uptime: 3600,
    loadAverage: [0.1, 0.2, 0.3],
    kernelVersion: '6.8.12',
    pveVersion: 'pve-manager/9.2.21',
    cpuInfo: {
      model:
        'Fixture CPU with a deliberately long hardware model and complete revision identifier 2026-A-extended',
      cores: 4,
      sockets: 1,
      mhz: '2400',
    },
    lastSeen: new Date().toISOString(),
    connectionHealth: 'healthy',
    linkedAgentId: 'agent:fixture-node-with-a-long-retained-identity-2026',
    networkIn: 1024 ** 2,
    networkOut: 2 * 1024 ** 2,
    diskRead: 1024 ** 2,
    diskWrite: 2 * 1024 ** 2,
    pendingUpdates: 7,
    pendingUpdatesStatus: 'stale',
    pendingUpdatesReason: 'permission_denied',
    pendingUpdatesCheckedAt: new Date(Date.now() - 60000).toISOString(),
  });
  (window as any).__integration = {
    update: (delta: Partial<Node>) => setNode((previous) => ({ ...previous, ...delta })),
    evidence: node,
  };
  return (
    <main class="mx-auto min-h-screen max-w-4xl space-y-3 bg-surface p-3 text-base-content">
      <h1>Combined node drawer</h1>
      <p class="text-xs text-muted">Synthetic observations; no permissions changed.</p>
      <NodeDrawer
        node={node()}
        alerts={[alert('fixture-guest-warning', 'fixture:pve1:101')]}
        disks={[
          { mountpoint: '/unknown', total: capacity, usage: -1 },
          { mountpoint: '/empty', total: capacity, free: capacity },
          { mountpoint: '/measured', total: capacity, used: capacity / 2, usage: 50 },
        ]}
      />
    </main>
  );
}
render(
  () => (
    <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
      <Router>
        <Route path="/docs/*docPath" component={Docs} />
        <Route path="/node" component={NodeFixture} />
        <Route path="*" component={GuestFixture} />
      </Router>
    </DarkModeContext.Provider>
  ),
  document.getElementById('root')!,
);
